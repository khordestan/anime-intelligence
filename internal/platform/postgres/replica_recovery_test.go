package postgres_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/khordestan/anime-intelligence/internal/httpapi"
	"github.com/khordestan/anime-intelligence/internal/platform/postgres"
	"github.com/khordestan/anime-intelligence/internal/platform/store"
)

func TestDashboardProgressRetryAfterReplicaResponseLoss(t *testing.T) {
	db, _, _, runtimeDSN := testDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	account, err := db.Register(ctx, "replica-recovery@example.test", "synthetic-replica-recovery-password")
	if err != nil {
		t.Fatal(err)
	}
	verifyAccount(t, db, account.Token)
	entry, err := db.Repository(account.Token).Add(ctx, "10000000-0000-4000-8000-000000000001", "WATCHING", 0)
	if err != nil {
		t.Fatal(err)
	}
	replica, err := postgres.Open(ctx, runtimeDSN)
	if err != nil {
		t.Fatal(err)
	}
	defer replica.Close()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	primary := httpapi.NewPostgresRouter(logger, db)
	finished := make(chan int, 1)
	first := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capture := httptest.NewRecorder()
		primary.ServeHTTP(capture, r)
		finished <- capture.Code
		connection, _, err := http.NewResponseController(w).Hijack()
		if err != nil {
			t.Errorf("could not interrupt test connection: %v", err)
			http.Error(w, "interruption unavailable", http.StatusInternalServerError)
			return
		}
		_ = connection.Close()
	}))
	defer first.Close()
	second := httptest.NewServer(httpapi.NewPostgresRouter(logger, replica))
	defer second.Close()
	call := func(server *httptest.Server, method, path, body string, authenticated bool) (int, []byte, error) {
		req, err := http.NewRequestWithContext(ctx, method, server.URL+path, strings.NewReader(body))
		if err != nil {
			return 0, nil, err
		}
		if authenticated {
			req.Header.Set("Authorization", "Bearer "+account.Token)
		}
		if body != "" {
			req.Header.Set("Content-Type", "application/json")
		}
		response, err := server.Client().Do(req)
		if err != nil {
			return 0, nil, err
		}
		defer response.Body.Close()
		payload, err := io.ReadAll(response.Body)
		return response.StatusCode, payload, err
	}
	advance := func(server *httptest.Server, expected int) (int, []byte, error) {
		body := fmt.Sprintf(`{"entries":[{"id":%q,"expectedCycle":1,"expectedEpisodesWatched":%d}]}`, entry.ID, expected)
		return call(server, http.MethodPatch, "/api/v1/tracker/bulk-progress", body, true)
	}
	if status, _, err := advance(first, 0); status != 0 || !errors.Is(err, io.EOF) {
		t.Fatalf("expected no response, status=%d err=%v", status, err)
	}
	select {
	case status := <-finished:
		if status != http.StatusOK {
			t.Fatalf("original write failed before losing the response: %d", status)
		}
	case <-ctx.Done():
		t.Fatal("original handler did not finish")
	}
	first.Close()

	if status, _, err := call(second, http.MethodGet, "/readyz", "", false); status != http.StatusOK || err != nil {
		t.Fatalf("surviving replica not ready: %d %v", status, err)
	}
	if status, _, err := advance(second, 0); status != http.StatusConflict || err != nil {
		t.Fatalf("replay did not enforce the original count: %d %v", status, err)
	}
	readProgress := func(want int) {
		t.Helper()
		status, raw, err := call(second, http.MethodGet, "/api/v1/tracker", "", true)
		var library struct{ Items []store.Entry }
		if status != http.StatusOK || err != nil || json.Unmarshal(raw, &library) != nil ||
			len(library.Items) != 1 || library.Items[0].ID != entry.ID || library.Items[0].EpisodesWatched != want {
			t.Fatalf("survivor progress=%+v status=%d err=%v, want %d", library.Items, status, err, want)
		}
	}
	readProgress(1)
	if status, _, err := advance(second, 1); status != http.StatusOK || err != nil {
		t.Fatalf("fresh progress update failed: %d %v", status, err)
	}
	readProgress(2)
}
