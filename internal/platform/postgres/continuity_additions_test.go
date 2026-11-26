package postgres_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/khordestan/anime-intelligence/internal/modules/catalog/feed"
	"github.com/khordestan/anime-intelligence/internal/platform/catalogsync"
	"github.com/khordestan/anime-intelligence/internal/platform/postgres"
)

func TestReviewedContinuityAdditionPreservesAuthorityAndRejectsCycles(t *testing.T) {
	owner, writer, curator, runtimeDSN := reviewedContinuityDB(t)
	ctx := context.Background()
	queueExec(t, owner, `INSERT INTO data_sources(provider_key,display_name,policy_url,enabled) VALUES
		('synthetic-main','Synthetic primary','https://example.test/primary',true),
		('synthetic-review','Synthetic reviewed','https://example.test/review',true)`)
	now := time.Now().UTC()
	feedSnapshot := feed.Snapshot{Version: 1, Provider: "synthetic-main", ObservedAt: now.Add(-time.Minute), Titles: []feed.Title{
		{ExternalID: "one", Title: "Synthetic first", MediaType: "TV", Status: "FINISHED"},
		{ExternalID: "two", Title: "Synthetic second", MediaType: "TV", Status: "ANNOUNCED"},
	}}
	if _, err := catalogsync.Import(ctx, writer, feedSnapshot, now); err != nil {
		t.Fatal(err)
	}
	var first, second string
	if err := owner.QueryRow(ctx, `SELECT a.anime_id::text,b.anime_id::text FROM anime_external_ids a CROSS JOIN anime_external_ids b
		WHERE a.provider=$1 AND b.provider=$1 AND a.external_id='one' AND b.external_id='two'`, feedSnapshot.Provider).Scan(&first, &second); err != nil {
		t.Fatal(err)
	}
	add := func(c *pgx.Conn, before, after, reference string, apply bool) (*string, error) {
		var id *string
		err := c.QueryRow(ctx, `SELECT public.review_continuity_addition($1::text::uuid,$2::text::uuid,'synthetic-review',$3,$4)::text`,
			before, after, reference, apply).Scan(&id)
		return id, err
	}
	const reference = "https://example.test/review/one"
	if id, err := add(curator, first, second, reference, false); err != nil || id != nil {
		t.Fatalf("unexpected preview result: %v %v", id, err)
	}
	if _, err := add(writer, first, second, reference, true); err == nil {
		t.Fatal("import writer could curate a continuity link")
	}
	id, err := add(curator, first, second, reference, true)
	if err != nil || id == nil {
		t.Fatalf("apply: %v %v", id, err)
	}
	if replay, err := add(curator, first, second, reference, true); err != nil || replay == nil || *replay != *id {
		t.Fatalf("review replay: %v %v", replay, err)
	}
	if _, err := add(curator, first, second, "different-reference", true); err == nil {
		t.Fatal("review was overwritten")
	}
	if _, err := add(curator, second, first, reference, true); err == nil || !strings.Contains(err.Error(), "cycle") {
		t.Fatalf("cycle was accepted: %v", err)
	}
	feedSnapshot.ObservedAt = now
	feedSnapshot.Titles[0].Relations = []feed.Relation{{ExternalID: "two", Type: "SEQUEL"}}
	if _, err := catalogsync.Import(ctx, writer, feedSnapshot, now); err != nil {
		t.Fatal("coincident provider claim blocked refresh:", err)
	}
	db, err := postgres.Open(ctx, runtimeDSN)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	graph, found, err := db.Repository("").CatalogContinuity(ctx, first)
	if err != nil || !found || len(graph.Edges) != 1 || len(graph.Edges[0].Providers) != 1 || graph.Edges[0].Providers[0] != "synthetic-review" {
		t.Fatalf("published attribution: %+v %v", graph, err)
	}
	queueExec(t, curator, `SELECT public.review_continuity_retraction($1::text::uuid,$2::text::uuid,ARRAY[$3::text::uuid],'synthetic withdrawal',true)`,
		first, second, *id)
	if _, err := add(curator, first, second, reference, true); err == nil || !strings.Contains(err.Error(), "retired") {
		t.Fatalf("retracted link could be restored: %v", err)
	}
}
