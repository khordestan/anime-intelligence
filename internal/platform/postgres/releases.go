package postgres

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

var ErrNotificationCursor = errors.New("invalid notification cursor")
var ErrReleaseTrack = errors.New("invalid release track")
var ErrFollowScope = errors.New("invalid follow scope")
var ErrFollowCursor = errors.New("invalid follow cursor")
var ErrFollowPageSize = errors.New("invalid follow page size")
var ErrFollowLimit = errors.New("follow limit reached")

type Follow struct {
	AnimeID string `json:"animeId"`
	Title string `json:"title"`
	Track string `json:"track"`
	Scope string `json:"scope"`
 ScheduleNotices bool `json:"scheduleNotices"`
	CreatedAt time.Time `json:"createdAt"`
}
type Notification struct {
	ID string `json:"id"`
	Title string `json:"title"`
	Body string `json:"body"`
	Provider string `json:"provider,omitempty"`
 ReadAt *time.Time `json:"readAt,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
}
type FollowPage struct {
	Items      []Follow
	NextCursor string
}

const releaseTargetsSQL = `WITH RECURSIVE franchise(anime_id,release_track,created_at,schedule_notices_since) AS (
SELECT anime_id,release_track,created_at,schedule_notices_since FROM user_follows WHERE user_id=$1 AND follow_scope='FRANCHISE'
UNION
SELECT CASE WHEN r.source_anime_id=f.anime_id THEN r.target_anime_id ELSE r.source_anime_id END,f.release_track,f.created_at,f.schedule_notices_since
FROM franchise f JOIN anime_relations r ON r.relation_type IN ('PREQUEL','SEQUEL')
AND (r.source_anime_id=f.anime_id OR r.target_anime_id=f.anime_id)
), reachable(anime_id,release_track,created_at,schedule_notices_since,scope) AS (
SELECT anime_id,release_track,created_at,schedule_notices_since,'ANIME' FROM user_follows WHERE user_id=$1 AND follow_scope='ANIME'
UNION
SELECT anime_id,release_track,created_at,schedule_notices_since,'FRANCHISE' FROM franchise
), targets AS (
SELECT anime_id,release_track,min(created_at) AS created_at,min(schedule_notices_since) AS schedule_notices_since,scope
FROM reachable GROUP BY anime_id,release_track,scope
)`

func (r *Repository) Follows(ctx context.Context) ([]Follow, error) {
	page, err := r.FollowPage(ctx, "", 1000)
	return page.Items, err
}

func (r *Repository) FollowPage(ctx context.Context, cursor string, limit int) (FollowPage, error) {
	out := FollowPage{Items: []Follow{}}
	if limit < 1 || limit > 1000 {
		return out, ErrFollowPageSize
	}
	tx, owner, err := r.db.productTx(ctx, r.token)
	if err != nil {
		return out, err
	}
	defer rollback(tx)

	before, afterID, err := parseReleaseCursor(cursor)
	if err != nil {
		return out, ErrFollowCursor
	}
	rows, err := tx.Query(ctx, `SELECT f.anime_id::text,a.canonical_title,f.release_track,f.follow_scope,f.schedule_notices_since IS NOT NULL,f.created_at,f.id::text
		FROM user_follows f JOIN anime a ON a.id=f.anime_id
		WHERE f.user_id=$1 AND ($2::timestamptz IS NULL OR f.created_at<$2 OR (f.created_at=$2 AND f.id>$3::uuid))
		ORDER BY f.created_at DESC,f.id LIMIT $4`, owner.ID, before, afterID, limit+1)
	if err != nil {
		return out, err
	}
	defer rows.Close()

	var lastID string
	for rows.Next() {
		var follow Follow
		var id string
		if err := rows.Scan(&follow.AnimeID, &follow.Title, &follow.Track, &follow.Scope, &follow.ScheduleNotices, &follow.CreatedAt, &id); err != nil {
			return out, err
		}
		if len(out.Items) == limit {
			out.NextCursor = releaseCursor(out.Items[limit-1].CreatedAt, lastID)
			break
		}
		out.Items = append(out.Items, follow)
		lastID = id
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return out, err
	}
	return out, tx.Commit(ctx)
}
// FollowForTitle resolves the caller's current subscription without paging.
func (r *Repository) FollowForTitle(ctx context.Context, id string) (Follow, bool, error) {
	var follow Follow
	tx, owner, err := r.db.productTx(ctx, r.token)
	if err != nil {
		return follow, false, err
	}
	defer rollback(tx)
	err = tx.QueryRow(ctx, `SELECT f.anime_id::text,a.canonical_title,f.release_track,f.follow_scope,f.schedule_notices_since IS NOT NULL,f.created_at
		FROM user_follows f JOIN anime a ON a.id=f.anime_id
		WHERE f.user_id=$1 AND f.anime_id=$2
		ORDER BY f.created_at DESC, f.id LIMIT 1`, owner.ID, id).
		Scan(&follow.AnimeID, &follow.Title, &follow.Track, &follow.Scope, &follow.ScheduleNotices, &follow.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return follow, false, nil
	}
	if err != nil {
		return follow, false, err
	}
	return follow, true, tx.Commit(ctx)
}

func (r *Repository) Follow(ctx context.Context,id,track string) (bool,error) {
	return r.FollowScope(ctx,id,track,"ANIME")
}
func (r *Repository) FollowScope(ctx context.Context,id,track,scope string) (bool,error) {
	return r.FollowWithSettings(ctx, id, track, scope, nil)
}

func (r *Repository) FollowWithSettings(ctx context.Context, id, track, scope string, scheduleNotices *bool) (bool, error) {
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
	var started, now time.Time
	var since *time.Time
	var sameSettings bool
	err = tx.QueryRow(ctx, `SELECT COALESCE(min(created_at),statement_timestamp()), statement_timestamp(), max(schedule_notices_since),
		COALESCE(bool_or(follow_scope=$3 AND release_track=$4), false)
		FROM user_follows WHERE user_id=$1 AND anime_id=$2`, u.ID, id, scope, track).
		Scan(&started, &now, &since, &sameSettings)
	if err != nil {
		return false, err
	}
	if scheduleNotices != nil && !*scheduleNotices {
		since = nil
	} else if (scheduleNotices != nil && since == nil) || (since != nil && !sameSettings) {
		since = &now
	}
	if _,err=tx.Exec(ctx,`DELETE FROM user_follows WHERE user_id=$1 AND anime_id=$2 AND follow_scope<>$3`,u.ID,id,scope);err!=nil {return false,err}
	_,err=tx.Exec(ctx,`INSERT INTO user_follows(user_id,anime_id,follow_scope,release_track,created_at,schedule_notices_since)
VALUES($1,$2,$3,$4,$5,$6)
ON CONFLICT(user_id,anime_id,follow_scope) DO UPDATE SET
  release_track=excluded.release_track,schedule_notices_since=excluded.schedule_notices_since`,u.ID,id,scope,track,started,since)
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

type NotificationMaterialization struct {
 Created int `json:"created"`
 HasMore bool `json:"hasMore"`
 OldestPendingAt *time.Time `json:"oldestPendingAt,omitempty"`
}
type NotificationPage struct {
	UnreadCount int64 `json:"unreadCount"`
 Materialization *NotificationMaterialization `json:"materialization,omitempty"`
	Items []Notification `json:"items"`
	NextCursor string `json:"nextCursor,omitempty"`
}
func releaseCursor(createdAt time.Time, id string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(createdAt.UTC().Format(time.RFC3339Nano) + "|" + id))
}
func parseReleaseCursor(cursor string)(*time.Time,*string,error){
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
	return r.FilterNotificationHistory(ctx, cursor, false)
}

// FilterNotificationHistory applies the unread filter before the page limit.
func (r *Repository) FilterNotificationHistory(ctx context.Context, cursor string, unreadOnly bool) (NotificationPage, error) {
	return r.readNotificationPage(ctx, cursor, unreadOnly, 101)
}

// NotificationSummary materializes the latest batch and counts unread notices
// without selecting any notification text.
func (r *Repository) NotificationSummary(ctx context.Context) (NotificationPage, error) {
	return r.readNotificationPage(ctx, "", false, 0)
}

func (r *Repository) readNotificationPage(ctx context.Context, cursor string, unreadOnly bool, pageLimit int) (NotificationPage, error) {
	out:=NotificationPage{Items:[]Notification{}}
	tx,u,err:=r.db.productTx(ctx,r.token)
	if err!=nil {return out,err}
	defer rollback(tx)
	before,beforeID,err:=parseReleaseCursor(cursor)
	if err!=nil{return out,err}
	if _,err=tx.Exec(ctx,`SET LOCAL statement_timeout='2s'`);err!=nil {return out,err}
	if cursor=="" {
	if _,err=tx.Exec(ctx,`SELECT pg_advisory_xact_lock(hashtextextended($1,742019685))`,u.ID);err!=nil {return out,err}
	status:=NotificationMaterialization{}
 // For one reachable title, track and scope, an event needs any qualifying
 // subscription. The aggregate retains the earliest follow and consent;
 // the schema ensures that consent cannot precede its own follow.
 err=tx.QueryRow(ctx,releaseTargetsSQL+`, eligible AS MATERIALIZED (
SELECT DISTINCT e.id AS event_id,
CASE WHEN e.event_type='SEASON_ANNOUNCED' THEN 'New season announced'
     WHEN kind.movie_release THEN 'Movie released'
     WHEN e.event_type='SCHEDULE_CHANGED' AND e.payload->>'direction'='LATER' THEN 'Schedule moved later'
     WHEN e.event_type='SCHEDULE_CHANGED' THEN 'Schedule moved earlier' ELSE 'New episode released' END AS notice_title,
CASE WHEN e.event_type='SEASON_ANNOUNCED' THEN a.canonical_title || ' · A new linked season has been announced.'
     WHEN kind.movie_release THEN a.canonical_title || ' · Release confirmed (' || COALESCE(e.payload->>'track','UNKNOWN') || ')'
WHEN e.event_type='SCHEDULE_CHANGED' THEN a.canonical_title || ' · Episode ' || (e.payload->>'number') || ' (' ||
  (e.payload->>'track') || ') · Unconfirmed schedule: ' || (e.payload->>'previousSchedule') || ' → ' || (e.payload->>'currentSchedule')
ELSE a.canonical_title || ' · Episode ' || COALESCE(e.payload->>'number','?') || ' (' || COALESCE(e.payload->>'track','UNKNOWN') || ')' END AS notice_body,
e.occurred_at AS created_at
FROM domain_events e JOIN targets f ON f.anime_id=e.aggregate_id JOIN anime a ON a.id=e.aggregate_id
CROSS JOIN LATERAL (SELECT e.event_type='EPISODE_RELEASED' AND e.payload->>'mediaType'='MOVIE'
  AND e.payload->>'number'='1' AS movie_release) kind
WHERE e.aggregate_type='ANIME' AND e.occurred_at>=f.created_at
AND ((e.event_type='EPISODE_RELEASED' AND (f.release_track='ALL' OR f.release_track=e.payload->>'track'))
OR (e.event_type='SEASON_ANNOUNCED' AND f.scope='FRANCHISE')
OR (e.event_type='SCHEDULE_CHANGED' AND e.occurred_at>=f.schedule_notices_since
  AND (f.release_track='ALL' OR f.release_track=e.payload->>'track')))
AND NOT EXISTS(SELECT 1 FROM notifications n WHERE n.user_id=$1 AND n.event_id=e.id AND n.channel='IN_APP')
ORDER BY e.occurred_at,e.id LIMIT 101
), inserted AS (
INSERT INTO notifications(user_id,event_id,channel,title,body,created_at)
SELECT $1::uuid,event_id,'IN_APP',notice_title,notice_body,created_at FROM eligible
ORDER BY created_at,event_id LIMIT 100
ON CONFLICT(user_id,event_id,channel) WHERE event_id IS NOT NULL DO NOTHING
RETURNING id
)
SELECT (SELECT count(*) FROM inserted), pending.created_at IS NOT NULL, pending.created_at
FROM (SELECT 1) AS result
LEFT JOIN LATERAL (SELECT created_at FROM eligible ORDER BY created_at,event_id OFFSET 100 LIMIT 1) AS pending ON true`,u.ID).Scan(&status.Created,&status.HasMore,&status.OldestPendingAt)
 if err==nil{out.Materialization=&status}
	if err!=nil {return out,err}
	}
	rows, err := tx.Query(ctx, `SELECT page.id::text,page.title,page.body,page.read_at,page.created_at,page.provider,total.unread
	FROM (
		SELECT count(*) AS unread FROM notifications
		WHERE user_id=$1 AND channel='IN_APP' AND read_at IS NULL
	) total
	LEFT JOIN LATERAL (
		SELECT n.id,n.title,n.body,n.read_at,n.created_at,
			COALESCE(CASE WHEN e.event_type='SCHEDULE_CHANGED' THEN e.payload->>'provider' END, '') AS provider
		FROM notifications n LEFT JOIN domain_events e ON e.id=n.event_id
		WHERE n.user_id=$1 AND n.channel='IN_APP' AND (NOT $4::boolean OR n.read_at IS NULL)
		  AND ($2::timestamptz IS NULL OR (n.created_at,n.id)<($2::timestamptz,$3::uuid))
		ORDER BY n.created_at DESC,n.id DESC LIMIT $5
	) page ON true
	ORDER BY page.created_at DESC,page.id DESC`, u.ID, before, beforeID, unreadOnly, pageLimit)
	if err != nil {
		return out, err
	}
	for rows.Next() {
		var id, title, body, provider *string
		var createdAt, readAt *time.Time
		if err = rows.Scan(&id, &title, &body, &readAt, &createdAt, &provider, &out.UnreadCount); err != nil {
			rows.Close()
			return out, err
		}
		// An empty page still returns the account-wide total from the same snapshot.
		if id != nil {
			out.Items = append(out.Items, Notification{
				ID: *id, Title: *title, Body: *body, Provider: *provider, ReadAt: readAt, CreatedAt: *createdAt,
			})
		}
	}
	rows.Close()
	if err=rows.Err();err!=nil {return out,err}
	if len(out.Items)>100{out.Items=out.Items[:100];out.NextCursor=releaseCursor(out.Items[99].CreatedAt, out.Items[99].ID)}
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


func (r *Repository) ReadAllNotifications(ctx context.Context) (int64, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	tx, owner, err := r.db.productTx(ctx, r.token)
	if err != nil {
		return 0, err
	}
	defer rollback(tx)
	tag, err := tx.Exec(ctx, `UPDATE notifications SET read_at=now()
		WHERE user_id=$1 AND channel='IN_APP' AND read_at IS NULL`, owner.ID)
	if err != nil {
		return 0, err
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}
