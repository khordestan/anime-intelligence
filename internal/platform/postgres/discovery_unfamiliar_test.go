package postgres_test

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/khordestan/anime-intelligence/internal/httpapi"
)

func TestDiscoveryUnfamiliarUsesOnlyOwnedLibraryEvidence(t *testing.T) {
	db, owner, _, _ := testDB(t)
	ctx := context.Background()
	first, err := db.Register(ctx, "discovery-owner@example.test", "synthetic-discovery-password")
	if err != nil {
		t.Fatal(err)
	}
	second, err := db.Register(ctx, "discovery-other@example.test", "synthetic-discovery-password")
	if err != nil {
		t.Fatal(err)
	}
	verifyAccount(t, db, first.Token)
	verifyAccount(t, db, second.Token)
	const (
		tracked   = "10000000-0000-4000-8000-000000000001"
		both      = "10000000-0000-4000-8000-000000000002"
		novelOnly = "10000000-0000-4000-8000-000000000003"
		knownOnly = "10000000-0000-4000-8000-000000000004"
	)
	a := db.Repository(first.Token)
	if _, err := a.Add(ctx, tracked, "PLANNING", 0); err != nil {
		t.Fatal(err)
	}
	if _, err := owner.Exec(ctx, `INSERT INTO anime_features(feature_key,display_name,category)
		VALUES('synthetic.familiar','Familiar','SYNTHETIC'),('synthetic.novel','Novel','SYNTHETIC')`); err != nil {
		t.Fatal(err)
	}
	if _, err := owner.Exec(ctx, `INSERT INTO anime_feature_observations
		(anime_id,feature_id,value,confidence,source_kind,evidence,scope,spoiler_level)
		SELECT items.anime_id,f.id,1,1,'CURATED','{"reference":"https://example.test/source"}','SERIES','NONE'
		FROM (VALUES ($1::uuid,'synthetic.familiar'),($2::uuid,'synthetic.familiar'),
		             ($2::uuid,'synthetic.novel'),($3::uuid,'synthetic.novel'),($4::uuid,'synthetic.familiar')) items(anime_id,feature_key)
		JOIN anime_features f USING (feature_key)`, tracked, both, novelOnly, knownOnly); err != nil {
		t.Fatal(err)
	}
	page, err := a.DiscoverWithOptions(ctx, 1, nil, "", true)
	if err != nil || len(page.Items) != 1 || page.Items[0].AnimeID != both ||
		page.Items[0].UnfamiliarCount != 1 || page.Items[0].EvidenceCount != 2 ||
		page.Items[0].FeaturedEvidence[0].Key != "synthetic.novel" ||
		page.NextCursor == nil || !page.NextCursor.Unfamiliar || page.NextCursor.EvidenceCount != 1 {
		t.Fatalf("unexpected unfamiliar ranking: %+v %v", page, err)
	}
	next, err := a.DiscoverWithOptions(ctx, 1, page.NextCursor, "", true)
	if err != nil || len(next.Items) != 1 || next.Items[0].AnimeID != novelOnly || next.NextCursor != nil {
		t.Fatalf("unexpected second page: %+v %v", next, err)
	}
	other, err := db.Repository(second.Token).DiscoverWithOptions(ctx, 20, nil, "", true)
	if err != nil || len(other.Items) != 4 || other.Items[0].AnimeID != both || other.Items[0].UnfamiliarCount != 2 {
		t.Fatalf("library evidence crossed owners: %+v %v", other, err)
	}
	handler := httpapi.NewPostgresRouter(slog.New(slog.NewTextHandler(io.Discard, nil)), db)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/recommendations/discovery?after="+both+"&afterCount=1&afterUnfamiliar=true", nil)
	req.Header.Set("Authorization", "Bearer "+first.Token)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, req)
	if response.Code != 400 {
		t.Fatalf("mismatched cursor status = %d body = %s", response.Code, response.Body)
	}
}
