package catalogsync

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
)

func recordCancellation(ctx context.Context, tx pgx.Tx, animeID, provider string, observed time.Time) (int, error) {
	tag, err := tx.Exec(ctx, `INSERT INTO domain_events(event_key,event_type,aggregate_type,aggregate_id,payload,occurred_at)
		VALUES('title-cancelled:'||$1,'TITLE_CANCELLED','ANIME',$1::uuid,jsonb_build_object('provider',$2::text),$3)
		ON CONFLICT(event_key) DO NOTHING`, animeID, provider, observed)
	return int(tag.RowsAffected()), err
}
