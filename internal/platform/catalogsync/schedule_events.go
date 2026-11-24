package catalogsync

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/khordestan/anime-intelligence/internal/modules/catalog/feed"
)

func insertScheduleChangeEvent(ctx context.Context, tx pgx.Tx, observation episodeObservation, episode feed.Episode, previous time.Time, revision int) (int, error) {
	current := episode.ScheduledAt.UTC().Truncate(time.Microsecond)
	direction := "LATER"
	if current.Before(previous) {
		direction = "EARLIER"
	}
	payload, err := json.Marshal(struct {
		Number           int       `json:"number"`
		Track            string    `json:"track"`
		Provider         string    `json:"provider"`
		PreviousSchedule time.Time `json:"previousSchedule"`
		CurrentSchedule  time.Time `json:"currentSchedule"`
		Direction        string    `json:"direction"`
		Revision         int       `json:"revision"`
	}{episode.Number, episode.Track, observation.provider, previous.UTC(), current, direction, revision})
	if err != nil {
		return 0, err
	}
	key := fmt.Sprintf("schedule:%s:%d:%s:%d", observation.animeID, episode.Number, episode.Track, revision)
	tag, err := tx.Exec(ctx, `INSERT INTO domain_events(event_key,event_type,aggregate_type,aggregate_id,payload,occurred_at)
		VALUES($1,'SCHEDULE_CHANGED','ANIME',$2,$3,$4) ON CONFLICT(event_key) DO NOTHING`,
		key, observation.animeID, payload, observation.observed)
	return int(tag.RowsAffected()), err
}
