package catalogsync

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/khordestan/anime-intelligence/internal/modules/catalog/feed"
)
var ErrWorker = errors.New("catalog worker configuration or cycle failed")
type Fetcher interface{Fetch(context.Context,[]string)(feed.Snapshot,error)}

// CheckWriter rejects migration credentials and roles with account-data rights.
// This check reads privilege metadata, never private rows.
func CheckWriter(ctx context.Context,conn *pgx.Conn)error{
	var unsafe bool
	err:=conn.QueryRow(ctx,`SELECT r.rolsuper OR r.rolbypassrls OR r.rolcreatedb OR r.rolcreaterole OR r.rolreplication
OR EXISTS(SELECT 1 FROM pg_auth_members m WHERE m.member=r.oid)
OR EXISTS(SELECT 1 FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace
WHERE n.nspname='public' AND c.relkind='r' AND pg_has_role(current_user,c.relowner,'USAGE'))
OR EXISTS(SELECT 1 FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace
WHERE n.nspname='public' AND c.relname IN
('users','auth_sessions','auth_challenges','identity_outbox','identity_requests','user_anime','watch_progress','user_ratings','user_preferences','recommendations','recommendation_feedback','user_follows','notifications')
AND (has_any_column_privilege(current_user,c.oid,'SELECT') OR has_any_column_privilege(current_user,c.oid,'INSERT')
OR has_any_column_privilege(current_user,c.oid,'UPDATE') OR has_table_privilege(current_user,c.oid,'DELETE')
OR has_table_privilege(current_user,c.oid,'TRUNCATE')))
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
	var previous *time.Time
	err:=conn.QueryRow(ctx,`SELECT st.observed_at FROM data_sources s LEFT JOIN catalog_sync_state st ON st.source_id=s.id
WHERE s.provider_key='wikidata' AND s.enabled AND COALESCE(s.policy_url,'')<>''`).Scan(&previous)
	if err!=nil{return false,ErrSource}
	if previous!=nil{
		if previous.After(now){return false,ErrWorker}
		if now.Sub(*previous)<interval{return false,nil}
	}
	snapshot,err:=fetcher.Fetch(ctx,ids)
	if err!=nil{return false,ErrWorker}
	if snapshot.Provider!="wikidata"{return false,ErrWorker}
	if _,err=Import(ctx,conn,snapshot,time.Now().UTC());err!=nil{return false,err}
	return true,nil
}
