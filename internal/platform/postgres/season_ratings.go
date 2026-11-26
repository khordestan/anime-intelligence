package postgres

import (
	"context"

	"github.com/khordestan/anime-intelligence/internal/platform/store"
)

// The scope is attached to the canonical title, independent of source numbering.
const canonicalSeasonRatingKey = "canonical-title"

func (r *Repository) SetSeasonRating(ctx context.Context, id string, expectedCycle int, score *int) (store.Entry, bool, error) {
	if expectedCycle < 1 || expectedCycle > 1000 {
		return store.Entry{}, false, store.ErrWatchCycleInvalid
	}
	if score != nil && (*score < 1 || *score > 5) {
		return store.Entry{}, false, store.ErrSeasonRatingUnavailable
	}
	tx, owner, err := r.db.productTx(ctx, r.token)
	if err != nil {
		return store.Entry{}, false, err
	}
	defer rollback(tx)
	entry, found, err := updateCycleEntry(ctx, tx, owner.ID, id, nil, nil, expectedCycle)
	if err != nil || !found {
		return entry, found, err
	}
	if !entry.SeasonRatingAvailable && score != nil {
		return entry, true, store.ErrSeasonRatingUnavailable
	}

	if score == nil {
		_, err = tx.Exec(ctx, `DELETE FROM user_ratings WHERE user_id=$1 AND anime_id=$2 AND scope='SEASON' AND season_key=$3`, owner.ID, entry.AnimeID, canonicalSeasonRatingKey)
	} else {
		_, err = tx.Exec(ctx, `INSERT INTO user_ratings(user_id,anime_id,scope,season_key,score) VALUES($1,$2,'SEASON',$3,$4)
			ON CONFLICT(user_id,anime_id,scope,season_key) DO UPDATE SET score=excluded.score,rated_at=now()`,
			owner.ID, entry.AnimeID, canonicalSeasonRatingKey, *score)
	}
	if err != nil {
		return store.Entry{}, false, err
	}
	entry, err = scanEntry(tx.QueryRow(ctx, entrySelect+` WHERE ua.id=$1 AND ua.user_id=$2`, id, owner.ID))
	if err != nil {
		return store.Entry{}, false, err
	}
	return entry, true, tx.Commit(ctx)
}
