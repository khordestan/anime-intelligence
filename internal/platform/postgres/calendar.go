package postgres

import (
 "context"
 "errors"
 "time"
)
var ErrCalendarRange=errors.New("invalid calendar window")
type CalendarItem struct{
 AnimeID string `json:"animeId"`
 Title string `json:"title"`
 Number int `json:"number"`
 Track string `json:"track"`
 ScheduledAt time.Time `json:"scheduledAt"`
 VerifiedAt *time.Time `json:"verifiedAt,omitempty"`
 Revision int `json:"revision"`
 Provider string `json:"provider,omitempty"`
}
type ReleaseCalendar struct{
 From time.Time `json:"from"`
 To time.Time `json:"to"`
 Items []CalendarItem `json:"items"`
 Limit int `json:"limit"`
 Truncated bool `json:"truncated"`
}
func(r *Repository) ReleaseCalendar(ctx context.Context,from time.Time,days int)(ReleaseCalendar,error){
 from=from.UTC()
 day:=time.Date(from.Year(),from.Month(),from.Day(),0,0,0,0,time.UTC)
 if from.IsZero()||!from.Equal(day)||from.Year()<1900||days<1||days>31||from.AddDate(0,0,days).Year()>9999{return ReleaseCalendar{},ErrCalendarRange}
 return retrySnapshotRead(ctx,func()(ReleaseCalendar,error){return r.calendarSnapshot(ctx,from,days)})
}
func(r *Repository) calendarSnapshot(ctx context.Context,from time.Time,days int)(ReleaseCalendar,error){
 out:=ReleaseCalendar{From:from,To:from.AddDate(0,0,days),Items:[]CalendarItem{},Limit:100}
 tx,u,err:=r.verifiedSnapshotTx(ctx);if err!=nil{return out,err};defer rollback(tx)
 if _,err=tx.Exec(ctx,`SET LOCAL statement_timeout='2s'`);err!=nil{return out,err}
 // Resolve tracks before selecting latest revisions, then apply the window.
 // Filtering dates before DISTINCT ON could revive a superseded schedule.
 rows,err:=tx.Query(ctx,`WITH RECURSIVE franchise(origin,anime_id,release_track) AS (
 SELECT anime_id,anime_id,release_track FROM user_follows WHERE user_id=$1 AND follow_scope='FRANCHISE'
 UNION
 SELECT f.origin,CASE WHEN r.source_anime_id=f.anime_id THEN r.target_anime_id ELSE r.source_anime_id END,f.release_track
 FROM franchise f JOIN anime_relations r ON r.relation_type IN ('PREQUEL','SEQUEL')
 AND (r.source_anime_id=f.anime_id OR r.target_anime_id=f.anime_id)
),targets(anime_id,release_track) AS (
 SELECT anime_id,release_track FROM user_follows WHERE user_id=$1 AND follow_scope='ANIME'
 UNION SELECT anime_id,release_track FROM franchise
),latest AS (
 SELECT DISTINCT ON(sc.anime_id,sc.episode_number,sc.release_track)
 sc.anime_id,sc.episode_number,sc.release_track,sc.scheduled_at,sc.actual_released_at,sc.last_verified_at,sc.revision,sc.source_id
 FROM airing_schedule sc WHERE EXISTS(SELECT 1 FROM targets t WHERE t.anime_id=sc.anime_id AND (t.release_track='ALL' OR t.release_track=sc.release_track))
 ORDER BY sc.anime_id,sc.episode_number,sc.release_track,sc.revision DESC
)
SELECT a.id::text,a.canonical_title,l.episode_number,l.release_track,l.scheduled_at,l.last_verified_at,l.revision,COALESCE(s.provider_key,'')
FROM latest l JOIN anime a ON a.id=l.anime_id LEFT JOIN data_sources s ON s.id=l.source_id
WHERE l.actual_released_at IS NULL AND l.scheduled_at>=$2 AND l.scheduled_at<$3
ORDER BY l.scheduled_at,a.canonical_title,a.id,l.episode_number,l.release_track LIMIT 101`,u.ID,out.From,out.To)
 if err!=nil{return out,err}
 for rows.Next(){var item CalendarItem;if err=rows.Scan(&item.AnimeID,&item.Title,&item.Number,&item.Track,&item.ScheduledAt,&item.VerifiedAt,&item.Revision,&item.Provider);err!=nil{rows.Close();return out,err};out.Items=append(out.Items,item)}
 rows.Close();if err=rows.Err();err!=nil{return out,err}
 if len(out.Items)>100{out.Items=out.Items[:100];out.Truncated=true}
 return out,tx.Commit(ctx)
}
