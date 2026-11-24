package catalogsync

import (
 "context"
 "errors"
 "time"

 "github.com/jackc/pgx/v5"
 "github.com/khordestan/anime-intelligence/internal/modules/catalog/feed"
)

// applySelectedSchedules uses only existing exact provider mappings. A first
// scoped observation baselines historical confirmations independently for each
// title. Missing fields never claim metadata authority or retract confirmations.
func applySelectedSchedules(ctx context.Context,tx pgx.Tx,s feed.Snapshot,source string)(Result,error){
 out:=Result{SchedulesOnly:true}
 for _,title:=range s.Titles{
  var id string
  err:=tx.QueryRow(ctx,`SELECT anime_id::text FROM anime_external_ids
 WHERE provider=$1 AND external_id=$2 FOR UPDATE`,s.Provider,title.ExternalID).Scan(&id)
  if errors.Is(err,pgx.ErrNoRows){return Result{},ErrConflict}
  if err!=nil{return Result{},err}
  var total *int
  var mediaType string
  if err=tx.QueryRow(ctx,`SELECT episode_count,media_type FROM anime WHERE id=$1 FOR UPDATE`,id).Scan(&total,&mediaType);err!=nil{return Result{},err}
  var previous time.Time
  err=tx.QueryRow(ctx,`SELECT observed_at FROM catalog_schedule_observations
 WHERE source_id=$1 AND anime_id=$2 FOR UPDATE`,source,id).Scan(&previous)
  known:=err==nil
  if err!=nil&&!errors.Is(err,pgx.ErrNoRows){return Result{},err}
  if known&&!s.ObservedAt.After(previous){return Result{},ErrStale}
  for _,episode:=range title.Episodes{
   // A conflicting season match must not stretch an authoritative known total.
   if total!=nil&&episode.Number>*total{return Result{},ErrConflict}
   revisions,events,err:=syncEpisode(ctx,tx,source,id,mediaType,episode,s.ObservedAt,previous,known)
   if err!=nil{return Result{},err}
   out.ScheduleRevisions+=revisions;out.ReleaseEvents+=events
  }
  if _,err=tx.Exec(ctx,`INSERT INTO catalog_schedule_observations(source_id,anime_id,observed_at)
 VALUES($1,$2,$3) ON CONFLICT(source_id,anime_id) DO UPDATE SET observed_at=excluded.observed_at`,source,id,s.ObservedAt);err!=nil{return Result{},err}
  out.Titles++
 }
 return out,nil
}
