package postgres

import (
	"context"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/khordestan/anime-intelligence/internal/platform/store"
)

// AdvanceProgress adds one watched episode or records a reviewed known total.
// The expected count and cycle fence retries and overlapping edits.
func (r *Repository) AdvanceProgress(ctx context.Context, targets []store.BulkProgressTarget) ([]store.Entry, bool, error) {
	if len(targets) == 0 || len(targets) > store.MaxBulkStatusEntries {
		return nil, false, store.ErrBulkStatusInvalid
	}
	batch := make([]store.BulkStatusTarget, 0, len(targets))
	expected := make(map[string]store.BulkProgressTarget, len(targets))
	for _, target := range targets {
		if target.ExpectedEpisodesWatched == nil || *target.ExpectedEpisodesWatched < 0 || *target.ExpectedEpisodesWatched >= 100000 {
			return nil, false, store.ErrBulkStatusInvalid
		}
		if target.ExpectedTotalEpisodes != nil && (*target.ExpectedTotalEpisodes <= *target.ExpectedEpisodesWatched || *target.ExpectedTotalEpisodes > 100000) {
			return nil, false, store.ErrBulkStatusInvalid
		}
		batch = append(batch, target.BulkStatusTarget)
		expected[strings.ToLower(target.ID)] = target
	}
	return r.updateTrackerBatch(ctx, batch, func(ctx context.Context, tx pgx.Tx, ownerID string, target store.BulkStatusTarget) (store.Entry, bool, error) {
		entry, found, err := updateCycleEntry(ctx, tx, ownerID, target.ID, nil, nil, target.ExpectedCycle)
		if err != nil || !found {
			return entry, found, err
		}
		selection := expected[strings.ToLower(target.ID)]
		if entry.ExactProgress || entry.EpisodesWatched != *selection.ExpectedEpisodesWatched {
			return store.Entry{}, true, store.ErrBulkProgressConflict
		}
		var total *int
		if err := tx.QueryRow(ctx, `SELECT episode_count FROM anime WHERE id=$1`, entry.AnimeID).Scan(&total); err != nil {
			return store.Entry{}, false, err
		}
		next := entry.EpisodesWatched + 1
		if selection.ExpectedTotalEpisodes != nil {
			if total == nil || *total != *selection.ExpectedTotalEpisodes {
				return store.Entry{}, true, store.ErrBulkProgressConflict
			}
			next = *total
		}
		if total != nil && next > *total {
			return store.Entry{}, true, store.ErrBulkProgressConflict
		}
		if err := writeAggregateProgress(ctx, tx, ownerID, entry.AnimeID, next); err != nil {
			return store.Entry{}, false, err
		}
		entry.EpisodesWatched = next
		return entry, true, nil
	})
}
