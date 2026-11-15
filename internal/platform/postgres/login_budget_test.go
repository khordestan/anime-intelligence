package postgres_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/khordestan/anime-intelligence/internal/httpapi"
	"github.com/khordestan/anime-intelligence/internal/platform/postgres"
)

func TestLoginFailureBudgetIsPersistedSharedAndRecoverable(t *testing.T) {
	db, owner, _, runtimeDSN := testDB(t)
	ctx := context.Background()
	const password = "synthetic-budget-password"
	session, err := db.Register(ctx, "budget-one@example.test", password)
	if err != nil {
		t.Fatal(err)
	}
	other, err := db.Register(ctx, "budget-two@example.test", password)
	if err != nil {
		t.Fatal(err)
	}
	second, err := postgres.Open(ctx, runtimeDSN)
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	fail := func(instance *postgres.DB) error {
		_, err := instance.Login(ctx, session.User.Email, "incorrect-synthetic-password")
		return err
	}
	state := func() (int, *time.Time, *time.Time) {
		t.Helper()
		var failures int
		var until, started *time.Time
		if err := owner.QueryRow(ctx, `SELECT failed_login_count,locked_until,login_failure_started_at FROM users WHERE id=$1`, session.User.ID).Scan(&failures, &until, &started); err != nil {
			t.Fatal(err)
		}
		return failures, until, started
	}
	for _, instance := range []*postgres.DB{db, second} {
		if err := fail(instance); !errors.Is(err, postgres.ErrUnauthorized) {
			t.Fatal(err)
		}
	}
	if count, until, started := state(); count != 2 || until != nil || started == nil {
		t.Fatal("failures not persisted across instances", count, until, started)
	}
	if _, err = second.Login(ctx, session.User.Email, password); err != nil {
		t.Fatal("correct login before threshold", err)
	}
	if count, until, started := state(); count != 0 || until != nil || started != nil {
		t.Fatal("successful login did not clear budget")
	}
	// Two workers on independent connection pools must not lose increments.
	var wg sync.WaitGroup
	results := make(chan error, 10)
	for _, instance := range []*postgres.DB{db, second} {
		wg.Add(1)
		go func(instance *postgres.DB) {
			defer wg.Done()
			for range 5 {
				results <- fail(instance)
			}
		}(instance)
	}
	wg.Wait()
	close(results)
	for err := range results {
		if !errors.Is(err, postgres.ErrUnauthorized) {
			t.Fatal("concurrent denial", err)
		}
	}
	count, until, started := state()
	if count != 10 || until == nil || started == nil || time.Until(*until) < 14*time.Minute || time.Until(*until) > 16*time.Minute {
		t.Fatal("threshold/lock duration", count, until)
	}
	lockedUntil := *until
	for _, value := range []string{password, "another-incorrect-password"} {
		if _, err = db.Login(ctx, session.User.Email, value); !errors.Is(err, postgres.ErrUnauthorized) {
			t.Fatal("locked login accepted", err)
		}
	}
	if count, until, _ := state(); count != 10 || until == nil || !until.Equal(lockedUntil) {
		t.Fatal("request extended an active lock")
	}
	if _, err = db.Authenticate(ctx, session.Token); !errors.Is(err, postgres.ErrUnauthorized) {
		t.Fatal("locked account session accepted", err)
	}
	if _, err = second.Login(ctx, other.User.Email, password); err != nil {
		t.Fatal("lock affected another account", err)
	}
	if _, err = owner.Exec(ctx, `UPDATE users SET locked_until=now()-interval '1 second',login_failure_started_at=now()-interval '16 minutes' WHERE id=$1`, session.User.ID); err != nil {
		t.Fatal(err)
	}
	if err = fail(second); !errors.Is(err, postgres.ErrUnauthorized) {
		t.Fatal(err)
	}
	if count, until, _ := state(); count != 1 || until != nil {
		t.Fatal("expired lock/window did not start fresh", count, until)
	}
	// A stale observation window must also reset without having reached a lock.
	if _, err = owner.Exec(ctx, `UPDATE users SET failed_login_count=9,login_failure_started_at=now()-interval '16 minutes' WHERE id=$1`, session.User.ID); err != nil {
		t.Fatal(err)
	}
	if err = fail(db); !errors.Is(err, postgres.ErrUnauthorized) {
		t.Fatal(err)
	}
	if count, until, _ := state(); count != 1 || until != nil {
		t.Fatal("stale failures accumulated indefinitely")
	}
	// Recovery remains available during a lock and resets all failure state.
	if _, err = owner.Exec(ctx, `UPDATE users SET failed_login_count=10,locked_until=now()+interval '15 minutes',login_failure_started_at=now() WHERE id=$1`, session.User.ID); err != nil {
		t.Fatal(err)
	}
	proof, err := db.IssuePasswordReset(ctx, session.User.Email)
	if err != nil || proof == nil {
		t.Fatal("locked account cannot request recovery", err)
	}
	if err = db.ResetPassword(ctx, proof.Token, "synthetic-recovered-budget-password"); err != nil {
		t.Fatal(err)
	}
	if count, until, started := state(); count != 0 || until != nil || started != nil {
		t.Fatal("recovery did not clear budget")
	}
	if _, err = second.Login(ctx, session.User.Email, "synthetic-recovered-budget-password"); err != nil {
		t.Fatal("recovered account still locked", err)
	}
	if _, err = db.Authenticate(ctx, session.Token); !errors.Is(err, postgres.ErrUnauthorized) {
		t.Fatal("recovery restored an old session", err)
	}
}

func TestLoginBudgetErrorsRemainGenericAndSchemaIsRequired(t *testing.T) {
	db, owner, _, runtimeDSN := testDB(t)
	ctx := context.Background()
	session, err := db.Register(ctx, "budget-http@example.test", "synthetic-budget-password")
	if err != nil {
		t.Fatal(err)
	}
	handler := httpapi.NewPostgresRouter(slog.New(slog.NewTextHandler(io.Discard, nil)), db)
	call := func(body string) string {
		t.Helper()
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, httptest.NewRequest("POST", "/api/v1/auth/login", strings.NewReader(body)))
		if w.Code != 401 || w.Header().Get("Cache-Control") != "no-store" {
			t.Fatal("unexpected login denial", w.Code)
		}
		return w.Body.String()
	}
	known := call(`{"email":"budget-http@example.test","password":"incorrect-synthetic-password"}`)
	unknown := call(`{"email":"absent-budget@example.test","password":"incorrect-synthetic-password"}`)
	if _, err = owner.Exec(ctx, `UPDATE users SET locked_until=now()+interval '15 minutes' WHERE id=$1`, session.User.ID); err != nil {
		t.Fatal(err)
	}
	locked := call(`{"email":"budget-http@example.test","password":"synthetic-budget-password"}`)
	if known != unknown || known != locked || strings.Contains(known, "locked") || strings.Contains(known, "count") {
		t.Fatal("login disclosed account/lock state")
	}
	if _, err = owner.Exec(ctx, `ALTER TABLE users DROP COLUMN login_failure_started_at`); err != nil {
		t.Fatal(err)
	}
	if unsafe, err := postgres.Open(ctx, runtimeDSN); err == nil {
		unsafe.Close()
		t.Fatal("accepted missing login security migration")
	}
}
