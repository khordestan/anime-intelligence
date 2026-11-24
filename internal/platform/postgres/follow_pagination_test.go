package postgres_test

import (
	"context"
	"io"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/khordestan/anime-intelligence/internal/httpapi"
)

func TestFollowPaginationRetainsOwnerAndTieOrder(t *testing.T) {
	db, owner, _, _ := testDB(t)
	ctx := context.Background()
	first, err := db.Register(ctx, "follows-page@example.test", "synthetic-alerts-password")
	if err != nil {
		t.Fatal(err)
	}
	second, err := db.Register(ctx, "follows-foreign@example.test", "synthetic-alerts-password")
	if err != nil {
		t.Fatal(err)
	}
	verifyAccount(t, db, first.Token)
	verifyAccount(t, db, second.Token)
	a, b := db.Repository(first.Token), db.Repository(second.Token)
	const (
		one = "10000000-0000-4000-8000-000000000001"
		two = "10000000-0000-4000-8000-000000000002"
	)
	for _, id := range []string{one, two} {
		if ok, err := a.Follow(ctx, id, "SUB"); err != nil || !ok {
			t.Fatalf("follow %s: ok=%t err=%v", id, ok, err)
		}
	}
	if ok, err := b.Follow(ctx, two, "DUB"); err != nil || !ok {
		t.Fatalf("foreign follow: ok=%t err=%v", ok, err)
	}
	if _, err := owner.Exec(ctx, `UPDATE user_follows SET created_at=now() WHERE user_id=$1`, first.User.ID); err != nil {
		t.Fatal(err)
	}
	legacy, err := a.Follows(ctx)
	if err != nil || len(legacy) != 2 {
		t.Fatalf("full follow list: %+v err=%v", legacy, err)
	}
	page, err := a.FollowPage(ctx, "", 1)
	if err != nil || len(page.Items) != 1 || page.Items[0].AnimeID != legacy[0].AnimeID || page.NextCursor == "" {
		t.Fatalf("first follow page: %+v err=%v", page, err)
	}
	next, err := a.FollowPage(ctx, page.NextCursor, 1)
	if err != nil || len(next.Items) != 1 || next.Items[0].AnimeID != legacy[1].AnimeID || next.NextCursor != "" {
		t.Fatalf("second follow page: %+v err=%v", next, err)
	}
	foreign, err := b.FollowPage(ctx, page.NextCursor, 1)
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range foreign.Items {
		if item.AnimeID != two || item.Track != "DUB" {
			t.Fatalf("foreign cursor bypassed scope: %+v", foreign)
		}
	}

	ownFollow, found, err := a.FollowForTitle(ctx, two)
	if err != nil || !found || ownFollow.Track != "SUB" {
		t.Fatalf("owned lookup: %+v %t %v", ownFollow, found, err)
	}
	if _, found, err := b.FollowForTitle(ctx, one); err != nil || found {
		t.Fatalf("foreign lookup: found=%t err=%v", found, err)
	}

	handler := httpapi.NewPostgresRouter(slog.New(slog.NewTextHandler(io.Discard, nil)), db)
	for _, tc := range []struct {
		token, id string
		status    int
	}{
		{first.Token, one, 200},
		{second.Token, one, 404},
		{first.Token, "invalid", 400},
	} {
		req := httptest.NewRequest("GET", "/api/v1/follows/"+tc.id, nil)
		req.Header.Set("Authorization", "Bearer "+tc.token)
		res := httptest.NewRecorder()
		handler.ServeHTTP(res, req)
		if res.Code != tc.status {
			t.Fatalf("follow lookup for %s: status=%d body=%s", tc.id, res.Code, res.Body)
		}
	}

	for _, tc := range []struct {
		query  string
		status int
	}{
		{"?limit=1", 200},
		{"?limit=101", 400},
		{"?limit=1&limit=2", 400},
		{"?cursor=invalid", 400},
	} {
		req := httptest.NewRequest("GET", "/api/v1/follows"+tc.query, nil)
		req.Header.Set("Authorization", "Bearer "+first.Token)
		res := httptest.NewRecorder()
		handler.ServeHTTP(res, req)
		if res.Code != tc.status || (tc.status == 200 && !strings.Contains(res.Body.String(), `"nextCursor"`)) {
			t.Fatalf("follow query %s: status=%d body=%s", tc.query, res.Code, res.Body.String())
		}
	}
}
