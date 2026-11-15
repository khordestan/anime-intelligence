package catalogsync

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)
var ErrStatus = errors.New("catalog status unavailable")
type SourceStatus struct {
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
	if staleAfter<time.Minute || staleAfter>30*24*time.Hour {return Status{},ErrStatus}
	out,err:=inspect(ctx,conn,now.UTC(),staleAfter)
	if err!=nil {return Status{},ErrStatus}
	return out,nil
}
func inspect(ctx context.Context,conn *pgx.Conn,now time.Time,staleAfter time.Duration)(Status,error) {
	out:=Status{AsOf:now,StaleAfterSeconds:int64(staleAfter.Seconds()),Sources:[]SourceStatus{}}
	tx,err:=conn.BeginTx(ctx,pgx.TxOptions{IsoLevel:pgx.RepeatableRead,AccessMode:pgx.ReadOnly})
	if err!=nil {return out,err}
	defer tx.Rollback(context.Background())
	rows,err:=tx.Query(ctx,`SELECT s.provider_key,s.enabled,COALESCE(s.policy_url,'')<>'',st.observed_at,
(SELECT count(*) FROM catalog_metadata m WHERE m.source_id=s.id),f.last_attempt_at,f.next_attempt_at,f.outcome,f.attempts
FROM data_sources s LEFT JOIN catalog_sync_state st ON st.source_id=s.id LEFT JOIN catalog_fetch_state f ON f.source_id=s.id ORDER BY s.provider_key`)
	if err!=nil {return out,err}
	for rows.Next() {
		var source SourceStatus
		if err=rows.Scan(&source.Provider,&source.Enabled,&source.PolicyRecorded,&source.ObservedAt,&source.Titles,&source.LastAttemptAt,&source.NextAttemptAt,&source.Outcome,&source.Attempts);err!=nil {rows.Close();return out,err}
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
		out.Sources=append(out.Sources,source)
	}
	rows.Close()
	if err=rows.Err();err!=nil {return out,err}
	if err=tx.QueryRow(ctx,`SELECT count(*) FROM anime`).Scan(&out.Titles);err!=nil {return out,err}
	if err=tx.QueryRow(ctx,`SELECT count(*),count(*) FILTER(WHERE actual_released_at IS NOT NULL)
FROM (SELECT DISTINCT ON(anime_id,episode_number,release_track) actual_released_at
FROM airing_schedule ORDER BY anime_id,episode_number,release_track,revision DESC) latest`).Scan(&out.EpisodeSchedules,&out.ConfirmedEpisodes);err!=nil {return out,err}
	return out,tx.Commit(ctx)
}
