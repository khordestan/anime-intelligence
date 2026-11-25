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

func TestCatalogConfirmedReleaseUsesLatestTrackFactsAndBindsCursor(t *testing.T) {
	db, admin, _, _ := testDB(t)
	if _, err := admin.Exec(context.Background(), `INSERT INTO airing_schedule
		(anime_id,episode_number,release_track,scheduled_at,actual_released_at,revision) VALUES
		('10000000-0000-4000-8000-000000000001',1,'SUB',NULL,now()-interval '1 day',1),
		('10000000-0000-4000-8000-000000000002',1,'DUB',NULL,now()-interval '1 day',1),
		('10000000-0000-4000-8000-000000000003',1,'SUB',now()-interval '1 day',NULL,1),
		('10000000-0000-4000-8000-000000000004',1,'SUB',NULL,now()-interval '1 day',1),
		('10000000-0000-4000-8000-000000000004',1,'SUB',now()-interval '1 day',NULL,2),
		('10000000-0000-4000-8000-000000000005',1,'SUB',NULL,now()+interval '1 day',1)`); err != nil {
		t.Fatal(err)
	}
	handler := httpapi.NewPostgresRouter(slog.New(slog.NewTextHandler(io.Discard, nil)), db)
	type page struct {
		Items      []store.Anime `json:"items"`
		NextCursor string        `json:"nextCursor"`
	}
	request := func(query string, status int) page {
		t.Helper()
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/anime?"+query, nil))
		if recorder.Code != status {
			t.Fatalf("status=%d want=%d: %s", recorder.Code, status, recorder.Body)
		}
		var result page
		if status == 200 && json.Unmarshal(recorder.Body.Bytes(), &result) != nil {
			t.Fatalf("invalid catalog response: %s", recorder.Body)
		}
		return result
	}
	first := request("confirmedRelease=true&cursor=start&limit=1", 200)
	if len(first.Items) != 1 || first.Items[0].ID != "10000000-0000-4000-8000-000000000002" || first.NextCursor == "" {
		t.Fatalf("first page: %+v", first)
	}
	cursor := url.QueryEscape(first.NextCursor)
	second := request("confirmedRelease=true&cursor="+cursor+"&limit=1", 200)
	if len(second.Items) != 1 || second.Items[0].ID != "10000000-0000-4000-8000-000000000001" || second.NextCursor != "" {
		t.Fatalf("second page: %+v", second)
	}
	request("cursor="+cursor+"&limit=1", 400)
	request("confirmedRelease=1", 400)
	request("confirmedRelease=true&confirmedRelease=false", 400)
}
