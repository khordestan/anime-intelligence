package postgres_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/khordestan/anime-intelligence/internal/httpapi"
	"github.com/khordestan/anime-intelligence/internal/platform/postgres"
	"github.com/khordestan/anime-intelligence/internal/platform/store"
	"github.com/khordestan/anime-intelligence/migrations"
)

// The supplied administrator connection only provisions a fresh, randomly named
// database and role. All runtime assertions use that role with forced RLS.
func testDB(t *testing.T) (*postgres.DB, *pgx.Conn, string, string) {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is unset; PostgreSQL integration test runs in CI")
	}
	ctx := context.Background()
	admin, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatal("connect integration administrator:", err)
	}
	suffix := make([]byte, 8)
	if _, err = rand.Read(suffix); err != nil {
		t.Fatal(err)
	}
	name := "anime_test_" + hex.EncodeToString(suffix)
	role := name + "_app"
	secret := make([]byte, 32)
	if _, err = rand.Read(secret); err != nil {
		t.Fatal(err)
	}
	credential := hex.EncodeToString(secret)
	if _, err = admin.Exec(ctx, `CREATE ROLE `+pgx.Identifier{role}.Sanitize()+` LOGIN NOSUPERUSER NOBYPASSRLS PASSWORD '`+credential+`'`); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = admin.Exec(ctx, `DROP DATABASE IF EXISTS `+pgx.Identifier{name}.Sanitize()+` WITH (FORCE)`)
		_, _ = admin.Exec(ctx, `DROP ROLE IF EXISTS `+pgx.Identifier{role}.Sanitize())
		_ = admin.Close(ctx)
	})
	if _, err = admin.Exec(ctx, `CREATE DATABASE `+pgx.Identifier{name}.Sanitize()+` TEMPLATE template0 ENCODING 'UTF8'`); err != nil {
		t.Fatal(err)
	}
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	u.Path = "/" + name
	owner, err := pgx.Connect(ctx, u.String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = owner.Close(ctx) })
	if err = migrations.Apply(ctx, owner); err != nil {
		t.Fatal(err)
	}
	if err = migrations.Apply(ctx, owner); err != nil {
		t.Fatal("migration reapply:", err)
	}
	if _, err = owner.Exec(ctx, `GRANT USAGE ON SCHEMA public TO `+pgx.Identifier{role}.Sanitize()+`; GRANT SELECT ON anime TO `+pgx.Identifier{role}.Sanitize()+`; GRANT SELECT,INSERT,UPDATE,DELETE ON users,auth_sessions,user_anime,watch_progress,user_ratings TO `+pgx.Identifier{role}.Sanitize()); err != nil {
		t.Fatal(err)
	}
	ownerDSN := u.String()
	u.User = url.UserPassword(role, credential)
	db, err := postgres.Open(ctx, u.String())
	if err != nil {
		t.Fatal("open restricted runtime:", err)
	}
	t.Cleanup(db.Close)
	return db, owner, ownerDSN, u.String()
}

func TestPostgresRuntime(t *testing.T) {
	db, owner, ownerDSN, runtimeDSN := testDB(t)
	ctx := context.Background()
	if unsafe, err := postgres.Open(ctx, ownerDSN); err == nil {
		unsafe.Close()
		t.Fatal("accepted schema owner/superuser")
	}
	one, err := db.Register(ctx, " Runtime-One@example.test ", "synthetic-password-one")
	if err != nil {
		t.Fatal(err)
	}
	two, err := db.Register(ctx, "runtime-two@example.test", "synthetic-password-two")
	if err != nil {
		t.Fatal(err)
	}
	if one.User.Email != "runtime-one@example.test" {
		t.Fatal("email normalization failed")
	}
	if _, err = db.Register(ctx, "RUNTIME-ONE@example.test", "synthetic-password-one"); !errors.Is(err, postgres.ErrConflict) {
		t.Fatal("duplicate registration:", err)
	}
	for _, email := range []string{"runtime-one@example.test", "missing@example.test"} {
		if _, err = db.Login(ctx, email, "wrong-password"); !errors.Is(err, postgres.ErrUnauthorized) {
			t.Fatal("login must fail generically:", err)
		}
	}
	login, err := db.Login(ctx, "RUNTIME-ONE@example.test", "synthetic-password-one")
	if err != nil {
		t.Fatal("login:", err)
	}
	if login.Token == one.Token {
		t.Fatal("login reused session")
	}
	a := db.Repository(one.Token)
	b := db.Repository(two.Token)
	const anime = "10000000-0000-4000-8000-000000000001"
	catalog, err := a.Catalog(ctx, "solo", 24, 0)
	if err != nil || len(catalog) != 1 {
		t.Fatalf("catalog: %d %v", len(catalog), err)
	}
	entry, err := a.Add(ctx, anime, "WATCHING", 2)
	if err != nil {
		t.Fatal("add:", err)
	}
	if got, err := b.Library(ctx); err != nil || len(got) != 0 {
		t.Fatal("cross-user library leaked", err)
	}
	episodes := 8
	if _, ok, err := b.Update(ctx, entry.ID, nil, &episodes); err != nil || ok {
		t.Fatal("cross-user update accepted", err)
	}
	if _, ok, err := b.Rate(ctx, anime, 1); err != nil || ok {
		t.Fatal("rating without own tracker accepted", err)
	}
	for _, score := range []int{4, 5} {
		if e, ok, err := a.Rate(ctx, anime, score); err != nil || !ok || e.Rating != score {
			t.Fatal("rate:", err)
		}
	}
	var count int
	if err = owner.QueryRow(ctx, `SELECT count(*) FROM user_ratings WHERE user_id=$1 AND anime_id=$2`, one.User.ID, anime).Scan(&count); err != nil || count != 1 {
		t.Fatal("duplicate rating:", count, err)
	}
	status := "COMPLETED"
	if _, ok, err := a.Update(ctx, entry.ID, &status, &episodes); err != nil || !ok {
		t.Fatal("update:", err)
	}
	got, err := a.Library(ctx)
	if err != nil || len(got) != 1 || got[0].EpisodesWatched != 8 || got[0].Rating != 5 {
		t.Fatal("library persistence:", got, err)
	}
	if _, err = b.Add(ctx, anime, "PLANNING", 0); err != nil {
		t.Fatal(err)
	}
	if _, err = a.Add(ctx, anime, "INVALID", 0); err == nil {
		t.Fatal("accepted invalid status")
	}
	if _, err = db.Authenticate(ctx, two.Token); err != nil {
		t.Fatal("failed transaction contaminated pooled context:", err)
	}
	var hash []byte
	if err = owner.QueryRow(ctx, `SELECT token_hash FROM auth_sessions WHERE user_id=$1 LIMIT 1`, one.User.ID).Scan(&hash); err != nil || len(hash) != 32 || bytes.Equal(hash, []byte(one.Token)) {
		t.Fatal("session storage:", err)
	}
	rotated, err := db.Refresh(ctx, one.Token)
	if err != nil {
		t.Fatal("refresh:", err)
	}
	if _, err = db.Authenticate(ctx, one.Token); !errors.Is(err, postgres.ErrUnauthorized) {
		t.Fatal("rotated token accepted", err)
	}
	if _, err = db.Authenticate(ctx, rotated.Token); err != nil {
		t.Fatal(err)
	}
	if err = db.Logout(ctx, rotated.Token); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Authenticate(ctx, rotated.Token); !errors.Is(err, postgres.ErrUnauthorized) {
		t.Fatal("revoked token accepted", err)
	}
	if _, err = owner.Exec(ctx, `UPDATE auth_sessions SET expires_at=now()-interval '1 minute',created_at=now()-interval '1 hour' WHERE user_id=$1`, two.User.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Authenticate(ctx, two.Token); !errors.Is(err, postgres.ErrUnauthorized) {
		t.Fatal("expired session accepted", err)
	}
	if _, err = owner.Exec(ctx, `UPDATE users SET disabled_at=now() WHERE id=$1`, one.User.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Authenticate(ctx, login.Token); !errors.Is(err, postgres.ErrUnauthorized) {
		t.Fatal("disabled account session accepted", err)
	}
	if _, err = db.Login(ctx, one.User.Email, "synthetic-password-one"); !errors.Is(err, postgres.ErrUnauthorized) {
		t.Fatal("disabled login accepted", err)
	}
	raw, err := pgx.Connect(ctx, runtimeDSN)
	if err != nil {
		t.Fatal(err)
	}
	defer raw.Close(ctx)
	for _, table := range []string{"users", "auth_sessions", "user_anime", "watch_progress", "user_ratings"} {
		if err = raw.QueryRow(ctx, `SELECT count(*) FROM `+pgx.Identifier{table}.Sanitize()).Scan(&count); err != nil || count != 0 {
			t.Fatalf("unscoped RLS exposed %s: %d %v", table, count, err)
		}
	}
	tx, err := raw.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(ctx, `SELECT set_config('app.user_id',$1,true)`, two.User.ID); err != nil {
		t.Fatal(err)
	}
	if err = tx.QueryRow(ctx, `SELECT count(*) FROM user_anime WHERE user_id=$1`, one.User.ID).Scan(&count); err != nil || count != 0 {
		t.Fatal("RLS exposed another user's entry", err)
	}
	if _, err = tx.Exec(ctx, `INSERT INTO user_anime(user_id,anime_id,watch_status) VALUES($1,$2,'PLANNING')`, one.User.ID, anime); err == nil {
		t.Fatal("RLS accepted cross-user insert")
	}
	if err = tx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	if err = raw.QueryRow(ctx, `SELECT count(*) FROM users`).Scan(&count); err != nil || count != 0 {
		t.Fatal("transaction context leaked", err)
	}
	// Inspect a fresh unscoped connection with the runtime role: no private rows.
	// Rollback and commit paths both reset app settings before pool reuse.
	for _, token := range []string{"", strings.Repeat("a", 43), "not-a-token"} {
		if _, err = db.Authenticate(ctx, token); !errors.Is(err, postgres.ErrUnauthorized) {
			t.Fatal("invalid token:", err)
		}
	}
}

func TestConcurrentSessionRotation(t *testing.T) {
	db, _, _, _ := testDB(t)
	ctx := context.Background()
	s, err := db.Register(ctx, "rotate@example.test", "synthetic-rotate-password")
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for range 2 {
		wg.Add(1)
		go func() { defer wg.Done(); _, err := db.Refresh(ctx, s.Token); results <- err }()
	}
	wg.Wait()
	close(results)
	success := 0
	for err := range results {
		if err == nil {
			success++
		} else if !errors.Is(err, postgres.ErrUnauthorized) {
			t.Fatal(err)
		}
	}
	if success != 1 {
		t.Fatalf("expected exactly one rotation, got %d", success)
	}
}

func TestHTTPPostgresIsolation(t *testing.T) {
	db, _, _, _ := testDB(t)
	handler := httpapi.NewPostgresRouter(slog.New(slog.NewTextHandler(io.Discard, nil)), db)
	call := func(method, path, token, body string) *httptest.ResponseRecorder {
		t.Helper()
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		if token != "" {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	for _, request := range [][2]string{{"GET", "/api/v1/tracker"}, {"POST", "/api/v1/tracker"}, {"PATCH", "/api/v1/tracker/10000000-0000-4000-8000-000000000001"}, {"PUT", "/api/v1/ratings/10000000-0000-4000-8000-000000000001"}, {"GET", "/api/v1/auth/me"}} {
		if w := call(request[0], request[1], "", "{}"); w.Code != 401 {
			t.Fatal("unprotected endpoint:", request, w.Code)
		}
	}
	w := call("POST", "/api/v1/auth/register", "", `{"email":"http-one@example.test","password":"synthetic-http-password"}`)
	if w.Code != 201 {
		t.Fatal("register:", w.Code, w.Body.String())
	}
	var s postgres.Session
	if err := json.Unmarshal(w.Body.Bytes(), &s); err != nil {
		t.Fatal(err)
	}
	if w.Header().Get("Cache-Control") != "no-store" || !s.ExpiresAt.After(time.Now()) {
		t.Fatal("session response contract")
	}
	if w = call("GET", "/api/v1/auth/me", s.Token, ""); w.Code != 200 {
		t.Fatal("me:", w.Code)
	}
	w = call("POST", "/api/v1/tracker", s.Token, `{"animeId":"10000000-0000-4000-8000-000000000001","watchStatus":"WATCHING","episodesWatched":3}`)
	if w.Code != 201 {
		t.Fatal("tracker create:", w.Code, w.Body.String())
	}
	var entry store.Entry
	_ = json.Unmarshal(w.Body.Bytes(), &entry)
	other, err := db.Register(context.Background(), "http-two@example.test", "synthetic-other-password")
	if err != nil {
		t.Fatal(err)
	}
	if w = call("PATCH", "/api/v1/tracker/"+entry.ID, other.Token, `{"episodesWatched":99}`); w.Code != 404 {
		t.Fatal("cross-user HTTP update:", w.Code, w.Body.String())
	}
	if w = call("GET", "/api/v1/tracker", other.Token, ""); w.Code != 200 || !strings.Contains(w.Body.String(), `"count":0`) {
		t.Fatal("HTTP isolation:", w.Body.String())
	}
	if w = call("POST", "/api/v1/auth/login", "", `{"email":"http-one@example.test","password":"synthetic-http-password","user_id":"spoofed"}`); w.Code != 400 {
		t.Fatal("unknown auth fields accepted")
	}
	if w = call("POST", "/api/v1/auth/logout", s.Token, ""); w.Code != 204 {
		t.Fatal("logout:", w.Code)
	}
	if w = call("GET", "/api/v1/tracker", s.Token, ""); w.Code != 401 {
		t.Fatal("logged-out tracker accessible")
	}
}

func TestMigrationLedgerIntegrity(t *testing.T) {
	_, owner, _, _ := testDB(t)
	ctx := context.Background()
	if _, err := owner.Exec(ctx, `UPDATE schema_migrations SET sha256='tampered' WHERE name='000001_initial_schema.up.sql'`); err != nil {
		t.Fatal(err)
	}
	if err := migrations.Apply(ctx, owner); err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatal("migration tampering accepted", err)
	}
	if _, err := owner.Exec(ctx, `INSERT INTO schema_migrations(name,sha256) VALUES('unknown.up.sql','unknown')`); err != nil {
		t.Fatal(err)
	}
	if err := migrations.Apply(ctx, owner); err == nil || !strings.Contains(err.Error(), "unknown applied migration") {
		t.Fatal("unknown migration accepted", err)
	}
}

func TestMissingForcedRLSRejectsStartup(t *testing.T) {
	_, owner, _, runtimeDSN := testDB(t)
	ctx := context.Background()
	if _, err := owner.Exec(ctx, `ALTER TABLE user_anime NO FORCE ROW LEVEL SECURITY`); err != nil {
		t.Fatal(err)
	}
	if unsafe, err := postgres.Open(ctx, runtimeDSN); err == nil {
		unsafe.Close()
		t.Fatal("runtime accepted missing forced RLS")
	}
}

func TestMigrationFailureRollsBackSchemaAndLedger(t *testing.T) {
	_, owner, _, _ := testDB(t)
	ctx := context.Background()
	// Intentionally conflict with a migration in this disposable test database.
	if _, err := owner.Exec(ctx, `DROP SCHEMA public CASCADE; CREATE SCHEMA public; CREATE TABLE public.anime (synthetic integer)`); err != nil {
		t.Fatal(err)
	}
	if err := migrations.Apply(ctx, owner); err == nil {
		t.Fatal("expected conflicting schema to reject migration")
	}
	var users, ledger, existing *string
	if err := owner.QueryRow(ctx, `SELECT to_regclass('public.users')::text,to_regclass('public.schema_migrations')::text,to_regclass('public.anime')::text`).Scan(&users, &ledger, &existing); err != nil {
		t.Fatal(err)
	}
	if users != nil || ledger != nil || existing == nil {
		t.Fatal("failed migration was not atomic")
	}
}

func TestLockedAndUnprovisionedAccountsRejectAuthentication(t *testing.T) {
	db, owner, _, _ := testDB(t)
	ctx := context.Background()
	s, err := db.Register(ctx, "locked@example.test", "synthetic-locked-password")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = owner.Exec(ctx, `UPDATE users SET locked_until=now()+interval '1 hour' WHERE id=$1`, s.User.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Login(ctx, s.User.Email, "synthetic-locked-password"); !errors.Is(err, postgres.ErrUnauthorized) {
		t.Fatal("locked login accepted", err)
	}
	if _, err = db.Authenticate(ctx, s.Token); !errors.Is(err, postgres.ErrUnauthorized) {
		t.Fatal("locked session accepted", err)
	}
	if _, err = owner.Exec(ctx, `UPDATE users SET locked_until=NULL WHERE id=$1`, s.User.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Authenticate(ctx, s.Token); err != nil {
		t.Fatal("unlocked session failed", err)
	}
	if _, err = owner.Exec(ctx, `INSERT INTO users(email) VALUES('unprovisioned@example.test')`); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Login(ctx, "unprovisioned@example.test", "synthetic-locked-password"); !errors.Is(err, postgres.ErrUnauthorized) {
		t.Fatal("account without credentials accepted", err)
	}
}
