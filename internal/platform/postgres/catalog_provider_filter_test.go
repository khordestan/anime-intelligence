package postgres_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/khordestan/anime-intelligence/internal/httpapi"
	"github.com/khordestan/anime-intelligence/internal/modules/catalog/feed"
	"github.com/khordestan/anime-intelligence/internal/platform/catalogsync"
	"github.com/khordestan/anime-intelligence/internal/platform/store"
)

func TestCatalogMetadataProviderFacetsAndCursors(t *testing.T) {
	db, owner, _, _ := testDB(t)
	ctx := context.Background()
	now := time.Now().UTC()
	queueExec(t, owner, `INSERT INTO data_sources(provider_key,display_name,enabled,policy_url)
		VALUES('synthetic-primary','Synthetic primary',true,'https://example.test/policy'),
		      ('synthetic-unused','Synthetic unused',false,'https://example.test/policy')`)
	snapshot := feed.Snapshot{Version: 1, Provider: "synthetic-primary", ObservedAt: now, Titles: []feed.Title{
		{ExternalID: "one", Title: "Synthetic A", MediaType: "TV", Status: "FINISHED"},
		{ExternalID: "two", Title: "Synthetic B", MediaType: "TV", Status: "FINISHED"},
	}}
	if _, err := catalogsync.Import(ctx, owner, snapshot, now); err != nil {
		t.Fatal(err)
	}
	facets, err := db.Repository("").CatalogFacets(ctx, "providers", "SYNTHETIC-", 1)
	if err != nil || len(facets.Items) != 1 || facets.Items[0] != "synthetic-primary" || facets.HasMore {
		t.Fatalf("provider facets: %+v %v", facets, err)
	}
	handler := httpapi.NewPostgresRouter(slog.New(slog.NewTextHandler(io.Discard, nil)), db)
	type page struct {
		Items      []store.Anime
		NextCursor string
	}
	search := func(query string, wantStatus int) page {
		t.Helper()
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/anime?"+query, nil))
		if recorder.Code != wantStatus {
			t.Fatalf("provider search %s: %d %s", query, recorder.Code, recorder.Body)
		}
		var result page
		if wantStatus == 200 && json.Unmarshal(recorder.Body.Bytes(), &result) != nil {
			t.Fatal("invalid catalog response")
		}
		return result
	}
	first := search("metadataProvider=synthetic-primary&cursor=start&limit=1", 200)
	if len(first.Items) != 1 || first.Items[0].Title != "Synthetic A" || first.NextCursor == "" {
		t.Fatalf("initial page: %+v", first)
	}
	cursor := url.QueryEscape(first.NextCursor)
	second := search("metadataProvider=synthetic-primary&cursor="+cursor, 200)
	if len(second.Items) != 1 || second.Items[0].Title != "Synthetic B" {
		t.Fatalf("next page: %+v", second)
	}
	search("metadataProvider=synthetic-unused&cursor="+cursor, 400)
	search("metadataProvider=synthetic-primary&metadataProvider=synthetic-unused", 400)
	search("metadataProvider=%21", 400)
}
