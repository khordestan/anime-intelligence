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

// recordCancellationCorrection links the correction to the published event.
// A historical or unpublished cancellation has no event to correct.
func recordCancellationCorrection(ctx context.Context, tx pgx.Tx, animeID, provider, status string, observed time.Time) (int, error) {
	tag, err := tx.Exec(ctx, `INSERT INTO domain_events(event_key,event_type,aggregate_type,aggregate_id,payload,occurred_at)
		SELECT 'title-cancellation-corrected:'||original.id,'TITLE_CANCELLATION_CORRECTED','ANIME',original.aggregate_id,
		       jsonb_build_object('provider',$2::text,'status',$3::text,'cancellationEventId',original.id),$4
		FROM domain_events original
		WHERE original.event_key='title-cancelled:'||$1 AND original.event_type='TITLE_CANCELLED'
		  AND original.aggregate_id=$1::uuid AND original.payload->>'provider'=$2
		  AND original.occurred_at < $4
		ON CONFLICT(event_key) DO NOTHING`, animeID, provider, status, observed)
	return int(tag.RowsAffected()), err
}
