package postgres_test

import (
	"context"
	"testing"
	"time"
)

func TestOverlappingFranchiseFollowsKeepUniqueCalendarAndNotices(t *testing.T) {
	db, owner, _, _ := testDB(t)
	ctx := context.Background()
	s, err := db.Register(ctx, "franchise-scale@example.test", "synthetic-franchise-scale-password")
	if err != nil {
		t.Fatal(err)
	}
	verifyAccount(t, db, s.Token)
	now := time.Now().UTC()
	day := now.Truncate(24 * time.Hour)
	const titles = 128
	const follows = 80
	if _, err := owner.Exec(ctx, `INSERT INTO anime(id,canonical_title,media_type)
		SELECT ('20000000-0000-4000-8000-'||lpad(n::text,12,'0'))::uuid,'Synthetic season '||n,'TV'
		FROM generate_series(1,$1::int) n`, titles); err != nil {
		t.Fatal(err)
	}
	if _, err := owner.Exec(ctx, `INSERT INTO anime_relations(source_anime_id,target_anime_id,relation_type)
		SELECT ('20000000-0000-4000-8000-'||lpad(n::text,12,'0'))::uuid,
		       ('20000000-0000-4000-8000-'||lpad((n+1)::text,12,'0'))::uuid,'SEQUEL'
		FROM generate_series(1,$1::int-1) n`, titles); err != nil {
		t.Fatal(err)
	}
	if _, err := owner.Exec(ctx, `INSERT INTO user_follows(user_id,anime_id,follow_scope,release_track,created_at)
		SELECT $1,('20000000-0000-4000-8000-'||lpad(n::text,12,'0'))::uuid,
		       'FRANCHISE','SUB',$2::timestamptz+n*interval '1 second'
		FROM generate_series(1,$3::int) n`, s.User.ID, now.Add(-2*time.Hour), follows); err != nil {
		t.Fatal(err)
	}
	if _, err := owner.Exec(ctx, `INSERT INTO airing_schedule(anime_id,episode_number,release_track,scheduled_at)
		SELECT id,1,'SUB',$1::timestamptz+interval '12 hours'
		FROM anime WHERE id::text LIKE '20000000-%'`, day); err != nil {
		t.Fatal(err)
	}
	if _, err := owner.Exec(ctx, `INSERT INTO domain_events(event_key,event_type,aggregate_type,aggregate_id,payload,occurred_at)
		SELECT 'scale:'||id,'EPISODE_RELEASED','ANIME',id,'{"number":2,"track":"SUB"}',$1
		FROM anime WHERE id::text LIKE '20000000-%'`, now.Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}
	r := db.Repository(s.Token)
	calendarStarted := time.Now()
	calendar, err := r.ReleaseCalendar(ctx, day, 1)
	t.Logf("synthetic calendar duration %v", time.Since(calendarStarted))
	if err != nil || len(calendar.Items) != 100 || !calendar.Truncated {
		t.Fatalf("calendar: %d rows truncated=%t %v", len(calendar.Items), calendar.Truncated, err)
	}
	seen := make(map[string]bool, len(calendar.Items))
	for _, row := range calendar.Items {
		if seen[row.AnimeID] {
			t.Fatalf("calendar repeated %s", row.AnimeID)
		}
		seen[row.AnimeID] = true
	}
	alertsStarted := time.Now()
	for _, expected := range []int{100, titles - 100, 0} {
		page, err := r.NotificationSummary(ctx)
		if err != nil || page.Materialization == nil || page.Materialization.Created != expected {
			t.Fatalf("materialization: %+v %v, want %d", page, err, expected)
		}
	}
	t.Logf("synthetic three-poll alert duration %v", time.Since(alertsStarted))
}
