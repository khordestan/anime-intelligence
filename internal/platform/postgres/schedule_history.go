package postgres

import (
	"context"
	"time"

	"github.com/khordestan/anime-intelligence/internal/platform/store"
)

func (r *Repository) CatalogScheduleHistory(ctx context.Context, id string, number int, track string, before int) (store.ScheduleHistory, error) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	out := store.ScheduleHistory{Items: []store.CatalogEpisode{}}
	rows, err := r.db.pool.Query(ctx, `SELECT a.episode_number,a.release_track,a.scheduled_at,a.actual_released_at,
		a.last_verified_at,a.revision,COALESCE(s.provider_key,'')
		FROM airing_schedule a LEFT JOIN data_sources s ON s.id=a.source_id
		WHERE a.anime_id=$1 AND a.episode_number=$2 AND a.release_track=$3
		  AND ($4::integer=0 OR a.revision<$4)
		ORDER BY a.revision DESC LIMIT $5`, id, number, track, before, store.ScheduleHistoryPageSize+1)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		var episode store.CatalogEpisode
		if err := rows.Scan(&episode.Number, &episode.Track, &episode.ScheduledAt, &episode.ReleasedAt,
			&episode.VerifiedAt, &episode.Revision, &episode.Provider); err != nil {
			return out, err
		}
		out.Items = append(out.Items, episode)
	}
	if err := rows.Err(); err != nil {
		return out, err
	}
	if len(out.Items) > store.ScheduleHistoryPageSize {
		out.Items = out.Items[:store.ScheduleHistoryPageSize]
		out.NextBeforeRevision = out.Items[len(out.Items)-1].Revision
	}
	return out, nil
}
