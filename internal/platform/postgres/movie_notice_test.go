package postgres_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/khordestan/anime-intelligence/internal/modules/catalog/feed"
	"github.com/khordestan/anime-intelligence/internal/platform/catalogsync"
)

func TestConfirmedMovieNoticeRequiresFollowAndReleaseEvidence(t *testing.T) {
	db, owner, _, _ := testDB(t)
	ctx := context.Background()
	const provider = "movie-notice-fixture"
	if _, err := owner.Exec(ctx, `INSERT INTO data_sources(provider_key,display_name,enabled,policy_url)
		VALUES ($1,'Synthetic movie source',true,'https://example.test/policy')`, provider); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Microsecond)
	release := now.Add(-30 * time.Minute)
	snapshot := feed.Snapshot{Version: 1, Provider: provider, ObservedAt: now.Add(-2 * time.Hour), Titles: []feed.Title{
		{ExternalID: "movie", Title: "Synthetic Film", MediaType: "MOVIE", Status: "ANNOUNCED", Episodes: []feed.Episode{
			{Number: 1, Track: "SUB", ScheduledAt: &release},
		}},
	}}
	if result, err := catalogsync.Import(ctx, owner, snapshot, now); err != nil || result.ReleaseEvents != 0 {
		t.Fatalf("schedule import: %+v, %v", result, err)
	}
	var animeID string
	if err := owner.QueryRow(ctx, `SELECT anime_id::text FROM anime_external_ids WHERE provider=$1`, provider).Scan(&animeID); err != nil {
		t.Fatal(err)
	}
	account, err := db.Register(ctx, "movie-follower@example.test", "synthetic-movie-password")
	if err != nil {
		t.Fatal(err)
	}
	verifyAccount(t, db, account.Token)
	repository := db.Repository(account.Token)
	if ok, err := repository.Follow(ctx, animeID, "SUB"); !ok || err != nil {
		t.Fatalf("follow movie: %t, %v", ok, err)
	}
	if _, err := owner.Exec(ctx, `UPDATE user_follows SET created_at=$1 WHERE user_id=$2`, now.Add(-time.Hour), account.User.ID); err != nil {
		t.Fatal(err)
	}
	if notices, err := repository.Notifications(ctx); err != nil || len(notices) != 0 {
		t.Fatalf("elapsed schedule produced notices: %+v, %v", notices, err)
	}

	snapshot.ObservedAt = now.Add(-20 * time.Minute)
	snapshot.Titles[0].Status = "FINISHED"
	snapshot.Titles[0].Episodes[0].ReleasedAt = &release
	if result, err := catalogsync.Import(ctx, owner, snapshot, now); err != nil || result.ReleaseEvents != 1 {
		t.Fatalf("confirmed release import: %+v, %v", result, err)
	}
	// Reclassifying display metadata after the event cannot relabel its notice.
	snapshot.ObservedAt = now.Add(-10 * time.Minute)
	snapshot.Titles[0].MediaType = "SPECIAL"
	if result, err := catalogsync.Import(ctx, owner, snapshot, now); err != nil || result.ReleaseEvents != 0 {
		t.Fatalf("refresh reapplied event: %+v, %v", result, err)
	}
	notices, err := repository.Notifications(ctx)
	if err != nil || len(notices) != 1 || notices[0].Title != "Movie released" ||
		!strings.Contains(notices[0].Body, "Synthetic Film · Release confirmed (SUB)") {
		t.Fatalf("confirmed movie notice: %+v, %v", notices, err)
	}
}
