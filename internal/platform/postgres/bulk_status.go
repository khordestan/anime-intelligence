package postgres

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/khordestan/anime-intelligence/internal/platform/store"
)

// UpdateStatuses applies status-only edits in one verified-owner transaction.
// A missing/stale target rolls back every preceding edit.
func (r *Repository) UpdateStatuses(ctx context.Context, status string, targets []store.BulkStatusTarget) ([]store.Entry, bool, error) {
	switch status {
	case "PLANNING", "WATCHING", "COMPLETED", "ON_HOLD", "DROPPED":
	default:
		return nil, false, store.ErrBulkStatusInvalid
	}
	return r.updateTrackerBatch(ctx, targets, func(ctx context.Context, tx pgx.Tx, ownerID string, target store.BulkStatusTarget) (store.Entry, bool, error) {
		return updateCycleEntry(ctx, tx, ownerID, target.ID, &status, nil, target.ExpectedCycle)
	})
}

// UpdateRatings clears the current series rating when rating is zero.
func (r *Repository) UpdateRatings(ctx context.Context, rating int, targets []store.BulkStatusTarget) ([]store.Entry, bool, error) {
	if rating < 0 || rating > 5 {
		return nil, false, store.ErrBulkStatusInvalid
	}
	return r.updateTrackerBatch(ctx, targets, func(ctx context.Context, tx pgx.Tx, ownerID string, target store.BulkStatusTarget) (store.Entry, bool, error) {
		entry, found, err := updateCycleEntry(ctx, tx, ownerID, target.ID, nil, nil, target.ExpectedCycle)
		if err != nil || !found {
			return entry, found, err
		}
		if rating == 0 {
			_, err = tx.Exec(ctx, `DELETE FROM user_ratings WHERE user_id=$1 AND anime_id=$2 AND scope='SERIES' AND season_key IS NULL`, ownerID, entry.AnimeID)
		} else {
			err = upsertSeriesRating(ctx, tx, ownerID, entry.AnimeID, rating)
		}
		if err != nil {
			return store.Entry{}, false, err
		}
		entry.Rating = rating
		return entry, true, nil
	})
}

func (r *Repository) updateTrackerBatch(ctx context.Context, targets []store.BulkStatusTarget, edit func(context.Context, pgx.Tx, string, store.BulkStatusTarget) (store.Entry, bool, error)) ([]store.Entry, bool, error) {
	if len(targets) == 0 || len(targets) > store.MaxBulkStatusEntries {
		return nil, false, store.ErrBulkStatusInvalid
	}
	byID := make(map[string]int, len(targets))
	ids := make([]string, 0, len(targets))
	for index, target := range targets {
		id := strings.ToLower(target.ID)
		if _, exists := byID[id]; exists || target.ExpectedCycle < 1 || target.ExpectedCycle > 1000 {
			return nil, false, store.ErrBulkStatusInvalid
		}
		byID[id] = index
		ids = append(ids, id)
	}
	// Lock tracker entries in the same order across overlapping batches.
	sort.Strings(ids)
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	tx, owner, err := r.db.productTx(ctx, r.token)
	if err != nil {
		return nil, false, err
	}
	defer rollback(tx)
	items := make([]store.Entry, len(targets))
	for _, id := range ids {
		index := byID[id]
		entry, found, err := edit(ctx, tx, owner.ID, targets[index])
		if err != nil || !found {
			return nil, found, err
		}
		items[index] = entry
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, false, err
	}
	return items, true, nil
}
