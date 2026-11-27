package postgres_test

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/khordestan/anime-intelligence/internal/modules/catalog/feed"
	"github.com/khordestan/anime-intelligence/internal/platform/catalogsync"
)

func TestReviewedMetadataMergeBindsObservationAndPreservesAuthority(t *testing.T) {
	db, owner, _, runtimeDSN := testDB(t)
	ctx := context.Background()
	reviewer := connectMergeReviewer(t, owner, runtimeDSN)
	runtime, err := pgx.Connect(ctx, runtimeDSN)
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Close(ctx)

	const retired = "30000000-0000-4000-8000-000000000110"
	const survivor = "30000000-0000-4000-8000-000000000111"
	const mapping = "30000000-0000-4000-8000-000000000112"
	const reference = "https://example.test/review/metadata-merge"
	queueExec(t, owner, `INSERT INTO anime(id,canonical_title) VALUES($1,'Synthetic metadata duplicate')`, retired)
	queueExec(t, owner, `INSERT INTO data_sources(provider_key,display_name,enabled,policy_url)
		VALUES('synthetic-metadata-merge','Synthetic metadata source',true,'https://example.test/policy')`)
	queueExec(t, owner, `INSERT INTO anime_external_ids(id,anime_id,provider,external_id)
		VALUES($1,$2,'synthetic-metadata-merge','duplicate')`, mapping, retired)

	now := time.Now().UTC().Truncate(time.Microsecond)
	snapshot := feed.Snapshot{Version: 1, Provider: "synthetic-metadata-merge", ObservedAt: now.Add(-time.Hour), Titles: []feed.Title{
		{ExternalID: "duplicate", Title: "Synthetic metadata duplicate", NativeTitle: "Synthetic native",
			MediaType: "TV", Status: "FINISHED", ReleaseYear: 2025, Genres: []string{"Synthetic first genre"}},
	}}
	if _, err := catalogsync.Import(ctx, owner, snapshot, now); err != nil {
		t.Fatal(err)
	}
	queueExec(t, owner, `INSERT INTO anime(id,canonical_title,native_title,synopsis,media_type,release_year,season_name,status,episode_count,cover_image_url)
		SELECT $2,canonical_title,native_title,synopsis,media_type,release_year,season_name,status,episode_count,cover_image_url
		FROM anime WHERE id=$1`, retired, survivor)

	review := func(conn *pgx.Conn, digest string, ids []string, apply bool) ([]string, string, error) {
		var mapped []string
		var current string
		err := conn.QueryRow(ctx, `SELECT ARRAY(SELECT id::text FROM unnest(mapping_ids) id),observation_digest
			FROM public.review_catalog_metadata_merge($1::text::uuid,$2::text::uuid,$3::text[]::uuid[],$4,$5,$6)`,
			retired, survivor, ids, digest, reference, apply).Scan(&mapped, &current)
		return mapped, current, err
	}
	preview := func() string {
		t.Helper()
		ids, digest, err := review(reviewer, "", []string{}, false)
		if err != nil || len(ids) != 1 || ids[0] != mapping || len(digest) != 64 {
			t.Fatalf("metadata preview: %v %s %v", ids, digest, err)
		}
		return digest
	}
	first := preview()
	if _, _, err := review(runtime, first, []string{mapping}, true); err == nil {
		t.Fatal("restricted API runtime could apply a reviewed catalog merge")
	}
	if _, err := reviewer.Exec(ctx, "UPDATE catalog_metadata SET payload='{}'"); err == nil {
		t.Fatal("merge reviewer received direct observation writes")
	}
	snapshot.ObservedAt = now.Add(-30 * time.Minute)
	snapshot.Titles[0].Genres[0] = "Synthetic revised genre"
	if _, err := catalogsync.Import(ctx, owner, snapshot, now); err != nil {
		t.Fatal(err)
	}
	queueExec(t, owner, "UPDATE anime SET native_title='Synthetic different' WHERE id=$1", survivor)
	if _, _, err := review(reviewer, "", []string{}, false); err == nil || !strings.Contains(err.Error(), "matching public title fields") {
		t.Fatalf("preview accepted differing public title fields: %v", err)
	}
	queueExec(t, owner, "UPDATE anime SET native_title='Synthetic native' WHERE id=$1", survivor)
	current := preview()
	if first == current {
		t.Fatal("review digest did not bind the observation")
	}
	if _, _, err := review(reviewer, first, []string{mapping}, true); err == nil || !strings.Contains(err.Error(), "changed since review") {
		t.Fatalf("stale metadata digest accepted: %v", err)
	}
	if _, _, err := review(reviewer, current, []string{}, true); err == nil {
		t.Fatal("review accepted missing mapping IDs")
	}

	account, err := db.Register(ctx, "metadata-merge-fixture@example.test", "synthetic-metadata-merge-password")
	if err != nil {
		t.Fatal(err)
	}
	verifyAccount(t, db, account.Token)
	repo := db.Repository(account.Token)
	planned, err := repo.Add(ctx, retired, "PLANNING", 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := review(reviewer, current, []string{mapping}, true); err == nil || !strings.Contains(err.Error(), "dependent records") {
		t.Fatalf("private dependency did not refuse the merge: %v", err)
	}
	if got := preview(); got != current {
		t.Fatal("failed merges changed or removed the source observation")
	}
	if removed, err := repo.Remove(ctx, planned.ID); err != nil || !removed {
		t.Fatal("fixture removal", err)
	}
	for range 2 {
		ids, digest, err := review(reviewer, current, []string{mapping}, true)
		if err != nil || len(ids) != 1 || ids[0] != mapping || digest != current {
			t.Fatalf("metadata apply/replay: %v %s %v", ids, digest, err)
		}
	}
	public := db.Repository("")
	details, found, err := public.CatalogDetails(ctx, retired)
	if err != nil || !found || details.Anime.ID != survivor || details.Provider != snapshot.Provider ||
		details.ObservedAt == nil || !details.ObservedAt.Equal(snapshot.ObservedAt) ||
		len(details.Genres) != 1 || details.Genres[0] != "Synthetic revised genre" ||
		len(details.ExternalIDs) != 1 || details.ExternalIDs[0].ExternalID != "duplicate" {
		t.Fatalf("metadata or source attribution was lost: %+v %t %v", details, found, err)
	}
	snapshot.ObservedAt = now.Add(-10 * time.Minute)
	snapshot.Titles[0].Genres[0] = "Synthetic post-merge genre"
	if _, err := catalogsync.Import(ctx, owner, snapshot, now); err != nil {
		t.Fatal("exact provider refresh could not find transferred authority", err)
	}
	refreshed, found, err := public.CatalogDetails(ctx, retired)
	if err != nil || !found || refreshed.Anime.ID != survivor || len(refreshed.Genres) != 1 || refreshed.Genres[0] != "Synthetic post-merge genre" {
		t.Fatalf("refresh did not preserve the redirect: %+v %t %v", refreshed, found, err)
	}
	rollbackSQL, err := os.ReadFile("../../../migrations/000033_reviewed_metadata_merge.down.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := owner.Exec(ctx, string(rollbackSQL)); err == nil || !strings.Contains(err.Error(), "refusing to drop") {
		t.Fatalf("rollback discarded recorded merge evidence: %v", err)
	}
}
