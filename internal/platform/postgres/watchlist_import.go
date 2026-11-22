package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

var (
	ErrWatchListImport = errors.New("invalid watch list")
	ErrWatchListCatalog = errors.New("watch list does not match the catalog")
)

type WatchListImportResult struct {
	Added int `json:"added"`
	Skipped int `json:"skipped"`
}

const watchListRows = `jsonb_to_recordset($1::jsonb) AS x(
	"animeId" uuid, "watchStatus" text, "episodesWatched" integer,
	"watchedEpisodeNumbers" integer[], rating integer)`

func (r *Repository) ImportWatchList(ctx context.Context, list WatchListExport) (WatchListImportResult, error) {
	if list.Version != 1 || list.Entries == nil || len(list.Entries) > 10000 {
		return WatchListImportResult{}, ErrWatchListImport
	}
	seen := make(map[[16]byte]struct{}, len(list.Entries))
	for _, entry := range list.Entries {
		var id pgtype.UUID
		if err := id.Scan(entry.AnimeID); err != nil || !id.Valid {
			return WatchListImportResult{}, ErrWatchListImport
		}
		if _, duplicate := seen[id.Bytes]; duplicate {
			return WatchListImportResult{}, ErrWatchListImport
		}
		seen[id.Bytes] = struct{}{}
		switch entry.WatchStatus {
		case "PLANNING", "WATCHING", "COMPLETED", "ON_HOLD", "DROPPED":
		default:
			return WatchListImportResult{}, ErrWatchListImport
		}
		if entry.EpisodesWatched < 0 || int64(entry.EpisodesWatched) > 2147483647 ||
			(entry.Rating != nil && (*entry.Rating < 1 || *entry.Rating > 5)) {
			return WatchListImportResult{}, ErrWatchListImport
		}
		numbers := entry.WatchedEpisodeNumbers
		if numbers != nil && (len(numbers) > 10000 || len(numbers) != entry.EpisodesWatched) {
			return WatchListImportResult{}, ErrWatchListImport
		}
		var previous int32
		for _, number := range numbers {
			if number <= previous || number > 100000 {
				return WatchListImportResult{}, ErrWatchListImport
			}
			previous = number
		}
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	tx, owner, err := r.db.productTx(ctx, r.token)
	if err != nil {
		return WatchListImportResult{}, err
	}
	defer rollback(tx)
	payload, err := json.Marshal(list.Entries)
	if err != nil {
		return WatchListImportResult{}, err
	}
	var valid bool
	var added []string
	err = tx.QueryRow(ctx, `WITH input AS MATERIALIZED (
		SELECT x.*, a.id IS NOT NULL AND (a.episode_count IS NULL OR
			COALESCE(x."watchedEpisodeNumbers"[cardinality(x."watchedEpisodeNumbers")], 0) <= a.episode_count) AS valid
		FROM `+watchListRows+` LEFT JOIN anime a ON a.id=x."animeId"
	), validation AS (
		SELECT COALESCE(bool_and(valid), true) AS valid FROM input
	), inserted AS (
		INSERT INTO user_anime(user_id,anime_id,watch_status)
		SELECT $2, "animeId", "watchStatus" FROM input
		WHERE (SELECT valid FROM validation) ORDER BY "animeId"
		ON CONFLICT (user_id,anime_id) DO NOTHING RETURNING anime_id
	)
	SELECT validation.valid, ARRAY(SELECT anime_id::text FROM inserted) FROM validation`, payload, owner.ID).Scan(&valid, &added)
	if err != nil {
		return WatchListImportResult{}, err
	}
	if !valid {
		return WatchListImportResult{}, ErrWatchListCatalog
	}
	if len(added) > 0 {
		if _, err := tx.Exec(ctx, `INSERT INTO watch_progress(user_id,anime_id,episodes_watched,watched_episode_numbers)
			SELECT $2, x."animeId", x."episodesWatched", x."watchedEpisodeNumbers"
			FROM `+watchListRows+` WHERE x."animeId"::text=ANY($3::text[])`, payload, owner.ID, added); err != nil {
			return WatchListImportResult{}, err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO user_ratings(user_id,anime_id,scope,score)
			SELECT $2, x."animeId", 'SERIES', x.rating
			FROM `+watchListRows+` WHERE x."animeId"::text=ANY($3::text[]) AND x.rating IS NOT NULL`, payload, owner.ID, added); err != nil {
			return WatchListImportResult{}, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return WatchListImportResult{}, err
	}
	return WatchListImportResult{Added: len(added), Skipped: len(list.Entries)-len(added)}, nil
}
