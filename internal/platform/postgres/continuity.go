package postgres

import (
 "context"
 "errors"
 "sort"
 "time"

 "github.com/jackc/pgx/v5"
 "github.com/khordestan/anime-intelligence/internal/platform/store"
)

// CatalogContinuity reads a bounded breadth-first component in one public-only
// snapshot. A frontier query fetches at most remaining capacity plus one ID.
// Retained node pairs have at most 50*49 distinct directed edges, including a
// defensive bound if manually inserted relations violate the importer's DAG.
func(r *Repository)CatalogContinuity(ctx context.Context,id string)(store.Continuity,bool,error){
 out:=store.EmptyContinuity(id)
 ctx,cancel:=context.WithTimeout(ctx,3*time.Second);defer cancel()
 tx,err:=r.db.beginTx(ctx,pgx.TxOptions{IsoLevel:pgx.RepeatableRead,AccessMode:pgx.ReadOnly})
 if err!=nil{return out,false,err};defer rollback(tx)
 if _,err=tx.Exec(ctx,"SET LOCAL statement_timeout='2s'");err!=nil{return out,false,err}
 var root string
 if err=tx.QueryRow(ctx,"SELECT id::text FROM anime WHERE id="+canonicalAnimeID,id).Scan(&root);errors.Is(err,pgx.ErrNoRows){return out,false,nil}else if err!=nil{return out,false,err}
 out.RootID=root
 ids:=[]string{root};frontier:=[]string{root};distance:=map[string]int{root:0};depth:=0
 for len(frontier)>0{
  remaining:=store.ContinuityLimit-len(ids)
  rows,err:=tx.Query(ctx,`SELECT id::text FROM (
   SELECT target_anime_id AS id FROM anime_relations WHERE source_anime_id=ANY($1::text[]::uuid[]) AND relation_type IN ('PREQUEL','SEQUEL')
   UNION
   SELECT source_anime_id AS id FROM anime_relations WHERE target_anime_id=ANY($1::text[]::uuid[]) AND relation_type IN ('PREQUEL','SEQUEL')
  ) candidates WHERE NOT(id=ANY($2::text[]::uuid[])) ORDER BY id LIMIT $3`,frontier,ids,remaining+1)
  if err!=nil{return out,false,err}
  next:=[]string{}
  for rows.Next(){var candidate string;if err=rows.Scan(&candidate);err!=nil{rows.Close();return out,false,err};next=append(next,candidate)}
  rows.Close();if err=rows.Err();err!=nil{return out,false,err}
  if len(next)>remaining{out.Truncated=true;next=next[:remaining]}
  depth++
  for _,candidate:=range next{distance[candidate]=depth;ids=append(ids,candidate)}
  if out.Truncated{break}
  frontier=next
 }
 rows,err:=tx.Query(ctx,`SELECT a.id::text,a.canonical_title,a.media_type,a.status,COALESCE(a.episode_count,0),a.episode_count IS NOT NULL,COALESCE(a.release_year,0),COALESCE(a.season_name,''),COALESCE(s.provider_key,''),m.observed_at
 FROM anime a LEFT JOIN catalog_metadata m ON m.anime_id=a.id LEFT JOIN data_sources s ON s.id=m.source_id WHERE a.id=ANY($1::text[]::uuid[])`,ids)
 if err!=nil{return out,false,err}
 for rows.Next(){
  var n store.ContinuityNode
  if err=rows.Scan(&n.ID,&n.Title,&n.MediaType,&n.Status,&n.EpisodeCount,&n.EpisodeCountKnown,&n.ReleaseYear,&n.Season,&n.Provider,&n.ObservedAt);err!=nil{rows.Close();return out,false,err}
  n.Distance=distance[n.ID];out.Nodes=append(out.Nodes,n)
 }
 rows.Close();if err=rows.Err();err!=nil{return out,false,err}
 sort.Slice(out.Nodes,func(i,j int)bool{if out.Nodes[i].Distance!=out.Nodes[j].Distance{return out.Nodes[i].Distance<out.Nodes[j].Distance};return out.Nodes[i].ID<out.Nodes[j].ID})
 rows,err=tx.Query(ctx,`WITH links AS (
 SELECT CASE WHEN r.relation_type='SEQUEL' THEN r.source_anime_id ELSE r.target_anime_id END AS from_id,
 CASE WHEN r.relation_type='SEQUEL' THEN r.target_anime_id ELSE r.source_anime_id END AS to_id,s.provider_key
 FROM anime_relations r LEFT JOIN data_sources s ON s.id=r.source_id
 WHERE r.relation_type IN ('PREQUEL','SEQUEL') AND r.source_anime_id<>r.target_anime_id AND r.source_anime_id=ANY($1::text[]::uuid[]) AND r.target_anime_id=ANY($1::text[]::uuid[])
 )
 SELECT from_id::text,to_id::text,COALESCE(array_agg(DISTINCT provider_key ORDER BY provider_key) FILTER(WHERE provider_key IS NOT NULL),ARRAY[]::text[])
 FROM links GROUP BY from_id,to_id ORDER BY from_id,to_id`,ids)
 if err!=nil{return out,false,err}
 for rows.Next(){var e store.ContinuityEdge;if err=rows.Scan(&e.FromID,&e.ToID,&e.Providers);err!=nil{rows.Close();return out,false,err};out.Edges=append(out.Edges,e)}
 rows.Close();if err=rows.Err();err!=nil{return out,false,err}
 if err=tx.Commit(ctx);err!=nil{return store.EmptyContinuity(id),false,err}
 return out,true,nil
}
