package postgres

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"strings"
	"errors"
	"time"

)

var ErrNotificationCursor = errors.New("invalid notification cursor")
var ErrReleaseTrack = errors.New("invalid release track")
var ErrFollowScope = errors.New("invalid follow scope")
var ErrFollowLimit = errors.New("follow limit reached")

type Follow struct {
	AnimeID string `json:"animeId"`
	Title string `json:"title"`
	Track string `json:"track"`
	Scope string `json:"scope"`
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
	rows,err:=tx.Query(ctx,`SELECT f.anime_id::text,a.canonical_title,f.release_track,f.follow_scope,f.created_at FROM user_follows f JOIN anime a ON a.id=f.anime_id WHERE f.user_id=$1 ORDER BY f.created_at DESC,f.id LIMIT 1000`,u.ID)
	if err!=nil {return nil,err}
	out:=[]Follow{}
	for rows.Next() {
		var f Follow
		if err=rows.Scan(&f.AnimeID,&f.Title,&f.Track,&f.Scope,&f.CreatedAt);err!=nil {rows.Close();return nil,err}
		out=append(out,f)
	}
	rows.Close()
	if err=rows.Err();err!=nil {return nil,err}
	return out,tx.Commit(ctx)
}
func (r *Repository) Follow(ctx context.Context,id,track string) (bool,error) {
	return r.FollowScope(ctx,id,track,"ANIME")
}
func (r *Repository) FollowScope(ctx context.Context,id,track,scope string) (bool,error) {
	if scope!="ANIME" && scope!="FRANCHISE" {return false,ErrFollowScope}
	switch track {case "RAW","SUB","DUB","UNKNOWN","ALL":default:return false,ErrReleaseTrack}
	tx,u,err:=r.db.productTx(ctx,r.token)
	if err!=nil {return false,err}
	defer rollback(tx)
	var exists bool
	if err=tx.QueryRow(ctx,`SELECT EXISTS(SELECT 1 FROM anime WHERE id=$1)`,id).Scan(&exists);err!=nil {return false,err}
	if !exists {return false,nil}
	if _,err=tx.Exec(ctx,`SELECT pg_advisory_xact_lock(hashtextextended($1,742019685))`,u.ID);err!=nil {return false,err}
	var already bool
	var count int
	if err=tx.QueryRow(ctx,`SELECT EXISTS(SELECT 1 FROM user_follows WHERE user_id=$1 AND anime_id=$2), (SELECT count(*) FROM user_follows WHERE user_id=$1)`,u.ID,id).Scan(&already,&count);err!=nil {return false,err}
	if !already && count>=1000 {return false,ErrFollowLimit}
	var started time.Time
	if err=tx.QueryRow(ctx,`SELECT COALESCE(min(created_at),now()) FROM user_follows WHERE user_id=$1 AND anime_id=$2`,u.ID,id).Scan(&started);err!=nil {return false,err}
	if _,err=tx.Exec(ctx,`DELETE FROM user_follows WHERE user_id=$1 AND anime_id=$2 AND follow_scope<>$3`,u.ID,id,scope);err!=nil {return false,err}
	_,err=tx.Exec(ctx,`INSERT INTO user_follows(user_id,anime_id,follow_scope,release_track,created_at) VALUES($1,$2,$3,$4,$5)
ON CONFLICT(user_id,anime_id,follow_scope) DO UPDATE SET release_track=excluded.release_track`,u.ID,id,scope,track,started)
	if err!=nil {return false,err}
	return true,tx.Commit(ctx)
}
func (r *Repository) Unfollow(ctx context.Context,id string) (bool,error) {
	return r.UnfollowScope(ctx,id,"ANIME")
}
func (r *Repository) UnfollowScope(ctx context.Context,id,scope string) (bool,error) {
	if scope!="ANIME" && scope!="FRANCHISE" {return false,ErrFollowScope}
	tx,u,err:=r.db.productTx(ctx,r.token)
	if err!=nil {return false,err}
	defer rollback(tx)
	if _,err=tx.Exec(ctx,`SELECT pg_advisory_xact_lock(hashtextextended($1,742019685))`,u.ID);err!=nil {return false,err}
	tag,err:=tx.Exec(ctx,`DELETE FROM user_follows WHERE user_id=$1 AND anime_id=$2 AND follow_scope=$3`,u.ID,id,scope)
	if err!=nil {return false,err}
	return tag.RowsAffected()>0,tx.Commit(ctx)
}

type NotificationPage struct {
	Items []Notification `json:"items"`
	NextCursor string `json:"nextCursor,omitempty"`
}
func notificationCursor(n Notification) string {
	return base64.RawURLEncoding.EncodeToString([]byte(n.CreatedAt.UTC().Format(time.RFC3339Nano)+"|"+n.ID))
}
func parseNotificationCursor(cursor string)(*time.Time,*string,error){
	if cursor==""{return nil,nil,nil}
	if len(cursor)>256{return nil,nil,ErrNotificationCursor}
	b,err:=base64.RawURLEncoding.DecodeString(cursor)
	if err!=nil{return nil,nil,ErrNotificationCursor}
	parts:=strings.Split(string(b),"|")
	if len(parts)!=2{return nil,nil,ErrNotificationCursor}
	stamp,err:=time.Parse(time.RFC3339Nano,parts[0])
	if err!=nil||stamp.IsZero(){return nil,nil,ErrNotificationCursor}
	id:=parts[1]
	if len(id)!=36||id[8]!='-'||id[13]!='-'||id[18]!='-'||id[23]!='-'{return nil,nil,ErrNotificationCursor}
	if _,err=hex.DecodeString(strings.ReplaceAll(id,"-",""));err!=nil{return nil,nil,ErrNotificationCursor}
	return &stamp,&id,nil
}

// Notifications preserves the first-page repository interface.
func (r *Repository) Notifications(ctx context.Context) ([]Notification,error) {
	page,err:=r.NotificationHistory(ctx,"")
	return page.Items,err
}
// NotificationHistory materializes a bounded batch only on the latest page.
// History pages read already materialized notices with a stable descending
// timestamp/ID boundary. Cursor data never determines account ownership.
func (r *Repository) NotificationHistory(ctx context.Context,cursor string) (NotificationPage,error) {
	out:=NotificationPage{Items:[]Notification{}}
	tx,u,err:=r.db.productTx(ctx,r.token)
	if err!=nil {return out,err}
	defer rollback(tx)
	before,beforeID,err:=parseNotificationCursor(cursor)
	if err!=nil{return out,err}
	if _,err=tx.Exec(ctx,`SET LOCAL statement_timeout='2s'`);err!=nil {return out,err}
	if cursor=="" {
	if _,err=tx.Exec(ctx,`SELECT pg_advisory_xact_lock(hashtextextended($1,742019685))`,u.ID);err!=nil {return out,err}
	_,err=tx.Exec(ctx,`WITH RECURSIVE franchise(origin,anime_id,release_track,created_at) AS (
SELECT anime_id,anime_id,release_track,created_at FROM user_follows WHERE user_id=$1 AND follow_scope='FRANCHISE'
UNION
SELECT f.origin,CASE WHEN r.source_anime_id=f.anime_id THEN r.target_anime_id ELSE r.source_anime_id END,f.release_track,f.created_at
FROM franchise f JOIN anime_relations r ON r.relation_type IN ('PREQUEL','SEQUEL')
AND (r.source_anime_id=f.anime_id OR r.target_anime_id=f.anime_id)
), targets(anime_id,release_track,created_at,scope) AS (
SELECT anime_id,release_track,created_at,'ANIME' FROM user_follows WHERE user_id=$1 AND follow_scope='ANIME'
UNION
SELECT anime_id,release_track,created_at,'FRANCHISE' FROM franchise
)
INSERT INTO notifications(user_id,event_id,channel,title,body,created_at)
SELECT DISTINCT $1,e.id,'IN_APP',
CASE WHEN e.event_type='SEASON_ANNOUNCED' THEN 'New season announced' ELSE 'New episode released' END,
CASE WHEN e.event_type='SEASON_ANNOUNCED' THEN a.canonical_title || ' · A new linked season has been announced.'
ELSE a.canonical_title || ' · Episode ' || COALESCE(e.payload->>'number','?') || ' (' || COALESCE(e.payload->>'track','UNKNOWN') || ')' END,
e.occurred_at
FROM domain_events e JOIN targets f ON f.anime_id=e.aggregate_id JOIN anime a ON a.id=e.aggregate_id
WHERE e.aggregate_type='ANIME' AND e.occurred_at>=f.created_at
AND ((e.event_type='EPISODE_RELEASED' AND (f.release_track='ALL' OR f.release_track=e.payload->>'track'))
OR (e.event_type='SEASON_ANNOUNCED' AND f.scope='FRANCHISE'))
AND NOT EXISTS(SELECT 1 FROM notifications n WHERE n.user_id=$1 AND n.event_id=e.id AND n.channel='IN_APP')
ORDER BY e.occurred_at,e.id LIMIT 100
ON CONFLICT(user_id,event_id,channel) WHERE event_id IS NOT NULL DO NOTHING`,u.ID)
	if err!=nil {return out,err}
	}
	rows,err:=tx.Query(ctx,`SELECT id::text,title,body,read_at,created_at FROM notifications
WHERE user_id=$1 AND channel='IN_APP' AND ($2::timestamptz IS NULL OR (created_at,id)<($2::timestamptz,$3::uuid))
ORDER BY created_at DESC,id DESC LIMIT 101`,u.ID,before,beforeID)
	if err!=nil {return out,err}
	for rows.Next() {
		var n Notification
		if err=rows.Scan(&n.ID,&n.Title,&n.Body,&n.ReadAt,&n.CreatedAt);err!=nil {rows.Close();return out,err}
		out.Items=append(out.Items,n)
	}
	rows.Close()
	if err=rows.Err();err!=nil {return out,err}
	if len(out.Items)>100{out.Items=out.Items[:100];out.NextCursor=notificationCursor(out.Items[99])}
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

