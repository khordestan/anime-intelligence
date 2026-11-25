package catalogsync

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/url"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

type ReviewedDelay struct {
	Provider         string
	ExternalID       string
	AnimeID          string
	Episode          int
	Track            string
	ExpectedRevision int
	ObservedAt       time.Time
	Reference        string
}

type ReviewedDelayResult struct {
	Preview          bool `json:"preview"`
	WouldPublish     bool `json:"wouldPublish"`
	Published        bool `json:"published"`
	AlreadyPublished bool `json:"alreadyPublished"`
}

var ErrDelayReview = errors.New("invalid delay review")

func PublishReviewedDelay(ctx context.Context, conn *pgx.Conn, review ReviewedDelay, now time.Time, apply bool) (ReviewedDelayResult, error) {
	var result ReviewedDelayResult
	ref, err := url.Parse(review.Reference)
	if err != nil || ref.Scheme != "https" || ref.Hostname() == "" || ref.User != nil || len(review.Reference) > 2048 ||
		strings.TrimSpace(review.Reference) != review.Reference || review.Provider == "" || review.ExternalID == "" ||
		review.Episode < 1 || review.Episode > 100000 || review.ExpectedRevision < 1 ||
		review.ObservedAt.IsZero() || review.ObservedAt.After(now) {
		return result, ErrDelayReview
	}
	switch review.Track {
	case "RAW", "SUB", "DUB", "UNKNOWN":
	default:
		return result, ErrDelayReview
	}
	if conn == nil || CheckWriter(ctx, conn) != nil {
		return result, ErrImport
	}
	tx, err := conn.Begin(ctx)
	if err != nil {
		return result, err
	}
	defer func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = tx.Rollback(cleanup)
	}()
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(742019684)`); err != nil {
		return result, err
	}

	var sourceID, animeID string
	err = tx.QueryRow(ctx, `SELECT s.id::text, i.anime_id::text
		FROM data_sources s JOIN anime_external_ids i ON i.provider=s.provider_key
		WHERE s.provider_key=$1 AND s.enabled AND COALESCE(s.policy_url,'')<>''
		  AND i.external_id=$2 AND i.anime_id::text=$3 FOR UPDATE OF s`,
		review.Provider, review.ExternalID, review.AnimeID).Scan(&sourceID, &animeID)
	if errors.Is(err, pgx.ErrNoRows) {
		return result, ErrSource
	}
	if err != nil {
		return result, err
	}
	identity, _ := json.Marshal([]any{animeID, review.Provider, review.Episode, review.Track, review.Reference})
	digest := sha256.Sum256(identity)
	eventKey := "episode-delay:" + hex.EncodeToString(digest[:])
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM domain_events WHERE event_key=$1)`, eventKey).Scan(&result.AlreadyPublished); err != nil {
		return result, err
	}
	result.Preview = !apply
	if result.AlreadyPublished {
		return result, nil
	}

	var revision int
	var authority string
	var released bool
	err = tx.QueryRow(ctx, `SELECT revision, COALESCE(source_id::text,''), actual_released_at IS NOT NULL
		FROM airing_schedule WHERE anime_id=$1 AND episode_number=$2 AND release_track=$3
		ORDER BY revision DESC LIMIT 1 FOR UPDATE`, animeID, review.Episode, review.Track).Scan(&revision, &authority, &released)
	if errors.Is(err, pgx.ErrNoRows) {
		return ReviewedDelayResult{}, ErrConflict
	}
	if err != nil {
		return ReviewedDelayResult{}, err
	}
	if revision != review.ExpectedRevision || authority != sourceID || released {
		return ReviewedDelayResult{}, ErrConflict
	}
	result.WouldPublish = !apply
	if !apply {
		return result, nil
	}
	_, err = tx.Exec(ctx, `INSERT INTO domain_events(event_key,event_type,aggregate_type,aggregate_id,payload,occurred_at)
		VALUES($1,'EPISODE_DELAY_REPORTED','ANIME',$2,jsonb_build_object(
			'provider',$3::text,'number',$4::int,'track',$5::text,'reference',$6::text,'revision',$7::int),$8)`,
		eventKey, animeID, review.Provider, review.Episode, review.Track, review.Reference, revision, review.ObservedAt)
	if err != nil {
		return ReviewedDelayResult{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return ReviewedDelayResult{}, err
	}
	result.Published = true
	return result, nil
}
