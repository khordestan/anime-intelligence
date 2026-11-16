package postgres

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/khordestan/anime-intelligence/internal/platform/store"
)

func (r *Repository) CatalogDetails(ctx context.Context,id string) (store.CatalogDetails,bool,error) {
	out:=store.EmptyCatalogDetails(store.Anime{})
	tx,err:=r.db.beginTx(ctx,pgx.TxOptions{IsoLevel:pgx.RepeatableRead,AccessMode:pgx.ReadOnly})
	if err!=nil {return out,false,err}
	defer rollback(tx)
	a:=&out.Anime
	err=tx.QueryRow(ctx,`SELECT `+animeColumns+` FROM anime WHERE id=$1`,id).Scan(&a.ID,&a.Title,&a.NativeTitle,&a.MediaType,&a.ReleaseYear,&a.Status,&a.EpisodeCount,&a.Synopsis,&a.Season,&a.CoverImageURL,&a.EpisodeCountKnown)
	if errors.Is(err,pgx.ErrNoRows) {return out,false,nil}
	if err!=nil {return out,false,err}
	var payload []byte
	err=tx.QueryRow(ctx,`SELECT s.provider_key,m.observed_at,m.payload FROM catalog_metadata m JOIN data_sources s ON s.id=m.source_id WHERE m.anime_id=$1`,id).Scan(&out.Provider,&out.ObservedAt,&payload)
	if err!=nil && !errors.Is(err,pgx.ErrNoRows) {return out,false,err}
	if len(payload)>0 {
		var metadata struct {
			Synonyms []string `json:"synonyms"`
			Genres []string `json:"genres"`
			Tags []string `json:"tags"`
			Studios []string `json:"studios"`
		}
		if err=json.Unmarshal(payload,&metadata);err!=nil {return out,false,err}
		if metadata.Synonyms!=nil {out.Synonyms=metadata.Synonyms}
		if metadata.Genres!=nil {out.Genres=metadata.Genres}
		if metadata.Tags!=nil {out.Tags=metadata.Tags}
		if metadata.Studios!=nil {out.Studios=metadata.Studios}
	}
	rows,err:=tx.Query(ctx,`WITH linked(anime_id,relation_type) AS (
SELECT target_anime_id,relation_type FROM anime_relations WHERE source_anime_id=$1
UNION
SELECT source_anime_id,CASE relation_type WHEN 'PREQUEL' THEN 'SEQUEL' ELSE 'PREQUEL' END
FROM anime_relations WHERE target_anime_id=$1 AND relation_type IN ('PREQUEL','SEQUEL')
)
SELECT a.id::text,l.relation_type,a.canonical_title,a.media_type,a.status
FROM linked l JOIN anime a ON a.id=l.anime_id ORDER BY l.relation_type,a.canonical_title,a.id`,id)
	if err!=nil {return out,false,err}
	for rows.Next() {
		var relation store.CatalogRelation
		if err=rows.Scan(&relation.AnimeID,&relation.Type,&relation.Title,&relation.MediaType,&relation.Status);err!=nil {rows.Close();return out,false,err}
		out.Relations=append(out.Relations,relation)
	}
	rows.Close()
	if err=rows.Err();err!=nil {return out,false,err}
	rows,err=tx.Query(ctx,`SELECT DISTINCT ON (episode_number,release_track) episode_number,release_track,scheduled_at,actual_released_at,last_verified_at,revision FROM airing_schedule WHERE anime_id=$1 ORDER BY episode_number,release_track,revision DESC`,id)
	if err!=nil {return out,false,err}
	for rows.Next() {
		var episode store.CatalogEpisode
		if err=rows.Scan(&episode.Number,&episode.Track,&episode.ScheduledAt,&episode.ReleasedAt,&episode.VerifiedAt,&episode.Revision);err!=nil {rows.Close();return out,false,err}
		out.Episodes=append(out.Episodes,episode)
	}
	rows.Close()
	if err=rows.Err();err!=nil {return out,false,err}
	return out,true,tx.Commit(ctx)
}
