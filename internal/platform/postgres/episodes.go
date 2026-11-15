package postgres

import (
	"context"
	"errors"
	"sort"

	"github.com/jackc/pgx/v5"
)

var ErrEpisodes = errors.New("invalid episode selection")
type EpisodeProgress struct {
	AnimeID string `json:"animeId"`
	Mode string `json:"mode"`
	Numbers []int32 `json:"numbers"`
	Count int `json:"count"`
	Total *int `json:"total"`
}
func progressRow(ctx context.Context,tx pgx.Tx,user,id string) (EpisodeProgress,bool,error) {
	var p EpisodeProgress
	var exact bool
	err:=tx.QueryRow(ctx,`SELECT ua.anime_id::text,a.episode_count,COALESCE(wp.episodes_watched,0),wp.watched_episode_numbers,wp.watched_episode_numbers IS NOT NULL
FROM user_anime ua JOIN anime a ON a.id=ua.anime_id
LEFT JOIN watch_progress wp ON wp.user_id=ua.user_id AND wp.anime_id=ua.anime_id
WHERE ua.user_id=$1 AND ua.id=$2 FOR UPDATE OF ua`,user,id).Scan(&p.AnimeID,&p.Total,&p.Count,&p.Numbers,&exact)
	if errors.Is(err,pgx.ErrNoRows) {return p,false,nil}
	if err!=nil {return p,false,err}
	p.Mode="COUNT"
	if exact {p.Mode="EXACT";if p.Numbers==nil {p.Numbers=[]int32{}}}
	return p,true,nil
}
func (r *Repository) EpisodeProgress(ctx context.Context,id string) (EpisodeProgress,bool,error) {
	tx,u,err:=r.db.productTx(ctx,r.token)
	if err!=nil {return EpisodeProgress{},false,err}
	defer rollback(tx)
	p,ok,err:=progressRow(ctx,tx,u.ID,id)
	if err!=nil || !ok {return p,ok,err}
	return p,true,tx.Commit(ctx)
}
// SetEpisodeProgress explicitly replaces aggregate tracking with an exact set.
// It never guesses which episodes an old count represents. An empty set clears
// progress; count-based Add/Update explicitly switch back to aggregate mode.
func (r *Repository) SetEpisodeProgress(ctx context.Context,id string,numbers []int32) (EpisodeProgress,bool,error) {
	if numbers==nil || len(numbers)>10000 {return EpisodeProgress{},false,ErrEpisodes}
	ordered:=append([]int32{},numbers...)
	sort.Slice(ordered,func(i,j int)bool{return ordered[i]<ordered[j]})
	for i,n:=range ordered {
		if n<1 || n>100000 || (i>0 && n==ordered[i-1]) {return EpisodeProgress{},false,ErrEpisodes}
	}
	tx,u,err:=r.db.productTx(ctx,r.token)
	if err!=nil {return EpisodeProgress{},false,err}
	defer rollback(tx)
	p,ok,err:=progressRow(ctx,tx,u.ID,id)
	if err!=nil || !ok {return p,ok,err}
	if p.Total!=nil && len(ordered)>0 && int(ordered[len(ordered)-1])>*p.Total {return p,false,ErrEpisodes}
	_,err=tx.Exec(ctx,`INSERT INTO watch_progress(user_id,anime_id,episodes_watched,watched_episode_numbers) VALUES($1,$2,$3,$4)
ON CONFLICT(user_id,anime_id) DO UPDATE SET episodes_watched=excluded.episodes_watched,watched_episode_numbers=excluded.watched_episode_numbers,updated_at=now()`,u.ID,p.AnimeID,len(ordered),ordered)
	if err!=nil {return p,false,err}
	if _,err=tx.Exec(ctx,`UPDATE user_anime SET updated_at=now() WHERE user_id=$1 AND id=$2`,u.ID,id);err!=nil {return p,false,err}
	p.Mode="EXACT";p.Numbers=ordered;p.Count=len(ordered)
	return p,true,tx.Commit(ctx)
}
