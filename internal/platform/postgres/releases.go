package postgres

import (
	"context"
	"errors"
	"time"

)

var ErrReleaseTrack = errors.New("invalid release track")

type Follow struct {
	AnimeID string `json:"animeId"`
	Title string `json:"title"`
	Track string `json:"track"`
	CreatedAt time.Time `json:"createdAt"`
}
type Notification struct {
	ID string `json:"id"`
	Title string `json:"title"`
	Body string `json:"body"`
	ReadAt *time.Time `json:"readAt,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
}
func (r *Repository) Follows(ctx context.Context) ([]Follow,error) {
	tx,u,err:=r.db.productTx(ctx,r.token)
	if err!=nil {return nil,err}
	defer rollback(tx)
	rows,err:=tx.Query(ctx,`SELECT f.anime_id::text,a.canonical_title,f.release_track,f.created_at FROM user_follows f JOIN anime a ON a.id=f.anime_id WHERE f.user_id=$1 AND f.follow_scope='ANIME' ORDER BY f.created_at DESC,f.id LIMIT 1000`,u.ID)
	if err!=nil {return nil,err}
	out:=[]Follow{}
	for rows.Next() {
		var f Follow
		if err=rows.Scan(&f.AnimeID,&f.Title,&f.Track,&f.CreatedAt);err!=nil {rows.Close();return nil,err}
		out=append(out,f)
	}
	rows.Close()
	if err=rows.Err();err!=nil {return nil,err}
	return out,tx.Commit(ctx)
}
func (r *Repository) Follow(ctx context.Context,id,track string) (bool,error) {
	switch track {case "RAW","SUB","DUB","UNKNOWN","ALL":default:return false,ErrReleaseTrack}
	tx,u,err:=r.db.productTx(ctx,r.token)
	if err!=nil {return false,err}
	defer rollback(tx)
	var exists bool
	if err=tx.QueryRow(ctx,`SELECT EXISTS(SELECT 1 FROM anime WHERE id=$1)`,id).Scan(&exists);err!=nil {return false,err}
	if !exists {return false,nil}
	_,err=tx.Exec(ctx,`INSERT INTO user_follows(user_id,anime_id,follow_scope,release_track) VALUES($1,$2,'ANIME',$3)
ON CONFLICT(user_id,anime_id,follow_scope) DO UPDATE SET release_track=excluded.release_track`,u.ID,id,track)
	if err!=nil {return false,err}
	return true,tx.Commit(ctx)
}
func (r *Repository) Unfollow(ctx context.Context,id string) (bool,error) {
	tx,u,err:=r.db.productTx(ctx,r.token)
	if err!=nil {return false,err}
	defer rollback(tx)
	tag,err:=tx.Exec(ctx,`DELETE FROM user_follows WHERE user_id=$1 AND anime_id=$2 AND follow_scope='ANIME'`,u.ID,id)
	if err!=nil {return false,err}
	return tag.RowsAffected()>0,tx.Commit(ctx)
}

// Notifications materializes only this verified account's eligible events.
// The bounded poll transaction requires no cross-user RLS bypass or worker
// identity. Unique event/channel keys make overlapping sessions safe.
func (r *Repository) Notifications(ctx context.Context) ([]Notification,error) {
	tx,u,err:=r.db.productTx(ctx,r.token)
	if err!=nil {return nil,err}
	defer rollback(tx)
	_,err=tx.Exec(ctx,`INSERT INTO notifications(user_id,event_id,channel,title,body)
SELECT $1,e.id,'IN_APP','New episode released',
a.canonical_title || ' · Episode ' || COALESCE(e.payload->>'number','?') || ' (' || COALESCE(e.payload->>'track','UNKNOWN') || ')'
FROM domain_events e
JOIN user_follows f ON f.anime_id=e.aggregate_id AND f.user_id=$1 AND f.follow_scope='ANIME'
JOIN anime a ON a.id=e.aggregate_id
WHERE e.event_type='EPISODE_RELEASED' AND e.aggregate_type='ANIME'
AND e.occurred_at>=f.created_at
AND (f.release_track='ALL' OR f.release_track=e.payload->>'track')
AND NOT EXISTS(SELECT 1 FROM notifications n WHERE n.user_id=$1 AND n.event_id=e.id AND n.channel='IN_APP')
ORDER BY e.occurred_at,e.id LIMIT 100
ON CONFLICT(user_id,event_id,channel) WHERE event_id IS NOT NULL DO NOTHING`,u.ID)
	if err!=nil {return nil,err}
	rows,err:=tx.Query(ctx,`SELECT id::text,title,body,read_at,created_at FROM notifications WHERE user_id=$1 AND channel='IN_APP' ORDER BY created_at DESC,id DESC LIMIT 100`,u.ID)
	if err!=nil {return nil,err}
	out:=[]Notification{}
	for rows.Next() {
		var n Notification
		if err=rows.Scan(&n.ID,&n.Title,&n.Body,&n.ReadAt,&n.CreatedAt);err!=nil {rows.Close();return nil,err}
		out=append(out,n)
	}
	rows.Close()
	if err=rows.Err();err!=nil {return nil,err}
	return out,tx.Commit(ctx)
}
func (r *Repository) ReadNotification(ctx context.Context,id string) (bool,error) {
	tx,u,err:=r.db.productTx(ctx,r.token)
	if err!=nil {return false,err}
	defer rollback(tx)
	tag,err:=tx.Exec(ctx,`UPDATE notifications SET read_at=COALESCE(read_at,now()) WHERE user_id=$1 AND id=$2 AND channel='IN_APP'`,u.ID,id)
	if err!=nil {return false,err}
	return tag.RowsAffected()>0,tx.Commit(ctx)
}

