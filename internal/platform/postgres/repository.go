package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/khordestan/anime-intelligence/internal/platform/store"
)

type Repository struct {
	db    *DB
	token string
}

var _ store.Repository = (*Repository)(nil)

const animeColumns = `id::text,canonical_title,COALESCE(native_title,''),media_type,COALESCE(release_year,0),status,COALESCE(episode_count,0),COALESCE(synopsis,''),COALESCE(season_name,''),COALESCE(cover_image_url,''),episode_count IS NOT NULL`

func (r *Repository) Catalog(ctx context.Context, q string, limit, offset int) ([]store.Anime, error) {
	rows, err := r.db.pool.Query(ctx, `SELECT `+animeColumns+` FROM anime WHERE $1='' OR strpos(lower(canonical_title),lower($1))>0 OR strpos(lower(COALESCE(native_title,'')),lower($1))>0 ORDER BY canonical_title,id LIMIT $2 OFFSET $3`, q, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []store.Anime{}
	for rows.Next() {
		var a store.Anime
		if err = rows.Scan(&a.ID, &a.Title, &a.NativeTitle, &a.MediaType, &a.ReleaseYear, &a.Status, &a.EpisodeCount, &a.Synopsis, &a.Season, &a.CoverImageURL, &a.EpisodeCountKnown); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}
func (r *Repository) GetAnime(ctx context.Context, id string) (store.Anime, bool, error) {
	var a store.Anime
	err := r.db.pool.QueryRow(ctx, `SELECT `+animeColumns+` FROM anime WHERE id=$1`, id).Scan(&a.ID, &a.Title, &a.NativeTitle, &a.MediaType, &a.ReleaseYear, &a.Status, &a.EpisodeCount, &a.Synopsis, &a.Season, &a.CoverImageURL, &a.EpisodeCountKnown)
	if errors.Is(err, pgx.ErrNoRows) {
		return a, false, nil
	}
	return a, err == nil, err
}

const entrySelect = `SELECT ua.id::text,ua.anime_id::text,a.canonical_title,ua.watch_status,COALESCE(wp.episodes_watched,0),COALESCE(a.episode_count,0),COALESCE(ur.score,0),ua.updated_at,wp.watched_episode_numbers IS NOT NULL
 FROM user_anime ua JOIN anime a ON a.id=ua.anime_id
 LEFT JOIN watch_progress wp ON wp.user_id=ua.user_id AND wp.anime_id=ua.anime_id
 LEFT JOIN user_ratings ur ON ur.user_id=ua.user_id AND ur.anime_id=ua.anime_id AND ur.scope='SERIES' AND ur.season_key IS NULL`

func scanEntry(row pgx.Row) (store.Entry, error) {
	var e store.Entry
	err := row.Scan(&e.ID, &e.AnimeID, &e.Title, &e.WatchStatus, &e.EpisodesWatched, &e.TotalEpisodes, &e.Rating, &e.UpdatedAt, &e.ExactProgress)
	return e, err
}
func (r *Repository) Library(ctx context.Context) ([]store.Entry, error) {
	tx, u, err := r.db.productTx(ctx, r.token)
	if err != nil {
		return nil, err
	}
	defer rollback(tx)
	rows, err := tx.Query(ctx, entrySelect+` WHERE ua.user_id=$1 ORDER BY ua.updated_at DESC,ua.id`, u.ID)
	if err != nil {
		return nil, err
	}
	out := []store.Entry{}
	for rows.Next() {
		e, err := scanEntry(rows)
		if err != nil {
			rows.Close()
			return nil, err
		}
		out = append(out, e)
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return nil, err
	}
	return out, tx.Commit(ctx)
}
func (r *Repository) Add(ctx context.Context, animeID, status string, episodes int) (store.Entry, error) {
	tx, u, err := r.db.productTx(ctx, r.token)
	if err != nil {
		return store.Entry{}, err
	}
	defer rollback(tx)
	var exists bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM anime WHERE id=$1)`, animeID).Scan(&exists); err != nil {
		return store.Entry{}, err
	}
	if !exists {
		return store.Entry{}, errors.New("anime_not_found")
	}
	_, err = tx.Exec(ctx, `INSERT INTO user_anime(user_id,anime_id,watch_status) VALUES($1,$2,$3) ON CONFLICT(user_id,anime_id) DO UPDATE SET watch_status=excluded.watch_status,updated_at=now()`, u.ID, animeID, status)
	if err != nil {
		return store.Entry{}, err
	}
	_, err = tx.Exec(ctx, `INSERT INTO watch_progress(user_id,anime_id,episodes_watched) VALUES($1,$2,$3) ON CONFLICT(user_id,anime_id) DO UPDATE SET episodes_watched=excluded.episodes_watched,watched_episode_numbers=NULL,updated_at=now()`, u.ID, animeID, episodes)
	if err != nil {
		return store.Entry{}, err
	}
	e, err := scanEntry(tx.QueryRow(ctx, entrySelect+` WHERE ua.user_id=$1 AND ua.anime_id=$2`, u.ID, animeID))
	if err != nil {
		return e, err
	}
	return e, tx.Commit(ctx)
}
func (r *Repository) Update(ctx context.Context, id string, status *string, episodes *int) (store.Entry, bool, error) {
	tx, u, err := r.db.productTx(ctx, r.token)
	if err != nil {
		return store.Entry{}, false, err
	}
	defer rollback(tx)
	var animeID string
	err = tx.QueryRow(ctx, `UPDATE user_anime SET watch_status=COALESCE($3,watch_status),updated_at=now() WHERE id=$1 AND user_id=$2 RETURNING anime_id::text`, id, u.ID, status).Scan(&animeID)
	if errors.Is(err, pgx.ErrNoRows) {
		return store.Entry{}, false, nil
	}
	if err != nil {
		return store.Entry{}, false, err
	}
	if episodes != nil {
		_, err = tx.Exec(ctx, `INSERT INTO watch_progress(user_id,anime_id,episodes_watched) VALUES($1,$2,$3) ON CONFLICT(user_id,anime_id) DO UPDATE SET episodes_watched=excluded.episodes_watched,watched_episode_numbers=NULL,updated_at=now()`, u.ID, animeID, *episodes)
		if err != nil {
			return store.Entry{}, false, err
		}
	}
	e, err := scanEntry(tx.QueryRow(ctx, entrySelect+` WHERE ua.id=$1 AND ua.user_id=$2`, id, u.ID))
	if err != nil {
		return e, false, err
	}
	return e, true, tx.Commit(ctx)
}
func (r *Repository) Rate(ctx context.Context, animeID string, score int) (store.Entry, bool, error) {
	tx, u, err := r.db.productTx(ctx, r.token)
	if err != nil {
		return store.Entry{}, false, err
	}
	defer rollback(tx)
	var id string
	err = tx.QueryRow(ctx, `UPDATE user_anime SET updated_at=now() WHERE anime_id=$1 AND user_id=$2 RETURNING id::text`, animeID, u.ID).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return store.Entry{}, false, nil
	}
	if err != nil {
		return store.Entry{}, false, err
	}
	_, err = tx.Exec(ctx, `INSERT INTO user_ratings(user_id,anime_id,scope,score) VALUES($1,$2,'SERIES',$3) ON CONFLICT(user_id,anime_id) WHERE scope='SERIES' AND season_key IS NULL DO UPDATE SET score=excluded.score,rated_at=now()`, u.ID, animeID, score)
	if err != nil {
		return store.Entry{}, false, err
	}
	e, err := scanEntry(tx.QueryRow(ctx, entrySelect+` WHERE ua.id=$1 AND ua.user_id=$2`, id, u.ID))
	if err != nil {
		return e, false, err
	}
	return e, true, tx.Commit(ctx)
}

// Remove locks the owned tracker row before associated progress/ratings, matching
// Update/Rate ordering. The transaction rolls back every deletion on failure.
func (r *Repository) Remove(ctx context.Context, id string) (bool, error) {
	tx, u, err := r.db.productTx(ctx, r.token)
	if err != nil {
		return false, err
	}
	defer rollback(tx)
	var animeID string
	err = tx.QueryRow(ctx, `DELETE FROM user_anime WHERE id=$1 AND user_id=$2 RETURNING anime_id::text`, id, u.ID).Scan(&animeID)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	for _, table := range []string{"watch_progress", "user_ratings"} {
		if _, err = tx.Exec(ctx, `DELETE FROM `+pgx.Identifier{table}.Sanitize()+` WHERE user_id=$1 AND anime_id=$2`, u.ID, animeID); err != nil {
			return false, err
		}
	}
	return true, tx.Commit(ctx)
}
