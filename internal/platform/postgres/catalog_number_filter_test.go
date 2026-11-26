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

	"github.com/khordestan/anime-intelligence/internal/httpapi"
	"github.com/khordestan/anime-intelligence/internal/platform/store"
)

func TestCatalogFiltersMatchExactSourcedSeasonAndPart(t *testing.T) {
	db, admin, _, _ := testDB(t)
	if _, err := admin.Exec(context.Background(), `INSERT INTO data_sources(provider_key,display_name)
		VALUES ('catalog-number-fixture','Synthetic');
		INSERT INTO catalog_metadata(anime_id,source_id,payload,observed_at)
		SELECT a.id,s.id,v.payload,now() FROM (VALUES
		 ('1','{"seasonNumber":2,"partNumber":1}'::jsonb),
		 ('2','{"seasonNumber":2,"partNumber":2}'::jsonb),
		 ('3','{"seasonNumber":3,"partNumber":1}'::jsonb),
		 ('4','{"partNumber":1}'::jsonb),
		 ('5','{"seasonNumber":"2","partNumber":1}'::jsonb),
		 ('6','{"seasonNumber":2,"partNumber":[1]}'::jsonb)
		) v(suffix,payload)
		JOIN anime a ON a.id=('10000000-0000-4000-8000-00000000000'||v.suffix)::uuid
		JOIN data_sources s ON s.provider_key='catalog-number-fixture'`); err != nil {
		t.Fatal(err)
	}
	router := httpapi.NewPostgresRouter(slog.New(slog.NewTextHandler(io.Discard, nil)), db)
	type page struct {
		Items      []store.Anime
		NextCursor string
	}
	request := func(query string, status int) page {
		t.Helper()
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest("GET", "/api/v1/anime?"+query, nil))
		if response.Code != status {
			t.Fatalf("%s: status=%d want=%d: %s", query, response.Code, status, response.Body)
		}
		var result page
		if status == http.StatusOK && json.Unmarshal(response.Body.Bytes(), &result) != nil {
			t.Fatalf("invalid response: %s", response.Body)
		}
		return result
	}
	match := request("seasonNumber=2&partNumber=1&metadataProvider=catalog-number-fixture&cursor=start", 200)
	if len(match.Items) != 1 || match.Items[0].ID != "10000000-0000-4000-8000-000000000001" {
		t.Fatalf("season and part selection: %+v", match)
	}
	first := request("partNumber=1&cursor=start&limit=1", 200)
	if len(first.Items) != 1 || first.NextCursor == "" {
		t.Fatalf("first page: %+v", first)
	}
	cursor := url.QueryEscape(first.NextCursor)
	second := request("partNumber=1&cursor="+cursor+"&limit=1", 200)
	if len(second.Items) != 1 || second.Items[0].ID == first.Items[0].ID || second.NextCursor != "" {
		t.Fatalf("second page: %+v", second)
	}
	request("partNumber=2&cursor="+cursor+"&limit=1", 400)
	request("partNumber=0", 400)
	request("seasonNumber=2147483648", 400)
	request("seasonNumber=2&seasonNumber=3", 400)
}
