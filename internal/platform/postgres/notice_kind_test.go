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
	"github.com/khordestan/anime-intelligence/internal/platform/postgres"
)

func TestNotificationKindFiltersBeforePaginationAndRetainsUnreadTotal(t *testing.T) {
	db, owner, _, _ := testDB(t)
	ctx := context.Background()
	session, err := db.Register(ctx, "notification-kind@example.test", "synthetic-notification-password")
	if err != nil {
		t.Fatal(err)
	}
	verifyAccount(t, db, session.Token)
	queueExec(t, owner, `WITH seeded AS (
		INSERT INTO domain_events(event_key,event_type,aggregate_type,aggregate_id,payload,occurred_at)
		SELECT 'notice-kind:'||n, CASE WHEN n=0 THEN 'EPISODE_DELAY_REPORTED' ELSE 'EPISODE_RELEASED' END,
		  'ANIME','10000000-0000-4000-8000-000000000001','{"provider":"synthetic-delay"}', now()+n*interval '1 second'
		FROM generate_series(0,101) n RETURNING id, event_type, occurred_at
	)
	INSERT INTO notifications(user_id,event_id,title,body,created_at) SELECT $1,id,event_type,'Synthetic notice',occurred_at FROM seeded`, session.User.ID)

	handler := httpapi.NewPostgresRouter(slog.New(slog.NewTextHandler(io.Discard, nil)), db)
	call := func(query string) *httptest.ResponseRecorder {
		request := httptest.NewRequest(http.MethodGet, "/api/v1/notifications"+query, nil)
		request.Header.Set("Authorization", "Bearer "+session.Token)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		return response
	}
	response := call("?kind=schedule&unreadOnly=true")
	var page postgres.NotificationPage
	if response.Code != 200 || json.Unmarshal(response.Body.Bytes(), &page) != nil ||
		len(page.Items) != 1 || page.Items[0].Title != "EPISODE_DELAY_REPORTED" ||
		page.Items[0].Provider != "synthetic-delay" || page.UnreadCount != 102 || page.NextCursor != "" {
		t.Fatalf("unexpected filtered response: %d %s", response.Code, response.Body.String())
	}
	response = call("?kind=release")
	if response.Code != 200 || json.Unmarshal(response.Body.Bytes(), &page) != nil || len(page.Items) != 100 || page.NextCursor == "" {
		t.Fatalf("unexpected release page: %d %s", response.Code, response.Body.String())
	}
	response = call("?kind=release&cursor=" + page.NextCursor)
	if response.Code != 200 || json.Unmarshal(response.Body.Bytes(), &page) != nil || len(page.Items) != 1 || page.Items[0].Title != "EPISODE_RELEASED" {
		t.Fatalf("unexpected release history: %d %s", response.Code, response.Body.String())
	}
	for _, query := range []string{"?kind=OTHER", "?kind=", "?kind=release&kind=schedule"} {
		if response := call(query); response.Code != 400 {
			t.Errorf("invalid filter %s: status %d", query, response.Code)
		}
	}
}
