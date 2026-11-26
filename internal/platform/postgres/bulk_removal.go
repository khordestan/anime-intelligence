package postgres

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/khordestan/anime-intelligence/internal/platform/store"
)

func (r *Repository) RemoveEntries(ctx context.Context, targets []store.BulkRemovalTarget) (bool, error) {
	if len(targets) < 1 || len(targets) > store.MaxBulkStatusEntries {
		return false, store.ErrBulkStatusInvalid
	}
	cycles := make([]store.BulkStatusTarget, 0, len(targets))
	expected := make(map[string]time.Time, len(targets))
	for _, target := range targets {
		if target.ExpectedUpdatedAt.IsZero() {
			return false, store.ErrBulkStatusInvalid
		}
		cycles = append(cycles, target.BulkStatusTarget)
		expected[strings.ToLower(target.ID)] = target.ExpectedUpdatedAt
	}
	_, found, err := r.updateTrackerBatch(ctx, cycles, func(ctx context.Context, tx pgx.Tx, owner string, target store.BulkStatusTarget) (store.Entry, bool, error) {
		var cycle int
		var updatedAt time.Time
		err := tx.QueryRow(ctx, `SELECT watch_cycle,updated_at FROM user_anime WHERE id=$1 AND user_id=$2 FOR UPDATE`, target.ID, owner).Scan(&cycle, &updatedAt)
		if errors.Is(err, pgx.ErrNoRows) {
			return store.Entry{}, false, nil
		}
		if err != nil {
			return store.Entry{}, false, err
		}
		if cycle != target.ExpectedCycle || !updatedAt.Equal(expected[strings.ToLower(target.ID)]) {
			return store.Entry{}, true, store.ErrBulkRemovalChanged
		}
		removed, err := removeOwnedTracker(ctx, tx, owner, target.ID)
		return store.Entry{}, removed, err
	})
	return found, err
}
