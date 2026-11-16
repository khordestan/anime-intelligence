package postgres

import (
 "context"
 "errors"

 "github.com/jackc/pgx/v5"
 "github.com/khordestan/anime-intelligence/internal/platform/store"
)
var _ store.IdentityCatalog=(*Repository)(nil)
func(r *Repository) GetAnimeByExternalID(ctx context.Context,provider,external string)(store.Anime,bool,error){
 var a store.Anime
 err:=r.db.pool.QueryRow(ctx,`SELECT `+animeColumns+` FROM anime WHERE EXISTS(
 SELECT 1 FROM anime_external_ids ext WHERE ext.anime_id=anime.id AND ext.provider=$1 AND ext.external_id=$2)`,provider,external).Scan(&a.ID,&a.Title,&a.NativeTitle,&a.MediaType,&a.ReleaseYear,&a.Status,&a.EpisodeCount,&a.Synopsis,&a.Season,&a.CoverImageURL,&a.EpisodeCountKnown)
 if errors.Is(err,pgx.ErrNoRows){return a,false,nil}
 return a,err==nil,err
}
