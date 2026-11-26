package postgres_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/khordestan/anime-intelligence/internal/platform/catalogidentity"
)

func TestCanonicalMergeImpactCountsRLSReferencesWithoutDisclosingOwners(t *testing.T) {
	db, owner, _, runtimeDSN := testDB(t)
	ctx := context.Background()
	const a = "10000000-0000-4000-8000-000000000001"
	const b = "10000000-0000-4000-8000-000000000002"
	var from, to pgtype.UUID
	if from.Scan(a) != nil || to.Scan(b) != nil {
		t.Fatal("invalid fixture IDs")
	}
	session, err := db.Register(ctx, "merge-impact@example.test", "synthetic-merge-impact-password")
	if err != nil {
		t.Fatal(err)
	}
	verifyAccount(t, db, session.Token)
	for _, title := range []string{a, b} {
		repo := db.Repository(session.Token)
		if _, err := repo.Add(ctx, title, "PLANNING", 0); err != nil {
			t.Fatal(err)
		}
		if ok, err := repo.Follow(ctx, title, "SUB"); err != nil || !ok {
			t.Fatalf("follow: %t %v", ok, err)
		}
	}
	queueExec(t, owner, `INSERT INTO anime_relations(source_anime_id,target_anime_id,relation_type) VALUES($1,$2,'SEQUEL')`, a, b)
	queueExec(t, owner, `INSERT INTO domain_events(id,event_key,event_type,aggregate_type,aggregate_id,payload)
		VALUES('30000000-0000-4000-8000-000000000010','synthetic-merge-review','EPISODE_RELEASED','ANIME',$1,'{}')`, a)
	queueExec(t, owner, `INSERT INTO notifications(user_id,event_id,title,body)
		VALUES($1,'30000000-0000-4000-8000-000000000010','Synthetic notice','Synthetic watch history')`, session.User.ID)

	queueExec(t, owner, `UPDATE anime SET episode_count=0,release_year=NULL WHERE id=$1`, a)
	queueExec(t, owner, `UPDATE anime SET episode_count=NULL,release_year=2026 WHERE id=$1`, b)

	report, err := catalogidentity.InspectMergeImpact(ctx, owner, from, to)
	if err != nil {
		t.Fatal(err)
	}
	counts := map[string][2]int64{}
	for _, item := range report.References {
		counts[item.Dependency] = [2]int64{item.Source, item.Target}
	}
	for dependency, expected := range map[string][2]int64{
		"public.user_anime.anime_id": {1, 1}, "public.user_follows.anime_id": {1, 1},
		"public.anime_relations.source_anime_id": {1, 0}, "public.anime_relations.target_anime_id": {0, 1},
		"public.domain_events.aggregate_id (ANIME)": {1, 0}, "public.notifications.event_id": {1, 0},
	} {
		if got, ok := counts[dependency]; !ok || got != expected {
			t.Errorf("%s: got %v, want %v", dependency, got, expected)
		}
	}
	conflicts := map[string]int64{}
	for _, conflict := range report.Conflicts {
		conflicts[conflict.Kind] = conflict.Count
	}
	for _, kind := range []string{"owners_tracking_both", "overlapping_follow_scope", "links_becoming_self_references"} {
		if conflicts[kind] != 1 {
			t.Errorf("%s: got %d conflicts, want 1", kind, conflicts[kind])
		}
	}
	facts := map[string]catalogidentity.MergePublicFact{}
	for _, fact := range report.PublicFacts {
		facts[fact.Field] = fact
	}
	if episode := facts["episodeCount"]; episode.Comparison != "SOURCE_ONLY" || episode.Source == nil || *episode.Source != "0" || episode.Target != nil {
		t.Fatalf("known zero total lost in merge comparison: %+v", episode)
	}
	if year := facts["releaseYear"]; year.Comparison != "TARGET_ONLY" || year.Target == nil || *year.Target != "2026" {
		t.Fatalf("one-sided year: %+v", year)
	}
	if facts["canonicalTitle"].Comparison != "DIFFERENT" || facts["mediaType"].Comparison != "EQUAL" || facts["primaryProvider"].Comparison != "UNKNOWN" {
		t.Fatalf("merge fact comparisons: %+v", facts)
	}
	serialized, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	for _, private := range []string{session.User.ID, session.User.Email, "Synthetic watch history"} {
		if strings.Contains(string(serialized), private) {
			t.Fatal("aggregate report exposed private data")
		}
	}
	runtimeConn, err := pgx.Connect(ctx, runtimeDSN)
	if err != nil {
		t.Fatal(err)
	}
	defer runtimeConn.Close(ctx)
	if _, err := catalogidentity.InspectMergeImpact(ctx, runtimeConn, from, to); err == nil {
		t.Fatal("restricted runtime could return incomplete impact counts")
	}
}
