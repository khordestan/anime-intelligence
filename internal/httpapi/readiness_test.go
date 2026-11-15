package httpapi

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/khordestan/anime-intelligence/internal/platform/store"
)

func TestReadinessIsBoundedUncachedAndDoesNotDiscloseFailure(t *testing.T) {
	for _, unavailable := range []bool{false, true} {
		check := func(ctx context.Context) error {
			deadline, ok := ctx.Deadline()
			if !ok || time.Until(deadline) > 2*time.Second {
				t.Fatal("probe not bounded")
			}
			if unavailable {
				return errors.New("synthetic-secret-database-and-role-details")
			}
			return nil
		}
		response := httptest.NewRecorder()
		readinessHandler("postgresql", check)(response, httptest.NewRequest("GET", "/readyz", nil))
		want := 200
		if unavailable {
			want = 503
		}
		if response.Code != want || response.Header().Get("Cache-Control") != "no-store" || strings.Contains(response.Body.String(), "synthetic-secret") {
			t.Fatal("readiness response leaked or cached", response.Code)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	response := httptest.NewRecorder()
	readinessHandler("postgresql", func(ctx context.Context) error { return ctx.Err() })(response, httptest.NewRequest("GET", "/readyz", nil).WithContext(ctx))
	if response.Code != 503 {
		t.Fatal("cancelled probe reported ready")
	}
}
func TestLocalReadinessIsSeparateFromLiveness(t *testing.T) {
	data, err := store.New("")
	if err != nil {
		t.Fatal(err)
	}
	handler := NewRouter(slog.New(slog.NewTextHandler(io.Discard, nil)), data)
	for _, path := range []string{"/healthz", "/readyz"} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest("GET", path, nil))
		if response.Code != 200 || !strings.Contains(response.Body.String(), "local-file") {
			t.Fatal("missing local probe", path, response.Code)
		}
	}
}

func TestReadinessRejectsOverlappingDatabaseChecks(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	done := make(chan struct{})
	handler := readinessHandler("postgresql", func(context.Context) error { close(started); <-release; return nil })
	first := httptest.NewRecorder()
	go func() { handler(first, httptest.NewRequest("GET", "/readyz", nil)); close(done) }()
	<-started
	second := httptest.NewRecorder()
	handler(second, httptest.NewRequest("GET", "/readyz", nil))
	if second.Code != 503 {
		t.Fatal("overlapping database probe was not bounded")
	}
	close(release)
	<-done
	if first.Code != 200 {
		t.Fatal("first probe did not complete")
	}
}
