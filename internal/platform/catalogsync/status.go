package catalogsync

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)
var ErrStatus = errors.New("catalog status unavailable")
// MetadataCoverage counts presence, not factual correctness or completeness.
type MetadataCoverage struct {
	EpisodeTotals int `json:"episodeTotals"`
	Synopses int `json:"synopses"`
	Synonyms int `json:"synonyms"`
	Genres int `json:"genres"`
	Tags int `json:"tags"`
	Themes int `json:"themes"`
	Demographics int `json:"demographics"`
	Studios int `json:"studios"`
	Relations int `json:"relations"`
	StaffCredits int `json:"staffCredits"`
	CharacterCredits int `json:"characterCredits"`
}
type SourceStatus struct {
	StaleTitles int `json:"staleTitles"`
	FutureTitles int `json:"futureTitles"`
	OldestTitleObservation *time.Time `json:"oldestTitleObservation"`
	NewestTitleObservation *time.Time `json:"newestTitleObservation"`
	Coverage MetadataCoverage `json:"coverage"`
	Provider string `json:"provider"`
	LastAttemptAt *time.Time `json:"lastAttemptAt"`
	NextAttemptAt *time.Time `json:"nextAttemptAt"`
	Outcome *string `json:"outcome"`
	Attempts *int `json:"attempts"`
	Enabled bool `json:"enabled"`
	PolicyRecorded bool `json:"policyRecorded"`
	ObservedAt *time.Time `json:"observedAt"`
	AgeSeconds *int64 `json:"ageSeconds"`
	Titles int `json:"titles"`
	NeverSynced bool `json:"neverSynced"`
	FutureObservation bool `json:"futureObservation"`
	Stale bool `json:"stale"`
}
type Status struct {
	ObservedTitles int `json:"observedTitles"`
	UnobservedTitles int `json:"unobservedTitles"`
	StaleTitles int `json:"staleTitles"`
	FutureTitles int `json:"futureTitles"`
	Coverage MetadataCoverage `json:"coverage"`
	AsOf time.Time `json:"asOf"`
	StaleAfterSeconds int64 `json:"staleAfterSeconds"`
	Sources []SourceStatus `json:"sources"`
	StaleSources int `json:"staleSources"`
	Titles int `json:"titles"`
	EpisodeSchedules int `json:"episodeSchedules"`
	ConfirmedEpisodes int `json:"confirmedEpisodes"`
}
// Inspect reads only public catalog tables in one read-only snapshot. No users,
// sessions, follows, watch progress or notifications are queried.
func Inspect(ctx context.Context,conn *pgx.Conn,now time.Time,staleAfter time.Duration)(Status,error) {
	if now.IsZero() || now.Unix()<0 || staleAfter<time.Minute || staleAfter>30*24*time.Hour {return Status{},ErrStatus}
	out,err:=inspect(ctx,conn,now.UTC(),staleAfter)
	if err!=nil {return Status{},ErrStatus}
	return out,nil
}
func inspect(ctx context.Context,conn *pgx.Conn,now time.Time,staleAfter time.Duration)(Status,error) {
	out:=Status{AsOf:now,StaleAfterSeconds:int64(staleAfter.Seconds()),Sources:[]SourceStatus{}}
	tx,err:=conn.BeginTx(ctx,pgx.TxOptions{IsoLevel:pgx.RepeatableRead,AccessMode:pgx.ReadOnly})
	if err!=nil {return out,err}
	defer func(){cleanup,cancel:=context.WithTimeout(context.Background(),5*time.Second);defer cancel();_=tx.Rollback(cleanup)}()
 if _,err=tx.Exec(ctx,"SET LOCAL statement_timeout='2s'");err!=nil{return out,err}
	rows,err:=tx.Query(ctx,`WITH metadata AS (
SELECT source_id,count(*) AS titles,
count(*) FILTER(WHERE observed_at<$2 OR observed_at>$1) AS stale,
count(*) FILTER(WHERE observed_at>$1) AS future,
min(observed_at) AS oldest,max(observed_at) AS newest,
count(*) FILTER(WHERE jsonb_typeof(payload->'episodeCount')='number') AS totals,
count(*) FILTER(WHERE jsonb_typeof(payload->'synopsis')='string' AND length(btrim(payload->>'synopsis'))>0) AS synopses,
count(*) FILTER(WHERE CASE WHEN jsonb_typeof(payload->'synonyms')='array' THEN jsonb_array_length(payload->'synonyms')>0 ELSE false END) AS synonyms,
count(*) FILTER(WHERE CASE WHEN jsonb_typeof(payload->'genres')='array' THEN jsonb_array_length(payload->'genres')>0 ELSE false END) AS genres,
count(*) FILTER(WHERE CASE WHEN jsonb_typeof(payload->'tags')='array' THEN jsonb_array_length(payload->'tags')>0 ELSE false END) AS tags,
count(*) FILTER(WHERE CASE WHEN jsonb_typeof(payload->'studios')='array' THEN jsonb_array_length(payload->'studios')>0 ELSE false END) AS studios,
count(*) FILTER(WHERE CASE WHEN jsonb_typeof(payload->'relations')='array' THEN jsonb_array_length(payload->'relations')>0 ELSE false END) AS relations,
count(*) FILTER(WHERE CASE WHEN jsonb_typeof(payload->'themes')='array' THEN jsonb_array_length(payload->'themes')>0 ELSE false END) AS themes,
count(*) FILTER(WHERE CASE WHEN jsonb_typeof(payload->'demographics')='array' THEN jsonb_array_length(payload->'demographics')>0 ELSE false END) AS demographics,
count(*) FILTER(WHERE CASE WHEN jsonb_typeof(payload->'staffCredits')='array' THEN jsonb_array_length(payload->'staffCredits')>0 ELSE false END) AS staff_credits,
count(*) FILTER(WHERE CASE WHEN jsonb_typeof(payload->'characterCredits')='array' THEN jsonb_array_length(payload->'characterCredits')>0 ELSE false END) AS character_credits
FROM (SELECT m.anime_id,m.source_id,m.observed_at,COALESCE(b.payload,m.payload) AS payload
 FROM catalog_metadata m LEFT JOIN catalog_base_metadata b ON b.anime_id=m.anime_id) primary_facts GROUP BY source_id
)
SELECT s.provider_key,s.enabled,COALESCE(s.policy_url,'')<>'',st.observed_at,
COALESCE(m.titles,0),f.last_attempt_at,f.next_attempt_at,f.outcome,f.attempts,
COALESCE(m.stale,0),COALESCE(m.future,0),m.oldest,m.newest,
COALESCE(m.totals,0),COALESCE(m.synopses,0),COALESCE(m.synonyms,0),
COALESCE(m.genres,0),COALESCE(m.tags,0),COALESCE(m.studios,0),COALESCE(m.relations,0),COALESCE(m.themes,0),COALESCE(m.demographics,0),
COALESCE(m.staff_credits,0),COALESCE(m.character_credits,0)
FROM data_sources s LEFT JOIN catalog_sync_state st ON st.source_id=s.id
LEFT JOIN catalog_fetch_state f ON f.source_id=s.id LEFT JOIN metadata m ON m.source_id=s.id
ORDER BY s.provider_key`,now,now.Add(-staleAfter))
	if err!=nil {return out,err}
	for rows.Next() {
		var source SourceStatus
		if err=rows.Scan(&source.Provider,&source.Enabled,&source.PolicyRecorded,&source.ObservedAt,&source.Titles,&source.LastAttemptAt,&source.NextAttemptAt,&source.Outcome,&source.Attempts,
&source.StaleTitles,&source.FutureTitles,&source.OldestTitleObservation,&source.NewestTitleObservation,
&source.Coverage.EpisodeTotals,&source.Coverage.Synopses,&source.Coverage.Synonyms,
&source.Coverage.Genres,&source.Coverage.Tags,&source.Coverage.Studios,&source.Coverage.Relations,&source.Coverage.Themes,&source.Coverage.Demographics,&source.Coverage.StaffCredits,&source.Coverage.CharacterCredits);err!=nil {rows.Close();return out,err}
		source.NeverSynced=source.ObservedAt==nil
		if source.ObservedAt!=nil {
			age:=now.Sub(*source.ObservedAt)
			source.FutureObservation=age<0
			if age<0 {age=0}
			seconds:=int64(age.Seconds())
			source.AgeSeconds=&seconds
		}
		source.Stale=source.Enabled && (!source.PolicyRecorded || source.NeverSynced || source.FutureObservation || now.Sub(*source.ObservedAt)>staleAfter)
		if source.Stale {out.StaleSources++}
		out.ObservedTitles+=source.Titles
		out.StaleTitles+=source.StaleTitles
		out.FutureTitles+=source.FutureTitles
		out.Coverage.EpisodeTotals+=source.Coverage.EpisodeTotals
		out.Coverage.Synopses+=source.Coverage.Synopses
		out.Coverage.Synonyms+=source.Coverage.Synonyms
		out.Coverage.Genres+=source.Coverage.Genres
		out.Coverage.Tags+=source.Coverage.Tags
		out.Coverage.Themes+=source.Coverage.Themes
		out.Coverage.Demographics+=source.Coverage.Demographics
		out.Coverage.StaffCredits+=source.Coverage.StaffCredits
		out.Coverage.CharacterCredits+=source.Coverage.CharacterCredits
		out.Coverage.Studios+=source.Coverage.Studios
		out.Coverage.Relations+=source.Coverage.Relations
		out.Sources=append(out.Sources,source)
	}
	rows.Close()
	if err=rows.Err();err!=nil {return out,err}
	if err=tx.QueryRow(ctx,`SELECT count(*) FROM anime`).Scan(&out.Titles);err!=nil {return out,err}
	out.UnobservedTitles=out.Titles-out.ObservedTitles
	if err=tx.QueryRow(ctx,`SELECT count(*),count(*) FILTER(WHERE actual_released_at IS NOT NULL)
FROM (SELECT DISTINCT ON(anime_id,episode_number,release_track) actual_released_at
FROM airing_schedule ORDER BY anime_id,episode_number,release_track,revision DESC) latest`).Scan(&out.EpisodeSchedules,&out.ConfirmedEpisodes);err!=nil {return out,err}
	return out,tx.Commit(ctx)
}
