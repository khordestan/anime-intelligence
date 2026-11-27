package postgres_test

import (
	"context"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
)

func connectMergeReviewer(t *testing.T, owner *pgx.Conn, runtimeDSN string) *pgx.Conn {
	t.Helper()
	ctx := context.Background()
	u, err := url.Parse(runtimeDSN)
	if err != nil {
		t.Fatal(err)
	}
	name := u.User.Username() + "_merge"
	role := pgx.Identifier{name}.Sanitize()
	grants, err := os.ReadFile("../../../deployments/catalog-merge-reviewer-role.sql")
	if err != nil {
		t.Fatal(err)
	}
	queueExec(t, owner, strings.ReplaceAll(string(grants), "anime_catalog_merge_reviewer", role))
	t.Cleanup(func() {
		_, _ = owner.Exec(ctx, "DROP OWNED BY "+role)
		_, _ = owner.Exec(ctx, "DROP ROLE "+role)
	})
	password, _ := u.User.Password()
	queueExec(t, owner, "ALTER ROLE "+role+" PASSWORD '"+password+"'")
	u.User = url.UserPassword(name, password)
	reviewer, err := pgx.Connect(ctx, u.String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = reviewer.Close(ctx) })
	return reviewer
}

func TestPlaceholderMergeRechecksMappingsAndPreservesPublicIdentity(t *testing.T) {
	db, owner, _, runtimeDSN := testDB(t)
	ctx := context.Background()
	reviewer := connectMergeReviewer(t, owner, runtimeDSN)
	runtime, err := pgx.Connect(ctx, runtimeDSN)
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Close(ctx)

	const placeholder = "30000000-0000-4000-8000-000000000096"
	const survivor = "10000000-0000-4000-8000-000000000001"
	const occupied = "10000000-0000-4000-8000-000000000002"
	const mapping = "30000000-0000-4000-8000-000000000097"
	queueExec(t, owner, "INSERT INTO anime(id,canonical_title) VALUES($1,'Synthetic duplicate placeholder')", placeholder)
	queueExec(t, owner, `INSERT INTO anime_external_ids(id,anime_id,provider,external_id)
		VALUES($1,$2,'synthetic-merge-source','duplicate-one')`, mapping, placeholder)
	review := func(conn *pgx.Conn, retired string, expected []string, apply bool) ([]string, error) {
		var result []string
		err := conn.QueryRow(ctx, `SELECT ARRAY(SELECT id::text FROM unnest(public.review_catalog_placeholder_merge(
			$1::text::uuid,$2::text::uuid,$3::text[]::uuid[],'https://example.test/review/duplicate', $4)) AS id)`,
			retired, survivor, expected, apply).Scan(&result)
		return result, err
	}
	if ids, err := review(reviewer, placeholder, []string{}, false); err != nil || len(ids) != 1 || ids[0] != mapping {
		t.Fatalf("merge preview: %v %v", ids, err)
	}
	if _, err := review(runtime, placeholder, []string{mapping}, true); err == nil {
		t.Fatal("runtime role could execute a reviewed merge")
	}
	if _, err := review(reviewer, placeholder, []string{}, true); err == nil || !strings.Contains(err.Error(), "changed since review") {
		t.Fatalf("stale mapping list was accepted: %v", err)
	}
	session, err := db.Register(ctx, "merge-guard@example.test", "synthetic-placeholder-merge-password")
	if err != nil {
		t.Fatal(err)
	}
	verifyAccount(t, db, session.Token)
	if _, err := db.Repository(session.Token).Add(ctx, occupied, "PLANNING", 0); err != nil {
		t.Fatal(err)
	}
	if _, err := review(reviewer, occupied, []string{}, true); err == nil || !strings.Contains(err.Error(), "dependent records") {
		t.Fatalf("merge could discard a private dependency: %v", err)
	}
	for range 2 {
		if ids, err := review(reviewer, placeholder, []string{mapping}, true); err != nil || len(ids) != 1 || ids[0] != mapping {
			t.Fatalf("merge and replay: %v %v", ids, err)
		}
	}
	repo := db.Repository("")
	if got, found, err := repo.GetAnime(ctx, placeholder); err != nil || !found || got.ID != survivor {
		t.Fatalf("canonical redirect: %+v %t %v", got, found, err)
	}
	if got, found, err := repo.CatalogDetails(ctx, placeholder); err != nil || !found || got.Anime.ID != survivor || len(got.ExternalIDs) != 1 {
		t.Fatalf("details after merge: %+v %t %v", got, found, err)
	}
	if got, found, err := repo.CatalogContinuity(ctx, placeholder); err != nil || !found || got.RootID != survivor {
		t.Fatalf("continuity redirect: %+v %t %v", got, found, err)
	}
	if _, err := owner.Exec(ctx, "INSERT INTO anime(id,canonical_title) VALUES($1,'Synthetic revived UUID')", placeholder); err == nil {
		t.Fatal("retired UUID was available for insertion")
	}
	if titles, err := repo.Catalog(ctx, "Synthetic duplicate placeholder", 10, 0); err != nil || len(titles) != 0 {
		t.Fatalf("retired catalog row remains discoverable: %+v %v", titles, err)
	}
}
