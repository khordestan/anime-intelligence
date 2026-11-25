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
	if _, err = owner.Exec(ctx, `GRANT USAGE ON SCHEMA public TO `+pgx.Identifier{role}.Sanitize()+`; GRANT SELECT ON anime,catalog_metadata,anime_relations,airing_schedule TO `+pgx.Identifier{role}.Sanitize()+`; GRANT SELECT,INSERT,UPDATE,DELETE ON users,auth_sessions,auth_challenges,identity_outbox,identity_requests,user_anime,watch_progress,user_ratings TO `+pgx.Identifier{role}.Sanitize()); err != nil {
		t.Fatal(err)
	}
	if _, err = owner.Exec(ctx, `GRANT SELECT,INSERT,UPDATE ON catalog_analysis_owner_budget TO `+pgx.Identifier{role}.Sanitize()+`; GRANT SELECT,UPDATE ON catalog_analysis_global_budget TO `+pgx.Identifier{role}.Sanitize()+`; GRANT SELECT,INSERT,UPDATE,DELETE ON release_email_jobs TO `+pgx.Identifier{role}.Sanitize()); err != nil {
		t.Fatal(err)
	}
	if _, err = owner.Exec(ctx, `GRANT SELECT ON user_preferences,recommendations,recommendation_feedback,user_follows,notifications TO `+pgx.Identifier{role}.Sanitize()); err != nil {
		t.Fatal(err)
	}
	if _, err = owner.Exec(ctx, `GRANT SELECT (id,provider_key) ON data_sources TO `+pgx.Identifier{role}.Sanitize()); err != nil { t.Fatal(err) }
	if _, err = owner.Exec(ctx, `GRANT INSERT,UPDATE,DELETE ON user_follows TO `+pgx.Identifier{role}.Sanitize()+`; GRANT INSERT,UPDATE ON notifications TO `+pgx.Identifier{role}.Sanitize()+`; GRANT SELECT ON domain_events TO `+pgx.Identifier{role}.Sanitize()); err != nil { t.Fatal(err) }
	if _, err = owner.Exec(ctx, `GRANT SELECT ON anime_features,anime_feature_observations TO `+pgx.Identifier{role}.Sanitize()+`; GRANT INSERT,UPDATE,DELETE ON user_preferences TO `+pgx.Identifier{role}.Sanitize()); err != nil { t.Fatal(err) }
	if _, err = owner.Exec(ctx, `GRANT SELECT,INSERT,DELETE ON personal_match_dismissals TO `+pgx.Identifier{role}.Sanitize()); err != nil {
		t.Fatal(err)
	}
	if _, err = owner.Exec(ctx, `GRANT SELECT,INSERT ON watch_cycles TO `+pgx.Identifier{role}.Sanitize()); err != nil { t.Fatal(err) }
	if _,err=owner.Exec(ctx,`GRANT SELECT(anime_id,provider,external_id) ON anime_external_ids TO `+pgx.Identifier{role}.Sanitize());err!=nil{t.Fatal(err)}
 if _,err=owner.Exec(ctx,`GRANT SELECT(anime_id,field,revision,mode,source_id,observed_at) ON catalog_field_choices TO `+pgx.Identifier{role}.Sanitize());err!=nil{t.Fatal(err)}
	ownerDSN := u.String()
	u.User = url.UserPassword(role, credential)
	db, err := postgres.Open(ctx, u.String())
	if err != nil {
		t.Fatal("open restricted runtime:", err)
	}
	t.Cleanup(db.Close)
	return db, owner, ownerDSN, u.String()
}

// Product tests obtain address ownership through the actual challenge flow.
func verifyAccount(t *testing.T, db *postgres.DB, token string) {
	t.Helper()
	proof, err := db.IssueEmailVerification(context.Background(), token)
	if err != nil || proof == nil {
		t.Fatal("issue fixture verification", err)
	}
	if err := db.VerifyEmail(context.Background(), proof.Token); err != nil {
		t.Fatal("verify fixture account", err)
	}
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
	verifyAccount(t, db, one.Token)
	verifyAccount(t, db, two.Token)
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
	verifyAccount(t, db, s.Token)
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
	verifyAccount(t, db, other.Token)
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

func TestAccountExportAndDeletionIsolation(t *testing.T) {
	db, owner, _, _ := testDB(t)
	ctx := context.Background()
	one, err := db.Register(ctx, "export-one@example.test", "synthetic-export-password")
	if err != nil {
		t.Fatal(err)
	}
	two, err := db.Register(ctx, "export-two@example.test", "synthetic-other-password")
	if err != nil {
		t.Fatal(err)
	}
	another, err := db.Login(ctx, one.User.Email, "synthetic-export-password")
	if err != nil {
		t.Fatal(err)
	}
	verifyAccount(t, db, one.Token)
	verifyAccount(t, db, two.Token)
	const anime = "10000000-0000-4000-8000-000000000001"
	if _, err = db.Repository(one.Token).Add(ctx, anime, "WATCHING", 3); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Repository(two.Token).Add(ctx, anime, "COMPLETED", 12); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := db.Repository(one.Token).Rate(ctx, anime, 5); err != nil || !ok {
		t.Fatal(err)
	}
	// Populate every exported resource for both accounts, so isolation is
	// tested against real foreign rows rather than empty tables.
	for _, seed := range []struct {
		sql  string
		args []any
	}{
		{`INSERT INTO watch_cycles(user_id,anime_id,cycle_number,episodes_watched,total_episodes) VALUES($1,$3,1,12,12),($2,$3,1,12,12)`, []any{one.User.ID,two.User.ID,anime}},
		{`UPDATE user_anime SET watch_cycle=2 WHERE user_id=$1 OR user_id=$2`, []any{one.User.ID,two.User.ID}},
		{`INSERT INTO user_preferences(user_id,feature_id,preference_weight,origin)
    SELECT accounts.uid,f.id,0.5,'DECLARED' FROM (VALUES($1::uuid),($2::uuid)) accounts(uid)
    CROSS JOIN (SELECT id FROM anime_features ORDER BY feature_key LIMIT 1) f`, []any{one.User.ID, two.User.ID}},
		{`INSERT INTO recommendations(user_id,anime_id,category,score,algorithm_version)
    SELECT accounts.uid,$3,'PERSONAL_MATCH',0.75,'synthetic-test-v1' FROM (VALUES($1::uuid),($2::uuid)) accounts(uid)`, []any{one.User.ID, two.User.ID, anime}},
		{`INSERT INTO recommendation_feedback(recommendation_id,user_id,feedback)
    SELECT id,user_id,'INTERESTED' FROM recommendations WHERE user_id=$1 OR user_id=$2`, []any{one.User.ID, two.User.ID}},
		{`INSERT INTO personal_match_dismissals(user_id,anime_id) VALUES($1,$3),($2,$3)`, []any{one.User.ID, two.User.ID, anime}},
		{`INSERT INTO user_follows(user_id,anime_id) VALUES($1,$3),($2,$3)`, []any{one.User.ID, two.User.ID, anime}},
		{`INSERT INTO catalog_analysis_owner_budget(user_id,window_start,requests) VALUES($1,now(),1),($2,now(),1)`, []any{one.User.ID,two.User.ID}},
		{`INSERT INTO notifications(user_id,title,body) VALUES($1,'Synthetic notice','Synthetic content'),($2,'Synthetic notice','Synthetic content')`, []any{one.User.ID, two.User.ID}},
	} {
		if _, err = owner.Exec(ctx, seed.sql, seed.args...); err != nil {
			t.Fatal("seed private export resources:", err)
		}
	}
	// A separate account also rates the same shared catalog item.
	if _, ok, err := db.Repository(two.Token).Rate(ctx, anime, 1); err != nil || !ok {
		t.Fatal(err)
	}
	exported, err := db.ExportAccount(ctx, one.Token)
	if err != nil {
		t.Fatal(err)
	}
	if exported.User.ID != one.User.ID || exported.SchemaVersion != 1 || len(exported.Data) != 11 {
		t.Fatal("export contract failed")
	}
	encoded, err := json.Marshal(exported)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{two.User.ID, two.User.Email, one.Token, "password_hash", "token_hash", "synthetic-export-password", "argon2id"} {
		if bytes.Contains(encoded, []byte(secret)) {
			t.Fatal("export leaked another account or credential field")
		}
	}
	for _, key := range []string{"tracker", "watchCycles", "ratings", "progress", "preferences", "recommendations", "feedback", "personalMatchDismissals", "follows", "notifications"} {
		var rows []map[string]any
		if err = json.Unmarshal(exported.Data[key], &rows); err != nil || len(rows) != 1 {
			t.Fatalf("export %s: %v", key, err)
		}
		if rows[0]["user_id"] != one.User.ID {
			t.Fatal("cross-account export row")
		}
	}
	if err = db.DeleteAccount(ctx, one.Token, "incorrect-password"); !errors.Is(err, postgres.ErrReauthentication) {
		t.Fatal("deletion without password accepted", err)
	}
	if _, err = db.Authenticate(ctx, one.Token); err != nil {
		t.Fatal("failed deletion modified account", err)
	}
	if err = db.DeleteAccount(ctx, one.Token, "synthetic-export-password"); err != nil {
		t.Fatal("delete:", err)
	}
	for _, token := range []string{one.Token, another.Token} {
		if _, err = db.Authenticate(ctx, token); !errors.Is(err, postgres.ErrUnauthorized) {
			t.Fatal("deleted account session accepted", err)
		}
	}
	for _, table := range []string{"users", "auth_sessions", "user_anime", "watch_cycles", "user_ratings", "watch_progress", "user_preferences", "recommendations", "recommendation_feedback", "personal_match_dismissals", "user_follows", "notifications", "catalog_analysis_owner_budget"} {
		field := "user_id"
		if table == "users" {
			field = "id"
		}
		var count int
		if err = owner.QueryRow(ctx, `SELECT count(*) FROM `+pgx.Identifier{table}.Sanitize()+` WHERE `+pgx.Identifier{field}.Sanitize()+`=$1`, one.User.ID).Scan(&count); err != nil || count != 0 {
			t.Fatalf("deletion left %s records: %d %v", table, count, err)
		}
		if err = owner.QueryRow(ctx, `SELECT count(*) FROM `+pgx.Identifier{table}.Sanitize()+` WHERE `+pgx.Identifier{field}.Sanitize()+`=$1`, two.User.ID).Scan(&count); err != nil || count != 1 {
			t.Fatalf("deletion damaged another account's %s: %d %v", table, count, err)
		}
	}
	if rows, err := db.Repository(two.Token).Library(ctx); err != nil || len(rows) != 1 || rows[0].EpisodesWatched != 12 {
		t.Fatal("deletion affected another account", err)
	}
}

func TestHTTPAccountExportAndPasswordConfirmedDeletion(t *testing.T) {
	db, _, _, _ := testDB(t)
	ctx := context.Background()
	s, err := db.Register(ctx, "http-export@example.test", "synthetic-export-password")
	if err != nil {
		t.Fatal(err)
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
	for _, spec := range [][2]string{{"GET", "/api/v1/auth/export"}, {"DELETE", "/api/v1/auth/account"}} {
		if w := call(spec[0], spec[1], "", "{}"); w.Code != 401 {
			t.Fatal("unprotected account route", w.Code)
		}
	}
	if w := call("GET", "/api/v1/auth/export", s.Token, ""); w.Code != 200 || w.Header().Get("Cache-Control") != "no-store" || !strings.Contains(w.Header().Get("Content-Disposition"), "attachment") {
		t.Fatal("export response contract:", w.Code)
	}
	if w := call("DELETE", "/api/v1/auth/account", s.Token, `{"password":"synthetic-export-password","userId":"spoofed"}`); w.Code != 400 {
		t.Fatal("deletion accepted an owner override", w.Code)
	}
	if w := call("DELETE", "/api/v1/auth/account", s.Token, `{"password":"wrong-password"}`); w.Code != 401 || !strings.Contains(w.Body.String(), "invalid_current_password") {
		t.Fatal("deletion accepted wrong password or obscured reauthentication failure", w.Code)
	}
	if w := call("DELETE", "/api/v1/auth/account", s.Token, `{"password":"synthetic-export-password"}`); w.Code != 204 {
		t.Fatal("delete failed:", w.Code, w.Body.String())
	}
	if w := call("GET", "/api/v1/auth/me", s.Token, ""); w.Code != 401 {
		t.Fatal("deleted account retained access", w.Code)
	}
}

func TestConcurrentAccountExportUsesConsistentSnapshot(t *testing.T) {
	db, _, _, _ := testDB(t)
	ctx := context.Background()
	s, err := db.Register(ctx, "snapshot@example.test", "synthetic-snapshot-password")
	if err != nil {
		t.Fatal(err)
	}
	writer, err := db.Login(ctx, s.User.Email, "synthetic-snapshot-password")
	if err != nil {
		t.Fatal(err)
	}
	verifyAccount(t, db, s.Token)
	const anime = "10000000-0000-4000-8000-000000000001"
	if _, err = db.Repository(writer.Token).Add(ctx, anime, "WATCHING", 1); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		for i := 0; i < 12; i++ {
			status, n := "WATCHING", 1
			if i%2 == 1 {
				status, n = "COMPLETED", 12
			}
			if _, err := db.Repository(writer.Token).Add(ctx, anime, status, n); err != nil {
				done <- err
				return
			}
		}
		done <- nil
	}()
	for range 8 {
		out, err := db.ExportAccount(ctx, s.Token)
		if err != nil {
			t.Fatal("concurrent export:", err)
		}
		var tracker []struct {
			Status string `json:"watch_status"`
		}
		var progress []struct {
			Episodes int `json:"episodes_watched"`
		}
		if err = json.Unmarshal(out.Data["tracker"], &tracker); err != nil {
			t.Fatal(err)
		}
		if err = json.Unmarshal(out.Data["progress"], &progress); err != nil {
			t.Fatal(err)
		}
		if len(tracker) != 1 || len(progress) != 1 || (tracker[0].Status == "WATCHING" && progress[0].Episodes != 1) || (tracker[0].Status == "COMPLETED" && progress[0].Episodes != 12) {
			t.Fatal("export mixed transaction snapshots")
		}
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestConcurrentAccountDeletionIsSerialized(t *testing.T) {
	db, _, _, _ := testDB(t)
	ctx := context.Background()
	s, err := db.Register(ctx, "delete-race@example.test", "synthetic-delete-password")
	if err != nil {
		t.Fatal(err)
	}
	other, err := db.Login(ctx, s.User.Email, "synthetic-delete-password")
	if err != nil {
		t.Fatal(err)
	}
	results := make(chan error, 2)
	for _, token := range []string{s.Token, other.Token} {
		go func(token string) { results <- db.DeleteAccount(ctx, token, "synthetic-delete-password") }(token)
	}
	success := 0
	for range 2 {
		err := <-results
		if err == nil {
			success++
		} else if !errors.Is(err, postgres.ErrUnauthorized) {
			t.Fatal("concurrent deletion failed unexpectedly:", err)
		}
	}
	if success != 1 {
		t.Fatalf("expected one deletion, got %d", success)
	}
}
