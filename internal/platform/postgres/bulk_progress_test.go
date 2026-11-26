package postgres_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sort"
	"testing"

	"github.com/khordestan/anime-intelligence/internal/httpapi"
	"github.com/khordestan/anime-intelligence/internal/platform/store"
)

func TestBulkProgressFencesRetriesAndRejectsExactAndFullEntries(t *testing.T) {
	db, admin, _, _ := testDB(t)
	ctx := context.Background()
	first, err := db.Register(ctx, "progress-one@example.test", "synthetic-bulk-progress-password")
	if err != nil {
		t.Fatal(err)
	}
	second, err := db.Register(ctx, "progress-two@example.test", "synthetic-bulk-progress-password")
	if err != nil {
		t.Fatal(err)
	}
	verifyAccount(t, db, first.Token)
	verifyAccount(t, db, second.Token)
	repo := db.Repository(first.Token)
	a, err := repo.Add(ctx, "10000000-0000-4000-8000-000000000001", "WATCHING", 0)
	if err != nil {
		t.Fatal(err)
	}
	b, err := repo.Add(ctx, "10000000-0000-4000-8000-000000000002", "WATCHING", 0)
	if err != nil {
		t.Fatal(err)
	}
	ordered := []store.Entry{a, b}
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].ID < ordered[j].ID })
	handler := httpapi.NewPostgresRouter(slog.New(slog.NewTextHandler(io.Discard, nil)), db)
	call := func(token string, firstCount, secondCount int, want int, totals ...int) {
		t.Helper()
		targets := []store.BulkProgressTarget{
			{BulkStatusTarget: store.BulkStatusTarget{ID: ordered[0].ID, ExpectedCycle: 1}, ExpectedEpisodesWatched: &firstCount},
			{BulkStatusTarget: store.BulkStatusTarget{ID: ordered[1].ID, ExpectedCycle: 1}, ExpectedEpisodesWatched: &secondCount},
		}
		for i := range totals {
			targets[i].ExpectedTotalEpisodes = &totals[i]
		}
		body, err := json.Marshal(map[string]any{"entries": targets})
		if err != nil {
			t.Fatal(err)
		}
		req := httptest.NewRequest(http.MethodPatch, "/api/v1/tracker/bulk-progress", bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+token)
		res := httptest.NewRecorder()
		handler.ServeHTTP(res, req)
		if res.Code != want {
			t.Fatalf("response %d, want %d: %s", res.Code, want, res.Body)
		}
	}
	check := func() {
		t.Helper()
		rows, err := repo.Library(ctx)
		if err != nil || len(rows) != 2 {
			t.Fatalf("library: %+v %v", rows, err)
		}
		for _, row := range rows {
			if row.EpisodesWatched != 1 || row.WatchStatus != "WATCHING" {
				t.Fatalf("unexpected persisted tracker entry: %+v", row)
			}
		}
	}
	call(second.Token, 0, 0, 404)
	call(first.Token, 0, 0, 200)
	call(first.Token, 0, 0, 409)
	call(first.Token, 1, 0, 409)
	check()
	for _, entry := range ordered {
		if _, err := admin.Exec(ctx, `UPDATE anime SET episode_count=5 WHERE id=$1`, entry.AnimeID); err != nil {
			t.Fatal(err)
		}
	}
	call(first.Token, 1, 1, 409, 5, 6)
	check()
	call(first.Token, 1, 1, 200, 5, 5)
	updated, err := repo.Library(ctx)
	if err != nil || len(updated) != 2 {
		t.Fatalf("library after known totals: %+v %v", updated, err)
	}
	for _, entry := range updated {
		if entry.EpisodesWatched != 5 || entry.WatchStatus != "WATCHING" || entry.ExactProgress {
			t.Fatalf("unexpected completed count: %+v", entry)
		}
	}
	call(first.Token, 1, 1, 409, 5, 5)

	if _, err := admin.Exec(ctx, `UPDATE anime SET episode_count=1 WHERE id=$1`, ordered[1].AnimeID); err != nil {
		t.Fatal(err)
	}
	call(first.Token, 5, 5, 409)
	if _, _, err := repo.SetEpisodeProgress(ctx, ordered[1].ID, []int32{1}); err != nil {
		t.Fatal(err)
	}
	call(first.Token, 5, 5, 409)
	rows, err := repo.Library(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range rows {
		if row.ID == ordered[1].ID && !row.ExactProgress {
			t.Fatal("batch replaced exact progress")
		}
	}
}
