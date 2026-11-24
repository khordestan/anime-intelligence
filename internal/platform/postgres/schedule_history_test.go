package postgres_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/khordestan/anime-intelligence/internal/httpapi"
	"github.com/khordestan/anime-intelligence/internal/platform/store"
)

func TestCatalogScheduleHistoryPublicAndTrackScoped(t *testing.T) {
	db, owner, _, _ := testDB(t)
	ctx := context.Background()
	const id = "10000000-0000-4000-8000-000000000001"
	if _, err := owner.Exec(ctx, `INSERT INTO airing_schedule
		(anime_id,episode_number,release_track,scheduled_at,actual_released_at,revision)
		SELECT $1::uuid,1,'SUB',now()+n*interval '1 hour',NULL,n FROM generate_series(1,22) n
		UNION ALL SELECT $1::uuid,1,'DUB',now(),now(),1
		UNION ALL SELECT $1::uuid,2,'SUB',now(),now(),1`, id); err != nil {
		t.Fatal(err)
	}
	handler := httpapi.NewPostgresRouter(slog.New(slog.NewTextHandler(io.Discard, nil)), db)
	request := func(query string, status int) store.ScheduleHistory {
		t.Helper()
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet,
			"/api/v1/anime/"+id+"/episodes/1/schedule-history"+query, nil))
		if recorder.Code != status {
			t.Fatalf("%s status = %d body = %s", query, recorder.Code, recorder.Body)
		}
		var result store.ScheduleHistory
		if status == 200 && json.Unmarshal(recorder.Body.Bytes(), &result) != nil {
			t.Fatalf("invalid history response: %s", recorder.Body)
		}
		return result
	}
	page := request("?track=SUB", 200)
	if len(page.Items) != 20 || page.NextBeforeRevision != 3 {
		t.Fatalf("unexpected first page: %+v", page)
	}
	for i, item := range page.Items {
		if item.Revision != 22-i || item.Number != 1 || item.Track != "SUB" || item.ScheduledAt == nil || item.ReleasedAt != nil {
			t.Fatalf("unexpected schedule row: %+v", item)
		}
	}
	older := request("?track=SUB&beforeRevision=3", 200)
	if len(older.Items) != 2 || older.Items[0].Revision != 2 || older.Items[1].Revision != 1 || older.NextBeforeRevision != 0 {
		t.Fatalf("unexpected older page: %+v", older)
	}
	dub := request("?track=DUB", 200)
	if len(dub.Items) != 1 || dub.Items[0].ReleasedAt == nil {
		t.Fatalf("unexpected DUB page: %+v", dub)
	}
	for _, query := range []string{"", "?track=ALL", "?track=SUB&track=DUB", "?track=SUB&beforeRevision=0", "?track=SUB&beforeRevision=1&beforeRevision=2"} {
		request(query, 400)
	}
}
