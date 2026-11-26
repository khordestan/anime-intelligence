package postgres_test

import (
	"context"
	"errors"
	"net/url"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/khordestan/anime-intelligence/internal/modules/catalog/feed"
	"github.com/khordestan/anime-intelligence/internal/platform/catalogsync"
	"github.com/khordestan/anime-intelligence/internal/platform/postgres"
)

func reviewedContinuityDB(t *testing.T) (*pgx.Conn, *pgx.Conn, *pgx.Conn, string) {
	t.Helper()
	owner, writer, _, runtimeDSN := queuedDB(t)
	ctx := context.Background()
	u, err := url.Parse(runtimeDSN)
	if err != nil {
		t.Fatal(err)
	}
	role := u.User.Username() + "_continuity"
	quotedRole := pgx.Identifier{role}.Sanitize()
	script, err := os.ReadFile("../../../deployments/catalog-continuity-curator-role.sql")
	if err != nil {
		t.Fatal(err)
	}
	queueExec(t, owner, strings.ReplaceAll(string(script), "anime_catalog_continuity_curator", quotedRole))
	t.Cleanup(func() {
		_, _ = owner.Exec(ctx, "DROP OWNED BY "+quotedRole)
		_, _ = owner.Exec(ctx, "DROP ROLE "+quotedRole)
	})
	password, _ := u.User.Password()
	queueExec(t, owner, "ALTER ROLE "+quotedRole+" PASSWORD '"+password+"'")
	u.User = url.UserPassword(role, password)
	curator, err := pgx.Connect(ctx, u.String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = curator.Close(ctx) })
	if err := catalogsync.CheckWriter(ctx, writer); err != nil {
		t.Fatal("restricted writer rejected:", err)
	}

	return owner, writer, curator, runtimeDSN
}

func TestReviewedContinuityRetractionRejectsStaleRowsAndSurvivesRefresh(t *testing.T) {
	owner, writer, curator, runtimeDSN := reviewedContinuityDB(t)
	ctx := context.Background()

	queueExec(t, owner, `INSERT INTO data_sources(provider_key,display_name,policy_url,enabled)
		VALUES('synthetic-continuity','Synthetic continuity','https://example.test/policy',true)`)
	now := time.Now().UTC().Truncate(time.Microsecond)
	snapshot := feed.Snapshot{Version: 1, Provider: "synthetic-continuity", ObservedAt: now.Add(-time.Hour), Titles: []feed.Title{
		{ExternalID: "one", Title: "Synthetic Prequel", MediaType: "TV", Status: "FINISHED", Relations: []feed.Relation{{ExternalID: "two", Type: "SEQUEL"}, {ExternalID: "two", Type: "SIDE_STORY"}}},
		{ExternalID: "two", Title: "Synthetic Sequel", MediaType: "TV", Status: "ANNOUNCED", Relations: []feed.Relation{{ExternalID: "one", Type: "PREQUEL"}}},
	}}
	if _, err := catalogsync.Import(ctx, writer, snapshot, now); err != nil {
		t.Fatal(err)
	}
	var first, second string
	if err := owner.QueryRow(ctx, `SELECT a.anime_id::text,b.anime_id::text FROM anime_external_ids a CROSS JOIN anime_external_ids b
		WHERE a.provider=$1 AND b.provider=$1 AND a.external_id='one' AND b.external_id='two'`, snapshot.Provider).Scan(&first, &second); err != nil {
		t.Fatal(err)
	}
	const query = `SELECT ARRAY(SELECT id::text FROM unnest(public.review_continuity_retraction($1::text::uuid,
		$2::text::uuid,$3::text[]::uuid[],'synthetic-review', $4)) AS id)`
	review := func(conn *pgx.Conn, expected []string, apply bool) ([]string, error) {
		var ids []string
		err := conn.QueryRow(ctx, query, first, second, expected, apply).Scan(&ids)
		return ids, err
	}

	// A collector session can create temporary tables. The privileged review
	// still resolves the canonical tables ahead of any temporary objects.
	queueExec(t, curator, "CREATE TEMP TABLE anime_relations(id uuid)")
	queueExec(t, curator, "CREATE TEMP TABLE catalog_continuity_retractions(prequel_id uuid)")
	ids, err := review(curator, []string{}, false)
	if err != nil || len(ids) != 2 {
		t.Fatalf("preview: %v %v", ids, err)
	}
	if _, err := review(curator, ids[:1], true); err == nil {
		t.Fatal("accepted a stale set of relation IDs")
	}
	if _, err := review(writer, ids, true); err == nil {
		t.Fatal("writer can apply a curator decision")
	}
	queueExec(t, owner, `GRANT EXECUTE ON FUNCTION review_continuity_retraction(uuid,uuid,uuid[],text,boolean) TO PUBLIC`)
	if _, err := review(writer, ids, true); err == nil {
		t.Fatal("default restored function permissions allowed a retraction")
	}
	queueExec(t, owner, `REVOKE ALL ON FUNCTION review_continuity_retraction(uuid,uuid,uuid[],text,boolean) FROM PUBLIC`)
	if _, err := curator.Exec(ctx, "DELETE FROM public.anime_relations"); err == nil {
		t.Fatal("curator can delete arbitrary relations")
	}
	for range 2 {
		if applied, err := review(curator, ids, true); err != nil || !reflect.DeepEqual(ids, applied) {
			t.Fatalf("apply or replay: %v %v", applied, err)
		}
	}
	if _, err := writer.Exec(ctx, `INSERT INTO anime_relations(source_anime_id,target_anime_id,relation_type)
		VALUES($1,$2,'PREQUEL')`, second, first); err == nil {
		t.Fatal("legacy write resurrected reciprocal edge")
	} else {
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.ConstraintName != "catalog_continuity_retired" {
			t.Fatal(err)
		}
	}
	snapshot.ObservedAt = now
	snapshot.Titles[0].Title = "Synthetic updated prequel"
	if _, err := catalogsync.Import(ctx, writer, snapshot, now); err != nil {
		t.Fatal("refresh could not advance past a retired link:", err)
	}
	db, err := postgres.Open(ctx, runtimeDSN)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	graph, found, err := db.Repository("").CatalogContinuity(ctx, first)
	if err != nil || !found || len(graph.Nodes) != 1 || len(graph.Edges) != 0 || graph.Nodes[0].Title != snapshot.Titles[0].Title {
		t.Fatalf("continuity view after refresh: %+v %v", graph, err)
	}
	down, err := os.ReadFile("../../../migrations/000028_continuity_retractions.down.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := owner.Exec(ctx, string(down)); err == nil {
		t.Fatal("downgrade dropped the retraction audit")
	}
	var retained int
	if err := owner.QueryRow(ctx, `SELECT count(*) FROM anime_relations WHERE source_anime_id=$1 AND
		target_anime_id=$2 AND relation_type='SIDE_STORY'`, first, second).Scan(&retained); err != nil || retained != 1 {
		t.Fatalf("side story relation changed: %d %v", retained, err)
	}
}
