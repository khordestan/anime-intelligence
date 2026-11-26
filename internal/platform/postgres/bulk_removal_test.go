package postgres_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http/httptest"
	"sort"
	"testing"

	"github.com/khordestan/anime-intelligence/internal/httpapi"
	"github.com/khordestan/anime-intelligence/internal/platform/store"
)

func TestBulkLibraryRemovalChecksEveryReviewedEntry(t *testing.T) {
	ctx := context.Background()
	db, admin, _, _ := testDB(t)
	owner, err := db.Register(ctx, "bulk-remove@example.test", "synthetic-bulk-removal-password")
	if err != nil {
		t.Fatal(err)
	}
	verifyAccount(t, db, owner.Token)
	repo := db.Repository(owner.Token)
	first, err := repo.Add(ctx, "10000000-0000-4000-8000-000000000001", "COMPLETED", 12)
	if err != nil {
		t.Fatal(err)
	}
	second, err := repo.Add(ctx, "10000000-0000-4000-8000-000000000002", "COMPLETED", 25)
	if err != nil {
		t.Fatal(err)
	}
	entries := []store.Entry{first, second}
	sort.Slice(entries, func(i, j int) bool { return entries[i].ID < entries[j].ID })
	for i, entry := range entries {
		if _, _, err := repo.Rate(ctx, entry.AnimeID, 4); err != nil {
			t.Fatal(err)
		}
		active, found, err := repo.StartRewatch(ctx, entry.ID, 1)
		if err != nil || !found {
			t.Fatalf("setup rewatch: %+v %v", active, err)
		}
		entries[i] = active
	}
	targets := make([]store.BulkRemovalTarget, 0, len(entries))
	for _, entry := range entries {
		targets = append(targets, store.BulkRemovalTarget{
			BulkStatusTarget:  store.BulkStatusTarget{ID: entry.ID, ExpectedCycle: entry.WatchCycle},
			ExpectedUpdatedAt: entry.UpdatedAt,
		})
	}
	handler := httpapi.NewPostgresRouter(slog.New(slog.NewTextHandler(io.Discard, nil)), db)
	call := func(token string, selected []store.BulkRemovalTarget, want int) {
		t.Helper()
		body, err := json.Marshal(map[string]any{"entries": selected})
		if err != nil {
			t.Fatal(err)
		}
		request := httptest.NewRequest("POST", "/api/v1/tracker/bulk-remove", bytes.NewReader(body))
		request.Header.Set("Authorization", "Bearer "+token)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != want || (want == 204 && response.Body.Len() != 0) {
			t.Fatalf("status=%d want=%d: %s", response.Code, want, response.Body)
		}
	}
	other, err := db.Register(ctx, "other-bulk-remove@example.test", "synthetic-bulk-removal-password")
	if err != nil {
		t.Fatal(err)
	}
	call(other.Token, targets, 403)
	verifyAccount(t, db, other.Token)
	call(other.Token, targets, 404)
	call(owner.Token, []store.BulkRemovalTarget{targets[0], targets[0]}, 400)
	stale := append([]store.BulkRemovalTarget(nil), targets...)
	stale[1].ExpectedCycle--
	call(owner.Token, stale, 409)

	if _, err := admin.Exec(ctx, `UPDATE user_anime SET updated_at=updated_at+interval '1 second' WHERE id=$1`, targets[1].ID); err != nil {
		t.Fatal(err)
	}
	call(owner.Token, targets, 409)
	library, err := repo.Library(ctx)
	if err != nil || len(library) != 2 {
		t.Fatalf("stale batch partially removed entries: %+v %v", library, err)
	}
	targets = targets[:0]
	for _, entry := range library {
		targets = append(targets, store.BulkRemovalTarget{
			BulkStatusTarget:  store.BulkStatusTarget{ID: entry.ID, ExpectedCycle: entry.WatchCycle},
			ExpectedUpdatedAt: entry.UpdatedAt,
		})
	}
	call(owner.Token, targets, 204)
	call(owner.Token, targets, 404)
	export, err := db.ExportAccount(ctx, owner.Token)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"tracker", "watchCycles", "ratings", "progress"} {
		if string(export.Data[key]) != "[]" {
			t.Errorf("orphaned %s: %s", key, export.Data[key])
		}
	}
}
