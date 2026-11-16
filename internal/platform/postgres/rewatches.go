package postgres

import (
 "context"
 "errors"
 "time"

 "github.com/jackc/pgx/v5"
 "github.com/khordestan/anime-intelligence/internal/platform/store"
)

var ErrRewatchRequest=errors.New("invalid rewatch request")
var ErrRewatchState=errors.New("rewatch state changed or not completed")
type WatchCycle struct {
 Cycle int `json:"cycle"`
 Count int `json:"count"`
 Total *int `json:"total"`
 Numbers []int32 `json:"numbers"`
 Mode string `json:"mode"`
 Rating *int `json:"rating"`
 ArchivedAt time.Time `json:"archivedAt"`
}
type WatchHistory struct {
 CurrentCycle int `json:"currentCycle"`
 Items []WatchCycle `json:"items"`
 NextBefore int `json:"nextBefore"`
}
// StartRewatch archives only an explicitly completed current pass. The expected
// cycle makes a replay fail closed rather than resetting a second pass.
// Lock order remains account -> session -> tracker -> progress/ratings/history.
func(r *Repository) StartRewatch(ctx context.Context,id string,expected int)(store.Entry,bool,error){
 if expected<1||expected>1000{return store.Entry{},false,ErrRewatchRequest}
 tx,u,err:=r.db.productTx(ctx,r.token)
 if err!=nil{return store.Entry{},false,err}
 defer rollback(tx)
 var anime,status string
 var cycle int
 err=tx.QueryRow(ctx,`SELECT anime_id::text,watch_status,watch_cycle FROM user_anime WHERE user_id=$1 AND id=$2 FOR UPDATE`,u.ID,id).Scan(&anime,&status,&cycle)
 if errors.Is(err,pgx.ErrNoRows){return store.Entry{},false,nil}
 if err!=nil{return store.Entry{},false,err}
 if status!="COMPLETED"||cycle!=expected||cycle>=1000{return store.Entry{},true,ErrRewatchState}
 _,err=tx.Exec(ctx,`INSERT INTO watch_cycles(user_id,anime_id,cycle_number,episodes_watched,total_episodes,watched_episode_numbers,rating)
 SELECT $1,a.id,$3,COALESCE(wp.episodes_watched,0),a.episode_count,wp.watched_episode_numbers,ur.score
 FROM anime a LEFT JOIN watch_progress wp ON wp.anime_id=a.id AND wp.user_id=$1
 LEFT JOIN user_ratings ur ON ur.anime_id=a.id AND ur.user_id=$1 AND ur.scope='SERIES' AND ur.season_key IS NULL
 WHERE a.id=$2`,u.ID,anime,cycle)
 if err!=nil{return store.Entry{},false,err}
 _,err=tx.Exec(ctx,`INSERT INTO watch_progress(user_id,anime_id,episodes_watched) VALUES($1,$2,0)
 ON CONFLICT(user_id,anime_id) DO UPDATE SET episodes_watched=0,watched_episode_numbers=NULL,total_episodes=NULL,updated_at=now()`,u.ID,anime)
 if err!=nil{return store.Entry{},false,err}
 if _,err=tx.Exec(ctx,`UPDATE user_anime SET watch_cycle=watch_cycle+1,watch_status='WATCHING',started_at=CURRENT_DATE,completed_at=NULL,updated_at=now() WHERE user_id=$1 AND id=$2`,u.ID,id);err!=nil{return store.Entry{},false,err}
 entry,err:=scanEntry(tx.QueryRow(ctx,entrySelect+` WHERE ua.user_id=$1 AND ua.id=$2`,u.ID,id))
 if err!=nil{return store.Entry{},false,err}
 return entry,true,tx.Commit(ctx)
}
// History pages expose only completed-pass snapshots, never another account's
// records. Later catalog/rating changes do not rewrite an archived snapshot.
func(r *Repository) WatchHistory(ctx context.Context,id string,before int)(WatchHistory,bool,error){
 out:=WatchHistory{Items:[]WatchCycle{}}
 if before<0||before>1000{return out,false,ErrRewatchRequest}
 tx,u,err:=r.db.productTx(ctx,r.token)
 if err!=nil{return out,false,err}
 defer rollback(tx)
 var anime string
 err=tx.QueryRow(ctx,`SELECT anime_id::text,watch_cycle FROM user_anime WHERE user_id=$1 AND id=$2 FOR UPDATE`,u.ID,id).Scan(&anime,&out.CurrentCycle)
 if errors.Is(err,pgx.ErrNoRows){return out,false,nil}
 if err!=nil{return out,false,err}
 rows,err:=tx.Query(ctx,`SELECT cycle_number,episodes_watched,total_episodes,watched_episode_numbers,rating,archived_at FROM watch_cycles
 WHERE user_id=$1 AND anime_id=$2 AND ($3::integer=0 OR cycle_number<$3) ORDER BY cycle_number DESC LIMIT 101`,u.ID,anime,before)
 if err!=nil{return out,false,err}
 for rows.Next(){
  var c WatchCycle
  if err=rows.Scan(&c.Cycle,&c.Count,&c.Total,&c.Numbers,&c.Rating,&c.ArchivedAt);err!=nil{rows.Close();return out,false,err}
  c.Mode="COUNT";if c.Numbers!=nil{c.Mode="EXACT"}
  out.Items=append(out.Items,c)
 }
 rows.Close();if err=rows.Err();err!=nil{return out,false,err}
 if len(out.Items)>100{out.Items=out.Items[:100];out.NextBefore=out.Items[99].Cycle}
 return out,true,tx.Commit(ctx)
}
