package postgres

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/khordestan/anime-intelligence/internal/platform/store"
)

func (r *Repository) FilterCatalog(ctx context.Context, f store.CatalogFilter, limit, offset int) ([]store.Anime, error) {
	if limit < 1 || limit > 100 || offset < 0 || offset > 100000 {
		return nil, store.ErrCatalogFilter
	}
	return r.filterCatalog(ctx, f, limit, offset, nil)
}

func (r *Repository) CatalogAfter(ctx context.Context, f store.CatalogFilter, limit int, after *store.CatalogPosition) ([]store.Anime, error) {
	if limit < 1 || limit > 101 {
		return nil, store.ErrCatalogFilter
	}
	return r.filterCatalog(ctx, f, limit, 0, after)
}

func (r *Repository) filterCatalog(ctx context.Context, f store.CatalogFilter, limit, offset int, after *store.CatalogPosition) ([]store.Anime, error) {
	f, err := f.Normalize()
	if err != nil {
		return nil, err
	}
	var yearKey any
	var title, id string
	if after != nil {
		yearKey, title, id = after.YearKey, after.Title, after.ID
	}
	const sortYear = `CASE $14::text WHEN 'NEWEST' THEN COALESCE(-release_year::bigint, 9223372036854775807)
 WHEN 'OLDEST' THEN COALESCE(release_year::bigint, 9223372036854775807) ELSE 0 END`
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	tx, err := r.db.beginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, err
	}
	defer rollback(tx)
	if _, err = tx.Exec(ctx, "SET LOCAL statement_timeout='2s'"); err != nil {
		return nil, err
	}
	rows, err := tx.Query(ctx, `SELECT `+animeColumns+` FROM anime WHERE
 ($1='' OR strpos(lower(canonical_title),lower($1))>0 OR strpos(lower(COALESCE(native_title,'')),lower($1))>0
 OR EXISTS(SELECT 1 FROM catalog_metadata m WHERE m.anime_id=anime.id AND EXISTS(SELECT 1 FROM jsonb_array_elements_text(COALESCE(m.payload->'synonyms','[]'::jsonb)) alias(value) WHERE strpos(lower(value),lower($1))>0)))
 AND ($4='' OR media_type=$4) AND ($5='' OR status=$5)
 AND ($6::int=0 OR release_year=$6) AND ($7='' OR season_name=$7)
 AND (($8='' AND $9='' AND $10='' AND $11='') OR EXISTS(
 SELECT 1 FROM catalog_metadata m WHERE m.anime_id=anime.id
 AND ($8='' OR EXISTS(SELECT 1 FROM jsonb_array_elements_text(COALESCE(m.payload->'genres','[]'::jsonb)) label(value) WHERE lower(btrim(value))=lower($8)))
 AND ($9='' OR EXISTS(SELECT 1 FROM jsonb_array_elements_text(COALESCE(m.payload->'themes','[]'::jsonb)) label(value) WHERE lower(btrim(value))=lower($9)))
 AND ($10='' OR EXISTS(SELECT 1 FROM jsonb_array_elements_text(COALESCE(m.payload->'demographics','[]'::jsonb)) label(value) WHERE lower(btrim(value))=lower($10)))
 AND ($11='' OR EXISTS(SELECT 1 FROM jsonb_array_elements_text(COALESCE(m.payload->'studios','[]'::jsonb)) label(value) WHERE lower(btrim(value))=lower($11)))
 ))
 AND ($12::int IS NULL OR episode_count >= $12) AND ($13::int IS NULL OR episode_count <= $13)
 AND ($15::int IS NULL OR release_year >= $15) AND ($16::int IS NULL OR release_year <= $16)
 AND ((NOT $20::boolean AND $22::text='') OR EXISTS (
   SELECT 1 FROM airing_schedule confirmed
   WHERE confirmed.anime_id=anime.id AND confirmed.actual_released_at<=statement_timestamp()
     AND ($22::text='' OR confirmed.release_track=$22)
     AND NOT EXISTS (
       SELECT 1 FROM airing_schedule newer WHERE newer.anime_id=confirmed.anime_id
       AND newer.episode_number=confirmed.episode_number AND newer.release_track=confirmed.release_track
       AND newer.revision>confirmed.revision
     )
 ))
 AND ($21='' OR EXISTS (
   SELECT 1 FROM catalog_metadata m JOIN data_sources d ON d.id=m.source_id
   WHERE m.anime_id=anime.id AND d.provider_key=$21
 ))
 AND ($17::bigint IS NULL OR ((`+sortYear+`),canonical_title,id) > ($17,$18,NULLIF($19,'')::uuid))
 ORDER BY (`+sortYear+`),canonical_title,id LIMIT $2 OFFSET $3`, f.Query, limit, offset, f.MediaType, f.Status, f.ReleaseYear, f.Season, f.Genre, f.Theme, f.Demographic, f.Studio, f.MinEpisodes, f.MaxEpisodes, f.Sort, f.MinYear, f.MaxYear, yearKey, title, id, f.ConfirmedRelease, f.MetadataProvider, f.ConfirmedTrack)
	if err != nil {
		return nil, err
	}
	out := []store.Anime{}
	for rows.Next() {
		var a store.Anime
		if err = rows.Scan(&a.ID, &a.Title, &a.NativeTitle, &a.MediaType, &a.ReleaseYear, &a.Status, &a.EpisodeCount, &a.Synopsis, &a.Season, &a.CoverImageURL, &a.EpisodeCountKnown); err != nil {
			rows.Close()
			return nil, err
		}
		out = append(out, a)
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return out, nil
}
