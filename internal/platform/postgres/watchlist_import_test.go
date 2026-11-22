package postgres_test

import (
	"context"
	"errors"
	"testing"

	"github.com/khordestan/anime-intelligence/internal/platform/postgres"
	"github.com/khordestan/anime-intelligence/internal/platform/store"
)

func TestWatchListImportAddsMissingTitlesAtomically(t *testing.T) {
	db, _, _, _ := testDB(t)
	ctx := context.Background()
	session, err := db.Register(ctx, "watch-list-import@example.test", "synthetic-watch-list-password")
	if err != nil {
		t.Fatal(err)
	}
	const existingID = "10000000-0000-4000-8000-000000000001"
	const exactID = "10000000-0000-4000-8000-000000000002"
	const emptyID = "10000000-0000-4000-8000-000000000003"
	rating := 5
	data := postgres.WatchListExport{Version: 1, Entries: []postgres.WatchListExportEntry{
		{AnimeID: existingID, WatchStatus: "DROPPED"},
		{AnimeID: exactID, WatchStatus: "WATCHING", EpisodesWatched: 2, WatchedEpisodeNumbers: []int32{1, 4}, Rating: &rating},
		{AnimeID: emptyID, WatchStatus: "ON_HOLD", WatchedEpisodeNumbers: []int32{}},
	}}
	repo := db.Repository(session.Token)
	if _, err := repo.ImportWatchList(ctx, data); !errors.Is(err, store.ErrEmailUnverified) {
		t.Fatalf("unverified import: %v", err)
	}
	verifyAccount(t, db, session.Token)
	if _, err := repo.Add(ctx, existingID, "COMPLETED", 12); err != nil {
		t.Fatal(err)
	}
	invalid := data
	invalid.Entries = append(append([]postgres.WatchListExportEntry{}, data.Entries...),
		postgres.WatchListExportEntry{AnimeID: "ffffffff-ffff-4fff-8fff-ffffffffffff", WatchStatus: "PLANNING"})
	if _, err := repo.ImportWatchList(ctx, invalid); !errors.Is(err, postgres.ErrWatchListCatalog) {
		t.Fatalf("invalid catalog identity: %v", err)
	}
	if _, err := repo.PreviewWatchList(ctx, invalid); !errors.Is(err, postgres.ErrWatchListCatalog) {
		t.Fatalf("preview accepted unknown catalog identity: %v", err)
	}
	if out, err := repo.PreviewWatchList(ctx, data); err != nil || out.Added != 2 || out.Skipped != 1 {
		t.Fatalf("preview: %+v err=%v", out, err)
	}
	current, err := repo.ExportWatchList(ctx)
	if err != nil || len(current.Entries) != 1 {
		t.Fatalf("invalid upload persisted changes: %+v err=%v", current, err)
	}
	if out, err := repo.ImportWatchList(ctx, data); err != nil || out.Added != 2 || out.Skipped != 1 {
		t.Fatalf("first import: %+v err=%v", out, err)
	}
	if out, err := repo.ImportWatchList(ctx, data); err != nil || out.Added != 0 || out.Skipped != 3 {
		t.Fatalf("replayed import: %+v err=%v", out, err)
	}
	current, err = repo.ExportWatchList(ctx)
	if err != nil || len(current.Entries) != 3 {
		t.Fatalf("export after import: %+v err=%v", current, err)
	}
	before, exact, empty := current.Entries[0], current.Entries[1], current.Entries[2]
	if before.AnimeID != existingID || before.WatchStatus != "COMPLETED" || before.EpisodesWatched != 12 {
		t.Fatalf("existing entry changed: %+v", before)
	}
	if exact.AnimeID != exactID || exact.EpisodesWatched != 2 || len(exact.WatchedEpisodeNumbers) != 2 ||
		exact.WatchedEpisodeNumbers[1] != 4 || exact.Rating == nil || *exact.Rating != 5 {
		t.Fatalf("exact progress or rating lost: %+v", exact)
	}
	if empty.AnimeID != emptyID || empty.WatchedEpisodeNumbers == nil || len(empty.WatchedEpisodeNumbers) != 0 {
		t.Fatalf("empty exact mode lost: %+v", empty)
	}
}
