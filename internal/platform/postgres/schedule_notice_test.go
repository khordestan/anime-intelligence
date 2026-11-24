package postgres_test

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/khordestan/anime-intelligence/internal/modules/catalog/feed"
	"github.com/khordestan/anime-intelligence/internal/platform/catalogsync"
)

func TestScheduleNoticesRequireBothPublicationAndOwnerConsent(t *testing.T) {
	db, owner, _, _ := testDB(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)
	const provider = "schedule-notice-fixture"
	if _, err := owner.Exec(ctx, `INSERT INTO data_sources(provider_key,display_name,enabled,policy_url)
		VALUES($1,'Synthetic schedule source',true,'https://example.test/policy')`, provider); err != nil {
		t.Fatal(err)
	}
	start := now.Add(24 * time.Hour)
	snapshot := feed.Snapshot{Version: 1, Provider: provider, ObservedAt: now.Add(-4 * time.Hour), Titles: []feed.Title{
		{ExternalID: "title", Title: "Synthetic Schedule", MediaType: "TV", Status: "RELEASING",
			Episodes: []feed.Episode{{Number: 1, Track: "SUB", ScheduledAt: &start}}},
	}}
	publish := catalogsync.ImportOptions{ScheduleChangeEvents: true}
	apply := func(options catalogsync.ImportOptions, want int) {
		t.Helper()
		result, err := catalogsync.ImportWithOptions(ctx, owner, snapshot, now, options)
		if err != nil || result.ScheduleChangeEvents != want || result.ReleaseEvents != 0 {
			t.Fatalf("import: %+v, %v", result, err)
		}
	}
	apply(publish, 0)
	var id string
	if err := owner.QueryRow(ctx, `SELECT anime_id::text FROM anime_external_ids WHERE provider=$1`, provider).Scan(&id); err != nil {
		t.Fatal(err)
	}
	opted, err := db.Register(ctx, "schedule-opted@example.test", "synthetic-schedule-password")
	if err != nil {
		t.Fatal(err)
	}
	other, err := db.Register(ctx, "schedule-default@example.test", "synthetic-schedule-password")
	if err != nil {
		t.Fatal(err)
	}
	verifyAccount(t, db, opted.Token)
	verifyAccount(t, db, other.Token)
	a, b := db.Repository(opted.Token), db.Repository(other.Token)
	enabled := true
	if ok, err := a.FollowWithSettings(ctx, id, "SUB", "ANIME", &enabled); !ok || err != nil {
		t.Fatalf("opted follow: %t, %v", ok, err)
	}
	if ok, err := b.Follow(ctx, id, "SUB"); !ok || err != nil {
		t.Fatalf("default follow: %t, %v", ok, err)
	}
	if _, err := owner.Exec(ctx, `UPDATE user_follows SET created_at=$1,
		schedule_notices_since=CASE WHEN schedule_notices_since IS NOT NULL THEN $1::timestamptz END
		WHERE anime_id=$2`, now.Add(-3*time.Hour), id); err != nil {
		t.Fatal(err)
	}
	if ok, err := a.Follow(ctx, id, "SUB"); !ok || err != nil {
		t.Fatalf("legacy track update: %t, %v", ok, err)
	}
	follow, found, err := a.FollowForTitle(ctx, id)
	if err != nil || !found || !follow.ScheduleNotices {
		t.Fatalf("legacy call cleared consent: %+v %t %v", follow, found, err)
	}
	schedule := func(observed, arrival time.Time) {
		snapshot.ObservedAt = observed
		snapshot.Titles[0].Episodes[0].ScheduledAt = &arrival
	}
	schedule(now.Add(-2*time.Hour), start.Add(time.Hour))
	apply(catalogsync.ImportOptions{}, 0)
	schedule(now.Add(-time.Hour), start.Add(2*time.Hour))
	preview := publish
	preview.Preview = true
	apply(preview, 1)
	if notices, err := a.Notifications(ctx); err != nil || len(notices) != 0 {
		t.Fatalf("preview escaped rollback: %+v %v", notices, err)
	}
	apply(publish, 1)
	for range 2 {
		notices, err := a.Notifications(ctx)
		if err != nil || len(notices) != 1 || notices[0].Provider != provider ||
			notices[0].Title != "Schedule moved later" || !strings.Contains(notices[0].Body, "Unconfirmed schedule") {
			t.Fatalf("consented notices: %+v %v", notices, err)
		}
	}
	if notices, err := b.Notifications(ctx); err != nil || len(notices) != 0 {
		t.Fatalf("default follow received a notice: %+v %v", notices, err)
	}
	if ok, err := b.FollowWithSettings(ctx, id, "SUB", "ANIME", &enabled); !ok || err != nil {
		t.Fatalf("late consent: %t %v", ok, err)
	}
	if notices, err := b.Notifications(ctx); err != nil || len(notices) != 0 {
		t.Fatalf("late consent replayed history: %+v %v", notices, err)
	}
	down, err := os.ReadFile("../../../migrations/000024_schedule_notice_consent.down.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := owner.Exec(ctx, string(down)); err == nil || !strings.Contains(err.Error(), "cannot remove retained schedule-notice consent") {
		t.Fatalf("downgrade failed to protect consent: %v", err)
	}
	// A track change takes effect from the new subscription settings.
	if ok, err := a.Follow(ctx, id, "ALL"); !ok || err != nil {
		t.Fatalf("track update: %t %v", ok, err)
	}
	schedule(now.Add(-30*time.Minute), start.Add(90*time.Minute))
	apply(publish, 1)
	if notices, err := a.Notifications(ctx); err != nil || len(notices) != 1 {
		t.Fatalf("track change replayed another earlier notice: %+v %v", notices, err)
	}
	disabled := false
	if ok, err := a.FollowWithSettings(ctx, id, "ALL", "ANIME", &disabled); !ok || err != nil {
		t.Fatalf("disable consent: %t %v", ok, err)
	}
	follow, found, err = a.FollowForTitle(ctx, id)
	if err != nil || !found || follow.ScheduleNotices {
		t.Fatalf("consent was not disabled: %+v %t %v", follow, found, err)
	}
	schedule(now.Add(-15*time.Minute), start.Add(3*time.Hour))
	apply(publish, 1)
	if notices, err := a.Notifications(ctx); err != nil || len(notices) != 1 {
		t.Fatalf("disabled consent changed history: %+v %v", notices, err)
	}

}
