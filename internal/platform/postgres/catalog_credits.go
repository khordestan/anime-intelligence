package postgres

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/khordestan/anime-intelligence/internal/platform/store"
)

func (r *Repository) CharacterCredits(ctx context.Context, id string) (store.CharacterCredits, bool, error) {
	out := store.CharacterCredits{Items: []store.CatalogCredit{}}
	var payload []byte
	err := r.db.pool.QueryRow(ctx, `SELECT m.payload->'characterCredits',COALESCE(s.provider_key,''),m.observed_at
		FROM anime a LEFT JOIN catalog_metadata m ON m.anime_id=a.id LEFT JOIN data_sources s ON s.id=m.source_id
		WHERE a.id=`+canonicalAnimeID, id).Scan(&payload, &out.Provider, &out.ObservedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return out, false, nil
	}
	if err != nil {
		return out, false, err
	}
	if len(payload) > 0 {
		if err := json.Unmarshal(payload, &out.Items); err != nil {
			return out, false, err
		}
	}
	if len(out.Items) == 0 {
		out = store.CharacterCredits{Items: []store.CatalogCredit{}}
	}
	return out, true, nil
}
