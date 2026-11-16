package postgres_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/khordestan/anime-intelligence/internal/httpapi"
	"github.com/khordestan/anime-intelligence/internal/platform/postgres"
)

func TestReadinessRechecksLiveRoleSchemaAndDatabase(t *testing.T) {
	db, owner, _, dsn := testDB(t)
	ctx := context.Background()
	handler := httpapi.NewPostgresRouter(slog.New(slog.NewTextHandler(io.Discard, nil)), db)
	assertProbe := func(path string, want int) {
		t.Helper()
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest("GET", path, nil))
		if response.Code != want {
			t.Fatal("probe status", path, response.Code, want)
		}
		if path == "/readyz" && response.Header().Get("Cache-Control") != "no-store" {
			t.Fatal("readiness cached")
		}
		for _, detail := range []string{"outcome", "identity_requests", "role", "postgres://"} {
			if strings.Contains(response.Body.String(), detail) {
				t.Fatal("probe disclosed internal error")
			}
		}
	}
	if err := db.Ready(ctx); err != nil {
		t.Fatal(err)
	}
	assertProbe("/readyz", 200)
	if _, err := owner.Exec(ctx, `ALTER TABLE identity_requests NO FORCE ROW LEVEL SECURITY`); err != nil {
		t.Fatal(err)
	}
	if err := db.Ready(ctx); !errors.Is(err, postgres.ErrNotReady) {
		t.Fatal("RLS drift accepted", err)
	}
	assertProbe("/readyz", 503)
	assertProbe("/healthz", 200)
	if _, err := owner.Exec(ctx, `ALTER TABLE identity_requests FORCE ROW LEVEL SECURITY`); err != nil {
		t.Fatal(err)
	}
	assertProbe("/readyz", 200)
	if _, err := owner.Exec(ctx, `ALTER TABLE watch_cycles NO FORCE ROW LEVEL SECURITY`); err != nil {t.Fatal(err)}
	assertProbe("/readyz",503)
	if _, err := owner.Exec(ctx, `ALTER TABLE watch_cycles FORCE ROW LEVEL SECURITY`); err != nil {t.Fatal(err)}
	assertProbe("/readyz",200)
	u, _ := url.Parse(dsn)
	role := pgx.Identifier{u.User.Username()}.Sanitize()
	if _, err := owner.Exec(ctx, `ALTER ROLE `+role+` BYPASSRLS`); err != nil {
		t.Fatal(err)
	}
	assertProbe("/readyz", 503)
	assertProbe("/healthz", 200)
	if _, err := owner.Exec(ctx, `ALTER ROLE `+role+` NOBYPASSRLS`); err != nil {
		t.Fatal(err)
	}
	assertProbe("/readyz", 200)
	if _, err := owner.Exec(ctx, `ALTER TABLE identity_outbox DROP COLUMN outcome`); err != nil {
		t.Fatal(err)
	}
	assertProbe("/readyz", 503)
	assertProbe("/healthz", 200)
	db.Close()
	assertProbe("/readyz", 503)
	assertProbe("/healthz", 200)
}
func TestReadinessHonorsDeadlineWhileSchemaIsLocked(t *testing.T) {
	db, owner, _, _ := testDB(t)
	tx, err := owner.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(context.Background())
	if _, err = tx.Exec(context.Background(), `LOCK TABLE identity_outbox IN ACCESS EXCLUSIVE MODE`); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	if err = db.Ready(ctx); !errors.Is(err, postgres.ErrNotReady) {
		t.Fatal("locked database reported ready", err)
	}
	if ctx.Err() == nil {
		t.Fatal("probe did not reach caller deadline")
	}
	if err = tx.Rollback(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err = db.Ready(context.Background()); err != nil {
		t.Fatal("probe did not recover", err)
	}
}
