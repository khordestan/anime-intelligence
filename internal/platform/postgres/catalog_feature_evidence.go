package postgres

import (
	"context"

	"github.com/khordestan/anime-intelligence/internal/platform/store"
)

func (r *Repository) CuratedCatalogFeatures(ctx context.Context, animeID string) ([]store.CuratedCatalogFeature, bool, error) {
	const limit = 20
	rows, err := r.db.pool.Query(ctx, `WITH latest AS (
		SELECT DISTINCT ON(o.feature_id) f.feature_key,f.display_name,o.value,o.confidence,o.observed_at
		FROM anime_feature_observations o JOIN anime_features f ON f.id=o.feature_id
		WHERE o.anime_id=$1 AND `+eligibleRecommendationObservation+`
		ORDER BY o.feature_id,o.observed_at DESC,o.id DESC
	)
	SELECT feature_key,display_name,confidence::float8,observed_at FROM latest
	WHERE value>0 ORDER BY value*confidence DESC,feature_key COLLATE "C" LIMIT $2`, animeID, limit+1)
	if err != nil {
		return nil, false, err
	}
	defer rows.Close()
	var features []store.CuratedCatalogFeature
	for rows.Next() {
		var feature store.CuratedCatalogFeature
		if err := rows.Scan(&feature.Key, &feature.Name, &feature.Confidence, &feature.ObservedAt); err != nil {
			return nil, false, err
		}
		features = append(features, feature)
	}
	if err := rows.Err(); err != nil {
		return nil, false, err
	}
	if len(features) > limit {
		return features[:limit], true, nil
	}
	return features, false, nil
}
