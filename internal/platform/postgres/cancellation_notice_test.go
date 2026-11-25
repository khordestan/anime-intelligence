package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/khordestan/anime-intelligence/internal/modules/catalog/feed"
	"github.com/khordestan/anime-intelligence/internal/platform/catalogsync"
)

func TestCancellationNoticesRequireReviewedPrimaryTransition(t *testing.T) {
	db, admin, _, _ := testDB(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)
	const provider = "synthetic-cancellation"
	if _, err := admin.Exec(ctx, `INSERT INTO data_sources(provider_key,display_name,enabled,policy_url)
		VALUES($1,'Synthetic cancellation feed',true,'https://example.test/policy')`, provider); err != nil {
		t.Fatal(err)
	}
	snapshot := feed.Snapshot{Version: 1, Provider: provider, ObservedAt: now.Add(-3 * time.Hour), Titles: []feed.Title{
		{ExternalID: "expected", Title: "Synthetic announced title", MediaType: "TV", Status: "ANNOUNCED"},
		{ExternalID: "historic", Title: "Synthetic historical cancellation", MediaType: "TV", Status: "CANCELLED"},
		{ExternalID: "unapproved", Title: "Synthetic unapproved publication", MediaType: "TV", Status: "HIATUS"},
	}}
	publish := catalogsync.ImportOptions{CancellationEvents: true}
	apply := func(snapshot feed.Snapshot, options catalogsync.ImportOptions, want int) {
		t.Helper()
		result, err := catalogsync.ImportWithOptions(ctx, admin, snapshot, now, options)
		if err != nil || result.CancellationEvents != want {
			t.Fatalf("import result %+v, error %v", result, err)
		}
	}
	apply(snapshot, publish, 0)
	var animeID string
	if err := admin.QueryRow(ctx, `SELECT anime_id::text FROM anime_external_ids WHERE provider=$1 AND external_id='expected'`, provider).Scan(&animeID); err != nil {
		t.Fatal(err)
	}
	opted, err := db.Register(ctx, "cancel-opted@example.test", "synthetic-cancellation-password")
	if err != nil {
		t.Fatal(err)
	}
	other, err := db.Register(ctx, "cancel-default@example.test", "synthetic-cancellation-password")
	if err != nil {
		t.Fatal(err)
	}
	verifyAccount(t, db, opted.Token)
	verifyAccount(t, db, other.Token)
	a, b := db.Repository(opted.Token), db.Repository(other.Token)
	on := true
	if ok, err := a.FollowWithSettings(ctx, animeID, "DUB", "ANIME", &on); err != nil || !ok {
		t.Fatalf("opted follow: %t %v", ok, err)
	}
	if ok, err := b.Follow(ctx, animeID, "DUB"); err != nil || !ok {
		t.Fatalf("normal follow: %t %v", ok, err)
	}
	if _, err := admin.Exec(ctx, `UPDATE user_follows SET created_at=$1,
		schedule_notices_since=CASE WHEN schedule_notices_since IS NOT NULL THEN $1::timestamptz END
		WHERE anime_id=$2`, now.Add(-2*time.Hour), animeID); err != nil {
		t.Fatal(err)
	}
	unapproved := snapshot
	unapproved.Titles = []feed.Title{snapshot.Titles[2]}
	unapproved.Titles[0].Status = "CANCELLED"
	unapproved.ObservedAt = now.Add(-90 * time.Minute)
	apply(unapproved, catalogsync.ImportOptions{}, 0)
	snapshot.ObservedAt = now.Add(-time.Hour)
	snapshot.Titles[0].Status = "CANCELLED"
	snapshot.Titles[2].Status = "CANCELLED"
	preview := publish
	preview.Preview = true
	apply(snapshot, preview, 1)
	if notices, err := a.Notifications(ctx); err != nil || len(notices) != 0 {
		t.Fatalf("preview produced a notice: %+v %v", notices, err)
	}
	apply(snapshot, publish, 1)
	snapshot.ObservedAt = now.Add(-30 * time.Minute)
	apply(snapshot, publish, 0)
	notices, err := a.Notifications(ctx)
	if err != nil || len(notices) != 1 || notices[0].Title != "Title cancelled" || notices[0].Provider != provider {
		t.Fatalf("unexpected notice: %+v %v", notices, err)
	}
	if notices, err := b.Notifications(ctx); err != nil || len(notices) != 0 {
		t.Fatalf("default follow received cancellation: %+v %v", notices, err)
	}
	correction := catalogsync.ImportOptions{CancellationCorrectionEvents: true, Preview: true}
	snapshot.ObservedAt = now.Add(-20 * time.Minute)
	for i := range snapshot.Titles {
		snapshot.Titles[i].Status = "ANNOUNCED"
	}
	checkCorrection := func(want int) {
		t.Helper()
		result, err := catalogsync.ImportWithOptions(ctx, admin, snapshot, now, correction)
		if err != nil || result.CancellationCorrectionEvents != want || result.CancellationEvents != 0 {
			t.Fatalf("correction result: %+v %v", result, err)
		}
	}
	checkCorrection(1)
	if notices, err := a.Notifications(ctx); err != nil || len(notices) != 1 {
		t.Fatalf("preview created a correction: %+v %v", notices, err)
	}
	correction.Preview = false
	checkCorrection(1)
	snapshot.ObservedAt = now.Add(-10 * time.Minute)
	checkCorrection(0)
	notices, err = a.Notifications(ctx)
	if err != nil || len(notices) != 2 || notices[0].Title != "Cancellation corrected" || notices[0].Provider != provider || notices[1].Title != "Title cancelled" {
		t.Fatalf("history did not retain both notices: %+v %v", notices, err)
	}
	if notices, err := b.Notifications(ctx); err != nil || len(notices) != 0 {
		t.Fatalf("default follow received a cancellation correction: %+v %v", notices, err)
	}

}
