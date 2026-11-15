package postgres_test

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/khordestan/anime-intelligence/internal/httpapi"
	"github.com/khordestan/anime-intelligence/internal/platform/postgres"
	"github.com/khordestan/anime-intelligence/internal/platform/store"
	"io"
	"log/slog"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestRemoveTrackerIsolationAndRollback(t *testing.T) {
	db, owner, _, runtimeDSN := testDB(t)
	ctx := context.Background()
	one, err := db.Register(ctx, "remove-one@example.test", "synthetic-remove-password")
	if err != nil {
		t.Fatal(err)
	}
	two, err := db.Register(ctx, "remove-two@example.test", "synthetic-remove-password")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Repository(two.Token).Remove(ctx, "10000000-0000-4000-8000-000000000001"); !errors.Is(err, store.ErrEmailUnverified) {
		t.Fatal("unverified removal accepted", err)
	}
	verifyAccount(t, db, one.Token)
	verifyAccount(t, db, two.Token)
	const anime = "10000000-0000-4000-8000-000000000001"
	a, err := db.Repository(one.Token).Add(ctx, anime, "WATCHING", 5)
	if err != nil {
		t.Fatal(err)
	}
	b, err := db.Repository(two.Token).Add(ctx, anime, "WATCHING", 8)
	if err != nil {
		t.Fatal(err)
	}
	for _, token := range []string{one.Token, two.Token} {
		if _, ok, err := db.Repository(token).Rate(ctx, anime, 4); err != nil || !ok {
			t.Fatal(err)
		}
	}
	if _, err = db.Repository("invalid").Remove(ctx, a.ID); !errors.Is(err, postgres.ErrUnauthorized) {
		t.Fatal("invalid credential accepted", err)
	}
	handler := httpapi.NewPostgresRouter(slog.New(slog.NewTextHandler(io.Discard, nil)), db)
	remove := func(token, id string, want int) {
		t.Helper()
		req := httptest.NewRequest("DELETE", "/api/v1/tracker/"+id, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		res := httptest.NewRecorder()
		handler.ServeHTTP(res, req)
		if res.Code != want {
			t.Fatalf("remove status %d want %d", res.Code, want)
		}
		if want == 204 && res.Body.Len() != 0 {
			t.Fatal("remove response had body")
		}
	}
	remove(two.Token, a.ID, 404)
	remove(one.Token, "invalid-id", 400)
	u, _ := url.Parse(runtimeDSN)
	role := pgx.Identifier{u.User.Username()}.Sanitize()
	if _, err = owner.Exec(ctx, `REVOKE DELETE ON user_ratings FROM `+role); err != nil {
		t.Fatal(err)
	}
	remove(one.Token, a.ID, 500)
	items, err := db.Repository(one.Token).Library(ctx)
	if err != nil || len(items) != 1 || items[0].EpisodesWatched != 5 || items[0].Rating != 4 {
		t.Fatal("failed delete did not roll back", err)
	}
	if _, err = owner.Exec(ctx, `GRANT DELETE ON user_ratings TO `+role); err != nil {
		t.Fatal(err)
	}
	remove(one.Token, a.ID, 204)
	remove(one.Token, a.ID, 404)
	for _, table := range []string{"user_anime", "watch_progress", "user_ratings"} {
		var n int
		if err = owner.QueryRow(ctx, `SELECT count(*) FROM `+pgx.Identifier{table}.Sanitize()+` WHERE user_id=$1`, one.User.ID).Scan(&n); err != nil || n != 0 {
			t.Fatal("retained deleted data", table, err)
		}
	}
	items, err = db.Repository(two.Token).Library(ctx)
	if err != nil || len(items) != 1 || items[0].ID != b.ID || items[0].Rating != 4 || items[0].EpisodesWatched != 8 {
		t.Fatal("removal changed other owner", err)
	}
	fresh, err := db.Repository(one.Token).Add(ctx, anime, "PLANNING", 0)
	if err != nil || fresh.Rating != 0 || fresh.EpisodesWatched != 0 || fresh.ID == a.ID {
		t.Fatal("re-add retained deleted state", err)
	}
}
