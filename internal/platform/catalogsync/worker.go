package catalogsync

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/khordestan/anime-intelligence/internal/modules/catalog/feed"
	"github.com/khordestan/anime-intelligence/internal/providers/wikidata"
)
var ErrWorker = errors.New("catalog worker configuration or cycle failed")
type Fetcher interface{Fetch(context.Context,[]string)(feed.Snapshot,error)}

// CheckWriter rejects migration credentials and roles with account-data rights.
// Only the fixed public catalog table contract is supported. Reject indirect
// access through views/foreign relations, other schemas and security definers.
// This check reads privilege metadata, never private rows.
func CheckWriter(ctx context.Context,conn *pgx.Conn)error{
	var unsafe bool
	err:=conn.QueryRow(ctx,`SELECT r.rolsuper OR r.rolbypassrls OR r.rolcreatedb OR r.rolcreaterole OR r.rolreplication
OR EXISTS(SELECT 1 FROM pg_auth_members m WHERE m.member=r.oid)
OR EXISTS(SELECT 1 FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace
WHERE n.nspname='public' AND c.relkind='r' AND pg_has_role(current_user,c.relowner,'USAGE'))
OR EXISTS(SELECT 1 FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace
WHERE left(n.nspname::text,3)<>'pg_' AND n.nspname<>'information_schema'
AND c.relkind IN ('r','p','v','m','f')
AND (n.nspname<>'public' OR c.relkind<>'r' OR c.relrowsecurity OR c.relforcerowsecurity
OR c.relname NOT IN ('anime','anime_external_ids','data_sources','source_records',
'catalog_sync_state','catalog_fetch_state','catalog_fetch_jobs','catalog_metadata','anime_relations',
'airing_schedule','domain_events','anime_features','anime_feature_observations'))
AND (has_any_column_privilege(current_user,c.oid,'SELECT') OR has_any_column_privilege(current_user,c.oid,'INSERT')
OR has_any_column_privilege(current_user,c.oid,'UPDATE') OR has_any_column_privilege(current_user,c.oid,'REFERENCES')
OR has_table_privilege(current_user,c.oid,'DELETE') OR has_table_privilege(current_user,c.oid,'TRUNCATE')
OR has_table_privilege(current_user,c.oid,'TRIGGER')))
OR EXISTS(SELECT 1 FROM pg_proc p JOIN pg_namespace n ON n.oid=p.pronamespace
WHERE left(n.nspname::text,3)<>'pg_' AND n.nspname<>'information_schema'
AND p.prosecdef AND has_function_privilege(current_user,p.oid,'EXECUTE'))
FROM pg_roles r WHERE r.rolname=current_user`).Scan(&unsafe)
	if err!=nil||unsafe{return ErrWorker}
	return nil
}
// SyncWikidata claims a session advisory lock outside any database transaction
// while fetching. Import rechecks approval and commits its own atomic batch.
// Persisted source observation times suppress premature cycles after restarts.
func SyncWikidata(ctx context.Context,conn *pgx.Conn,fetcher Fetcher,ids []string,interval time.Duration,now time.Time)(bool,error){
	if interval<time.Hour||interval>7*24*time.Hour||len(ids)==0||len(ids)>20||fetcher==nil{return false,ErrWorker}
	if err:=CheckWriter(ctx,conn);err!=nil{return false,err}
	var locked bool
	if err:=conn.QueryRow(ctx,`SELECT pg_try_advisory_lock(742019686)`).Scan(&locked);err!=nil{return false,ErrWorker}
	if !locked{return false,nil}
	defer func(){cleanup,cancel:=context.WithTimeout(context.Background(),5*time.Second);defer cancel();_,_=conn.Exec(cleanup,`SELECT pg_advisory_unlock(742019686)`)}()
	var source string
	var previous,next *time.Time
	err:=conn.QueryRow(ctx,`SELECT s.id::text,st.observed_at,f.next_attempt_at FROM data_sources s
LEFT JOIN catalog_sync_state st ON st.source_id=s.id
LEFT JOIN catalog_fetch_state f ON f.source_id=s.id
WHERE s.provider_key='wikidata' AND s.enabled AND COALESCE(s.policy_url,'')<>''`).Scan(&source,&previous,&next)
	if err!=nil{return false,ErrSource}
	if previous!=nil{
		if previous.After(now){return false,ErrWorker}
		if now.Sub(*previous)<interval{return false,nil}
	}
	if next!=nil&&now.Before(*next){return false,nil}
	// Admission persists the minimum backoff before any external request. A crash
	// therefore cannot cause an immediate refetch on process restart.
	_,err=conn.Exec(ctx,`INSERT INTO catalog_fetch_state(source_id,last_attempt_at,next_attempt_at,outcome,attempts)
VALUES($1,$2,$3,'STARTED',1) ON CONFLICT(source_id) DO UPDATE SET
last_attempt_at=excluded.last_attempt_at,next_attempt_at=excluded.next_attempt_at,outcome='STARTED',
attempts=LEAST(catalog_fetch_state.attempts+1,1000000000)`,source,now,now.Add(interval))
	if err!=nil{return false,ErrWorker}
	finish:=func(outcome string,until time.Time)error{
		if until.Before(now.Add(interval)){until=now.Add(interval)}
		_,err:=conn.Exec(ctx,`UPDATE catalog_fetch_state SET outcome=$2,next_attempt_at=$3 WHERE source_id=$1`,source,outcome,until)
		if err!=nil{return ErrWorker}
		return nil
	}
	snapshot,err:=fetcher.Fetch(ctx,ids)
	if err!=nil{
		until:=time.Now().UTC().Add(interval)
		outcome:="FETCH_FAILED"
		var limited *wikidata.RateLimitError
		if errors.As(err,&limited){
			outcome="RATE_LIMITED"
			retry:=limited.Until
			if retry.IsZero(){retry=time.Now().UTC().Add(limited.Wait)}
			if retry.After(until){until=retry}
		}
		if finish(outcome,until)!=nil{return false,ErrWorker}
		return false,ErrWorker
	}
	if snapshot.Provider!="wikidata"{
		_=finish("FETCH_FAILED",time.Now().UTC().Add(interval))
		return false,ErrWorker
	}
	if _,err=Import(ctx,conn,snapshot,time.Now().UTC());err!=nil{
		if finish("IMPORT_FAILED",time.Now().UTC().Add(interval))!=nil{return false,ErrWorker}
		return false,err
	}
	if finish("SUCCESS",time.Now().UTC().Add(interval))!=nil{return false,ErrWorker}
	return true,nil
}
