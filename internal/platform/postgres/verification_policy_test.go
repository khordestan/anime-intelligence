package postgres_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/khordestan/anime-intelligence/internal/httpapi"
	"github.com/khordestan/anime-intelligence/internal/platform/store"
)

func TestVerifiedEmailPolicyProtectsProductAndPreservesAccountAccess(t *testing.T) {
	db, owner, _, _ := testDB(t)
	ctx := context.Background()
	const password = "synthetic-policy-password"
	one, err := db.Register(ctx, "policy-one@example.test", password)
	if err != nil {
		t.Fatal(err)
	}
	two, err := db.Register(ctx, "policy-two@example.test", password)
	if err != nil {
		t.Fatal(err)
	}
	const anime = "10000000-0000-4000-8000-000000000001"
	// Existing unverified accounts may already have private records. They cannot
	// read or mutate them until address ownership has actually been proven.
	var entry string
	if err = owner.QueryRow(ctx, `INSERT INTO user_anime(user_id,anime_id,watch_status) VALUES($1,$2,'WATCHING') RETURNING id::text`, one.User.ID, anime).Scan(&entry); err != nil {
		t.Fatal(err)
	}
	repo := db.Repository(one.Token)
	if _, err = repo.Library(ctx); !errors.Is(err, store.ErrEmailUnverified) {
		t.Fatal("unverified library read", err)
	}
	if _, err = repo.Add(ctx, anime, "COMPLETED", 12); !errors.Is(err, store.ErrEmailUnverified) {
		t.Fatal("unverified tracker creation", err)
	}
	n := 9
	if _, _, err = repo.Update(ctx, entry, nil, &n); !errors.Is(err, store.ErrEmailUnverified) {
		t.Fatal("unverified tracker update", err)
	}
	if _, _, err = repo.Rate(ctx, anime, 5); !errors.Is(err, store.ErrEmailUnverified) {
		t.Fatal("unverified rating", err)
	}
	handler := httpapi.NewPostgresRouter(slog.New(slog.NewTextHandler(io.Discard, nil)), db)
	call := func(method, path, token, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		if token != "" {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	for _, spec := range [][2]string{{"GET", "/api/v1/tracker"}, {"POST", "/api/v1/tracker"}, {"PATCH", "/api/v1/tracker/" + entry}, {"PUT", "/api/v1/ratings/" + anime}} {
		if w := call(spec[0], spec[1], one.Token, "{}"); w.Code != 403 || !strings.Contains(w.Body.String(), "email_verification_required") || w.Header().Get("Cache-Control") != "no-store" {
			t.Fatal("missing verification gate", spec, w.Code, w.Body.String())
		}
		if w := call(spec[0], spec[1], "", "{}"); w.Code != 401 {
			t.Fatal("missing authentication", spec, w.Code)
		}
	}
	for _, path := range []string{"/api/v1/auth/me", "/api/v1/auth/sessions", "/api/v1/auth/export", "/api/v1/anime"} {
		if w := call("GET", path, one.Token, ""); w.Code != 200 {
			t.Fatal("verification blocked account/catalog access", path, w.Code)
		}
	}
	// Password recovery proves possession for a reset, but must not bypass the
	// separate verification policy or silently verify the email address.
	proof, err := db.IssuePasswordReset(ctx, one.User.Email)
	if err != nil || proof == nil {
		t.Fatal("issue reset", err)
	}
	if err = db.ResetPassword(ctx, proof.Token, "synthetic-recovered-policy-password"); err != nil {
		t.Fatal(err)
	}
	recovered, err := db.Login(ctx, one.User.Email, "synthetic-recovered-policy-password")
	if err != nil || recovered.User.EmailVerified {
		t.Fatal("reset bypassed verification", err)
	}
	refreshed, err := db.Refresh(ctx, recovered.Token)
	if err != nil || refreshed.User.EmailVerified {
		t.Fatal("refresh bypassed verification", err)
	}
	if w := call("GET", "/api/v1/tracker", refreshed.Token, ""); w.Code != 403 {
		t.Fatal("replacement session bypassed policy", w.Code)
	}
	verifyAccount(t, db, refreshed.Token)
	repo = db.Repository(refreshed.Token)
	if rows, err := repo.Library(ctx); err != nil || len(rows) != 1 || rows[0].WatchStatus != "WATCHING" || rows[0].EpisodesWatched != 0 || rows[0].Rating != 0 {
		t.Fatal("denied operation changed stored data", err)
	}
	if w := call("GET", "/api/v1/tracker", refreshed.Token, ""); w.Code != 200 {
		t.Fatal("verification did not unlock existing bearer", w.Code)
	}
	if _, err = repo.Add(ctx, anime, "WATCHING", 3); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := repo.Rate(ctx, anime, 4); err != nil || !ok {
		t.Fatal("verified rating blocked", err)
	}
	if w := call("GET", "/api/v1/tracker", two.Token, ""); w.Code != 403 {
		t.Fatal("verified a different account", w.Code)
	}
	verifyAccount(t, db, two.Token)
	if _, ok, err := db.Repository(two.Token).Update(ctx, entry, nil, &n); err != nil || ok {
		t.Fatal("verification bypassed ownership", err)
	}
	if _, err = owner.Exec(ctx, `UPDATE users SET email_verified_at=NULL WHERE id=$1`, one.User.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = repo.Library(ctx); !errors.Is(err, store.ErrEmailUnverified) {
		t.Fatal("repository trusted stale verification", err)
	}
	if w := call("GET", "/api/v1/tracker", refreshed.Token, ""); w.Code != 403 {
		t.Fatal("HTTP trusted stale verification", w.Code)
	}
	if err = db.DeleteAccount(ctx, refreshed.Token, "synthetic-recovered-policy-password"); err != nil {
		t.Fatal("unverified account cannot delete itself", err)
	}
}
