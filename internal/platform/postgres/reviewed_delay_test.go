package postgres_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/khordestan/anime-intelligence/internal/modules/catalog/feed"
	"github.com/khordestan/anime-intelligence/internal/platform/catalogsync"
	"github.com/khordestan/anime-intelligence/internal/platform/postgres"
)

func TestReviewedDelayRequiresAuthorityAndMatchingConsent(t *testing.T) {
	owner, writer, _, runtimeDSN := queuedDB(t)
	ctx := context.Background()
	const provider = "synthetic-delay"
	queueExec(t, owner, `INSERT INTO data_sources(provider_key,display_name,enabled,policy_url)
		VALUES($1,'Synthetic delay source',true,'https://example.test/policy')`, provider)
	now := time.Now().UTC().Truncate(time.Microsecond)
	scheduled := now.Add(time.Hour)
	snapshot := feed.Snapshot{Version: 1, Provider: provider, ObservedAt: now.Add(-2 * time.Hour), Titles: []feed.Title{{
		ExternalID: "one", Title: "Synthetic delayed release", MediaType: "TV", Status: "ANNOUNCED",
		Episodes: []feed.Episode{{Number: 1, Track: "SUB", ScheduledAt: &scheduled}},
	}}}
	if _, err := catalogsync.Import(ctx, writer, snapshot, now); err != nil {
		t.Fatal(err)
	}
	var animeID string
	if err := owner.QueryRow(ctx, `SELECT anime_id::text FROM anime_external_ids WHERE provider=$1 AND external_id='one'`,
		provider).Scan(&animeID); err != nil {
		t.Fatal(err)
	}
	db, err := postgres.Open(ctx, runtimeDSN)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	follow := func(email, track string, optedIn bool) *postgres.Repository {
		t.Helper()
		user, err := db.Register(ctx, email, "synthetic-delay-password")
		if err != nil {
			t.Fatal(err)
		}
		verifyAccount(t, db, user.Token)
		repo := db.Repository(user.Token)
		if ok, err := repo.FollowWithSettings(ctx, animeID, track, "ANIME", &optedIn); err != nil || !ok {
			t.Fatalf("follow: %t %v", ok, err)
		}
		return repo
	}
	matching := follow("delay-matching@example.test", "SUB", true)
	otherTrack := follow("delay-track@example.test", "DUB", true)
	defaultFollow := follow("delay-default@example.test", "ALL", false)
	queueExec(t, owner, `UPDATE user_follows SET created_at=$1,
		schedule_notices_since=CASE WHEN schedule_notices_since IS NOT NULL THEN $1::timestamptz END
		WHERE anime_id=$2`, now.Add(-time.Hour), animeID)
	review := catalogsync.ReviewedDelay{
		Provider: provider, ExternalID: "one", AnimeID: animeID, Episode: 1, Track: "SUB",
		ExpectedRevision: 1, ObservedAt: now.Add(-30 * time.Minute), Reference: "https://example.test/explicit-delay",
	}
	stale := review
	stale.ExpectedRevision++
	if _, err := catalogsync.PublishReviewedDelay(ctx, writer, stale, now, true); !errors.Is(err, catalogsync.ErrConflict) {
		t.Fatalf("stale revision accepted: %v", err)
	}
	queueExec(t, owner, `UPDATE data_sources SET enabled=false WHERE provider_key=$1`, provider)
	if _, err := catalogsync.PublishReviewedDelay(ctx, writer, review, now, true); !errors.Is(err, catalogsync.ErrSource) {
		t.Fatalf("unapproved source accepted: %v", err)
	}
	queueExec(t, owner, `UPDATE data_sources SET enabled=true WHERE provider_key=$1`, provider)
	preview, err := catalogsync.PublishReviewedDelay(ctx, writer, review, now, false)
	if err != nil || !preview.Preview || !preview.WouldPublish {
		t.Fatalf("preview: %+v %v", preview, err)
	}
	if notices, err := matching.Notifications(ctx); err != nil || len(notices) != 0 {
		t.Fatalf("preview created alerts: %+v %v", notices, err)
	}
	result, err := catalogsync.PublishReviewedDelay(ctx, writer, review, now, true)
	if err != nil || !result.Published {
		t.Fatalf("apply: %+v %v", result, err)
	}
	result, err = catalogsync.PublishReviewedDelay(ctx, writer, review, now, true)
	if err != nil || !result.AlreadyPublished || result.Published {
		t.Fatalf("replay: %+v %v", result, err)
	}
	notices, err := matching.Notifications(ctx)
	if err != nil || len(notices) != 1 || notices[0].Title != "Release delayed" || notices[0].Provider != provider {
		t.Fatalf("delay notice: %+v %v", notices, err)
	}
	for _, repo := range []*postgres.Repository{otherTrack, defaultFollow} {
		if notices, err := repo.Notifications(ctx); err != nil || len(notices) != 0 {
			t.Fatalf("unmatched follow received a delay notice: %+v %v", notices, err)
		}
	}
	var latest time.Time
	if err := owner.QueryRow(ctx, `SELECT scheduled_at FROM airing_schedule WHERE anime_id=$1
		AND episode_number=1 AND release_track='SUB' AND revision=1`, animeID).Scan(&latest); err != nil || !latest.Equal(scheduled) {
		t.Fatalf("delay changed the schedule: %v %v", latest, err)
	}
	snapshot.ObservedAt = now
	snapshot.Titles[0].Episodes[0].ReleasedAt = &now
	if _, err := catalogsync.Import(ctx, writer, snapshot, now); err != nil {
		t.Fatal(err)
	}
	review.ExpectedRevision = 2
	review.Reference += "-after-release"
	if _, err := catalogsync.PublishReviewedDelay(ctx, writer, review, now, true); !errors.Is(err, catalogsync.ErrConflict) {
		t.Fatalf("confirmed release accepted a delay: %v", err)
	}
}
