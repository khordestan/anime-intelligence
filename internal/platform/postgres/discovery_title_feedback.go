package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/khordestan/anime-intelligence/internal/modules/intelligence"
)

var ErrDiscoveryDirection = errors.New("invalid discovery feedback direction")

// SetDiscoveryTitleFeedback records a reaction to a title with currently eligible
// evidence. Direction zero clears the reaction even if the evidence later changes.
func (r *Repository) SetDiscoveryTitleFeedback(ctx context.Context, animeID string, direction int) (bool, error) {
	if direction < -1 || direction > 1 {
		return false, ErrDiscoveryDirection
	}
	tx, owner, err := r.db.productTx(ctx, r.token)
	if err != nil {
		return false, err
	}
	defer rollback(tx)
	if direction == 0 {
		if _, err := tx.Exec(ctx, `DELETE FROM discovery_title_feedback WHERE user_id=$1 AND anime_id=$2`, owner.ID, animeID); err != nil {
			return false, err
		}
		return true, tx.Commit(ctx)
	}

	var eligible bool
	err = tx.QueryRow(ctx, `WITH latest AS (
	  SELECT DISTINCT ON (o.feature_id) o.value
	  FROM anime_feature_observations o JOIN anime_features f ON f.id=o.feature_id
	  WHERE o.anime_id=$2 AND `+eligibleRecommendationObservation+`
	  ORDER BY o.feature_id,o.observed_at DESC,o.id DESC
	), candidate AS (
	  SELECT a.id FROM anime a WHERE a.id=$2 AND EXISTS (SELECT 1 FROM latest WHERE value>0)
	    AND NOT EXISTS (SELECT 1 FROM user_anime ua WHERE ua.user_id=$1 AND ua.anime_id=a.id)
	    AND NOT EXISTS (SELECT 1 FROM personal_match_dismissals d WHERE d.user_id=$1 AND d.anime_id=a.id)
	), saved AS (
	  INSERT INTO discovery_title_feedback(user_id,anime_id,direction) SELECT $1,id,$3 FROM candidate
	  ON CONFLICT (user_id,anime_id) DO UPDATE SET direction=excluded.direction,updated_at=now()
	  WHERE discovery_title_feedback.direction<>excluded.direction
	)
	SELECT EXISTS (SELECT 1 FROM candidate)`, owner.ID, animeID, direction).Scan(&eligible)
	if err != nil || !eligible {
		return false, err
	}
	return true, tx.Commit(ctx)
}

func loadDiscoveryTitleFeedback(ctx context.Context, tx pgx.Tx, ownerID string) ([]intelligence.TitleFeedback, error) {
	rows, err := tx.Query(ctx, `SELECT anime_id::text,direction FROM discovery_title_feedback
		WHERE user_id=$1 ORDER BY updated_at DESC,anime_id LIMIT 100`, ownerID)
	if err != nil {
		return nil, err
	}
	var signals []intelligence.TitleFeedback
	var ids []string
	for rows.Next() {
		var id string
		var signal intelligence.TitleFeedback
		if err := rows.Scan(&id, &signal.Direction); err != nil {
			rows.Close()
			return nil, err
		}
		ids = append(ids, id)
		signals = append(signals, signal)
	}
	rows.Close()
	if err := rows.Err(); err != nil || len(ids) == 0 {
		return nil, err
	}
	features, err := recommendationFeatures(ctx, tx, ids)
	if err != nil {
		return nil, err
	}
	for i, id := range ids {
		signals[i].Features = features[id]
	}
	return signals, nil
}

type SavedDiscoveryFeedback struct {
	AnimeID   string `json:"animeId"`
	Title     string `json:"title"`
	Direction int    `json:"direction"`
}

type DiscoveryFeedbackPage struct {
	Items      []SavedDiscoveryFeedback `json:"items"`
	NextCursor string                   `json:"nextCursor,omitempty"`
}

func (r *Repository) DiscoveryTitleFeedback(ctx context.Context, after string) (DiscoveryFeedbackPage, error) {
	page := DiscoveryFeedbackPage{Items: []SavedDiscoveryFeedback{}}
	tx, owner, err := r.db.productTx(ctx, r.token)
	if err != nil {
		return page, err
	}
	defer rollback(tx)
	rows, err := tx.Query(ctx, `SELECT f.anime_id::text,a.canonical_title,f.direction
		FROM discovery_title_feedback f JOIN anime a ON a.id=f.anime_id
		WHERE f.user_id=$1 AND (NULLIF($2,'') IS NULL OR f.anime_id>NULLIF($2,'')::uuid)
		ORDER BY f.anime_id LIMIT 51`, owner.ID, after)
	if err != nil {
		return page, err
	}
	defer rows.Close()
	for rows.Next() {
		var item SavedDiscoveryFeedback
		if err := rows.Scan(&item.AnimeID, &item.Title, &item.Direction); err != nil {
			return page, err
		}
		page.Items = append(page.Items, item)
	}
	if err := rows.Err(); err != nil {
		return page, err
	}
	if len(page.Items) > 50 {
		page.Items = page.Items[:50]
		page.NextCursor = page.Items[49].AnimeID
	}
	return page, tx.Commit(ctx)
}
