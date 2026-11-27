package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/khordestan/anime-intelligence/internal/platform/store"
)

var (
	ErrPlanningRelinkConflict = errors.New("planning entry or destination changed")
	ErrPlanningRelinkTarget   = errors.New("planning destination does not exist")
)

// RelinkPlanning only replaces an empty, first-pass planning entry. A new ID
// prevents stale writes to the reviewed source entry from updating the destination.
func (r *Repository) RelinkPlanning(ctx context.Context, entryID, targetAnimeID string, expected time.Time) (store.Entry, bool, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	tx, owner, err := r.db.productTx(ctx, r.token)
	if err != nil {
		return store.Entry{}, false, err
	}
	defer rollback(tx)

	var source string
	var updatedAt time.Time
	var emptyPlanning bool
	err = tx.QueryRow(ctx, `SELECT anime_id::text,updated_at,
		watch_status='PLANNING' AND watch_cycle=1 AND started_at IS NULL AND completed_at IS NULL AND COALESCE(notes,'')=''
		FROM user_anime WHERE id=$1 AND user_id=$2 FOR UPDATE`, entryID, owner.ID).Scan(&source, &updatedAt, &emptyPlanning)
	if errors.Is(err, pgx.ErrNoRows) {
		return store.Entry{}, false, nil
	}
	if err != nil {
		return store.Entry{}, false, err
	}
	if !emptyPlanning || !updatedAt.Equal(expected) || source == targetAnimeID {
		return store.Entry{}, true, ErrPlanningRelinkConflict
	}

	var destination, canMove bool
	err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM anime WHERE id=$3),
		NOT EXISTS(SELECT 1 FROM watch_progress WHERE user_id=$1 AND
			(anime_id=$3 OR (anime_id=$2 AND (episodes_watched<>0 OR watched_episode_numbers IS NOT NULL OR total_episodes IS NOT NULL))))
		AND NOT EXISTS(SELECT 1 FROM watch_cycles WHERE user_id=$1 AND anime_id=$2)
		AND NOT EXISTS(SELECT 1 FROM user_ratings WHERE user_id=$1 AND anime_id IN ($2,$3))`,
		owner.ID, source, targetAnimeID).Scan(&destination, &canMove)
	if err != nil {
		return store.Entry{}, true, err
	}
	if !destination {
		return store.Entry{}, true, ErrPlanningRelinkTarget
	}
	if !canMove {
		return store.Entry{}, true, ErrPlanningRelinkConflict
	}

	var newEntryID string
	err = tx.QueryRow(ctx, `INSERT INTO user_anime(user_id,anime_id,watch_status)
		VALUES($1,$2,'PLANNING') ON CONFLICT(user_id,anime_id) DO NOTHING RETURNING id::text`,
		owner.ID, targetAnimeID).Scan(&newEntryID)
	if errors.Is(err, pgx.ErrNoRows) {
		return store.Entry{}, true, ErrPlanningRelinkConflict
	}
	if err != nil {
		return store.Entry{}, true, err
	}
	progress, err := tx.Exec(ctx, `INSERT INTO watch_progress(user_id,anime_id,episodes_watched)
		VALUES($1,$2,0) ON CONFLICT(user_id,anime_id) DO NOTHING`, owner.ID, targetAnimeID)
	if err != nil {
		return store.Entry{}, true, err
	}
	if progress.RowsAffected() == 0 {
		return store.Entry{}, true, ErrPlanningRelinkConflict
	}
	if _, err = tx.Exec(ctx, `DELETE FROM watch_progress WHERE user_id=$1 AND anime_id=$2`, owner.ID, source); err != nil {
		return store.Entry{}, true, err
	}
	if _, err = tx.Exec(ctx, `DELETE FROM user_anime WHERE user_id=$1 AND id=$2`, owner.ID, entryID); err != nil {
		return store.Entry{}, true, err
	}
	entry, err := scanEntry(tx.QueryRow(ctx, entrySelect+` WHERE ua.user_id=$1 AND ua.id=$2`, owner.ID, newEntryID))
	if err != nil {
		return store.Entry{}, true, err
	}
	return entry, true, tx.Commit(ctx)
}
