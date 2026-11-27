package postgres_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/khordestan/anime-intelligence/internal/httpapi"
	"github.com/khordestan/anime-intelligence/internal/platform/postgres"
	"github.com/khordestan/anime-intelligence/internal/platform/store"
)

func TestPlanningRelinkRequiresOwnedReviewedEmptyEntry(t *testing.T) {
	db, _, _, _ := testDB(t)
	ctx := context.Background()
	register := func(email string) postgres.Session {
		t.Helper()
		session, err := db.Register(ctx, email, "synthetic-planning-relink-password")
		if err != nil {
			t.Fatal(err)
		}
		return session
	}
	owner, other := register("planning-first@example.test"), register("planning-other@example.test")
	verifyAccount(t, db, owner.Token)
	const source = "10000000-0000-4000-8000-000000000001"
	const target = "10000000-0000-4000-8000-000000000002"
	repo := db.Repository(owner.Token)
	entry, err := repo.Add(ctx, source, "PLANNING", 0)
	if err != nil {
		t.Fatal(err)
	}
	h := httpapi.NewPostgresRouter(slog.New(slog.NewTextHandler(io.Discard, nil)), db)
	move := func(token, destination string, expected time.Time, want int) store.Entry {
		t.Helper()
		body, err := json.Marshal(map[string]any{"targetAnimeId": destination, "expectedUpdatedAt": expected})
		if err != nil {
			t.Fatal(err)
		}
		request := httptest.NewRequest("POST", "/api/v1/tracker/"+entry.ID+"/relink-planning", bytes.NewReader(body))
		if token != "" {
			request.Header.Set("Authorization", "Bearer "+token)
		}
		recorder := httptest.NewRecorder()
		h.ServeHTTP(recorder, request)
		if recorder.Code != want || recorder.Header().Get("Cache-Control") != "no-store" {
			t.Fatalf("move status=%d, want=%d: %s", recorder.Code, want, recorder.Body)
		}
		var out store.Entry
		if want == 200 && json.Unmarshal(recorder.Body.Bytes(), &out) != nil {
			t.Fatalf("invalid planning response: %s", recorder.Body)
		}
		return out
	}
	move("", target, entry.UpdatedAt, 401)
	move(other.Token, target, entry.UpdatedAt, 403)
	verifyAccount(t, db, other.Token)
	otherRepo := db.Repository(other.Token)
	otherEntry, err := otherRepo.Add(ctx, target, "WATCHING", 3)
	if err != nil {
		t.Fatal(err)
	}
	move(other.Token, target, entry.UpdatedAt, 404)
	move(owner.Token, "10000000-0000-4000-8000-000000009999", entry.UpdatedAt, 404)
	move(owner.Token, source, entry.UpdatedAt, 409)
	if used, err := repo.Add(ctx, target, "PLANNING", 0); err != nil {
		t.Fatal(err)
	} else {
		move(owner.Token, target, entry.UpdatedAt, 409)
		if removed, err := repo.Remove(ctx, used.ID); err != nil || !removed {
			t.Fatal("remove fixture destination", err)
		}
	}
	note, found, err := repo.SetTrackerNotes(ctx, entry.ID, "Synthetic private note", entry.UpdatedAt)
	if err != nil || !found {
		t.Fatal("note fixture", err)
	}
	move(owner.Token, target, note.UpdatedAt, 409)
	cleared, found, err := repo.SetTrackerNotes(ctx, entry.ID, "", note.UpdatedAt)
	if err != nil || !found {
		t.Fatal("clear fixture note", err)
	}
	move(owner.Token, target, entry.UpdatedAt, 409)
	relinked := move(owner.Token, target, cleared.UpdatedAt, 200)
	if relinked.ID == "" || relinked.ID == entry.ID || relinked.AnimeID != target ||
		relinked.WatchStatus != "PLANNING" || relinked.EpisodesWatched != 0 {
		t.Fatalf("invalid replacement: %+v", relinked)
	}
	move(owner.Token, target, cleared.UpdatedAt, 404)
	if _, found, err := repo.UpdateCycle(ctx, entry.ID, nil, nil, 1); err != nil || found {
		t.Fatal("stale tracker ID resolved after replacement", err)
	}
	ours, err := repo.Library(ctx)
	if err != nil || len(ours) != 1 || ours[0].ID != relinked.ID {
		t.Fatal("replacement did not leave one owned entry", err)
	}
	theirs, err := otherRepo.Library(ctx)
	if err != nil || len(theirs) != 1 || theirs[0].ID != otherEntry.ID || theirs[0].EpisodesWatched != 3 {
		t.Fatal("replacement affected another owner's planning data", err)
	}
}
