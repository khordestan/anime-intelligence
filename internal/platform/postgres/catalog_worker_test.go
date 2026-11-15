package postgres_test

import (
	"context"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/khordestan/anime-intelligence/internal/modules/catalog/feed"
	"github.com/khordestan/anime-intelligence/internal/platform/catalogsync"
	"github.com/khordestan/anime-intelligence/internal/providers/wikidata"
)
type fakeWikidataFetcher struct{calls int;at time.Time;err error}
func(f *fakeWikidataFetcher)Fetch(context.Context,[]string)(feed.Snapshot,error){
	f.calls++
	if f.err!=nil{return feed.Snapshot{},f.err}
	return feed.Snapshot{Version:1,Provider:"wikidata",ObservedAt:f.at,Titles:[]feed.Title{{ExternalID:"Q1",Title:"Synthetic Worker Title",MediaType:"TV",Status:"UNKNOWN"}}},nil
}
func TestCatalogWorkerDedicatedRoleApprovalAndDurableDueTime(t *testing.T){
	_,owner,ownerDSN,runtimeDSN:=testDB(t)
	ctx:=context.Background()
	u,err:=url.Parse(runtimeDSN)
	if err!=nil{t.Fatal(err)}
	role:=u.User.Username()+"_catalog"
	script,err:=os.ReadFile("../../../deployments/catalog-writer-role.sql")
	if err!=nil{t.Fatal(err)}
	if _,err=owner.Exec(ctx,strings.ReplaceAll(string(script),"anime_catalog_writer",pgx.Identifier{role}.Sanitize()));err!=nil{t.Fatal(err)}
	t.Cleanup(func(){_,_=owner.Exec(ctx,`DROP OWNED BY `+pgx.Identifier{role}.Sanitize());_,_=owner.Exec(ctx,`DROP ROLE `+pgx.Identifier{role}.Sanitize())})
	password,_:=u.User.Password()
	if _,err=owner.Exec(ctx,`ALTER ROLE `+pgx.Identifier{role}.Sanitize()+` PASSWORD '`+password+`'`);err!=nil{t.Fatal(err)}
	u.User=url.UserPassword(role,password)
	writer,err:=pgx.Connect(ctx,u.String())
	if err!=nil{t.Fatal(err)}
	t.Cleanup(func(){_=writer.Close(ctx)})
	if err=catalogsync.CheckWriter(ctx,writer);err!=nil{t.Fatal("restricted writer rejected",err)}
	admin,err:=pgx.Connect(ctx,ownerDSN)
	if err!=nil{t.Fatal(err)}
	defer admin.Close(ctx)
	if err=catalogsync.CheckWriter(ctx,admin);err==nil{t.Fatal("administrator accepted as worker")}
	api,err:=pgx.Connect(ctx,runtimeDSN)
	if err!=nil{t.Fatal(err)}
	defer api.Close(ctx)
	if err=catalogsync.CheckWriter(ctx,api);err==nil{t.Fatal("account-access role accepted as worker")}
	now:=time.Now().UTC().Truncate(time.Microsecond)
	f:=&fakeWikidataFetcher{at:now}
	if _,err=catalogsync.SyncWikidata(ctx,writer,f,[]string{"Q1"},time.Hour,now);err!=catalogsync.ErrSource||f.calls!=0{t.Fatal("fetch before approval",err,f.calls)}
	if _,err=owner.Exec(ctx,`INSERT INTO data_sources(provider_key,display_name,policy_url,enabled) VALUES('wikidata','Synthetic approved source','https://example.test/policy',true)`);err!=nil{t.Fatal(err)}
	if _,err=writer.Exec(ctx,`UPDATE data_sources SET enabled=false WHERE provider_key='wikidata'`);err==nil{t.Fatal("writer changed source approval")}
	if _,err=owner.Exec(ctx,`SELECT pg_advisory_lock(742019686)`);err!=nil{t.Fatal(err)}
	did,err:=catalogsync.SyncWikidata(ctx,writer,f,[]string{"Q1"},time.Hour,now)
	if err!=nil||did||f.calls!=0{t.Fatal("concurrent collector lock ignored",did,err,f.calls)}
	if _,err=owner.Exec(ctx,`SELECT pg_advisory_unlock(742019686)`);err!=nil{t.Fatal(err)}
	did,err=catalogsync.SyncWikidata(ctx,writer,f,[]string{"Q1"},time.Hour,now)
	if err!=nil||!did||f.calls!=1{t.Fatal("first cycle",did,err,f.calls)}
	did,err=catalogsync.SyncWikidata(ctx,writer,f,[]string{"Q1"},time.Hour,now.Add(30*time.Minute))
	if err!=nil||did||f.calls!=1{t.Fatal("persisted freshness did not suppress fetch",did,err,f.calls)}
	var title string
	if err=writer.QueryRow(ctx,`SELECT canonical_title FROM anime_external_ids x JOIN anime a ON a.id=x.anime_id WHERE x.provider='wikidata' AND x.external_id='Q1'`).Scan(&title);err!=nil||title!="Synthetic Worker Title"{t.Fatal("worker import",title,err)}
	// Simulate a fresh process after a failed first fetch: no source observation
	// exists, so only the durable attempt state can enforce the provider cooldown.
	if _,err=owner.Exec(ctx,`DELETE FROM catalog_sync_state; DELETE FROM catalog_fetch_state`);err!=nil{t.Fatal(err)}
	until:=now.Add(24*time.Hour)
	f.err=&wikidata.RateLimitError{Wait:24*time.Hour,Until:until}
	did,err=catalogsync.SyncWikidata(ctx,writer,f,[]string{"Q1"},time.Hour,now)
	if err==nil||did||f.calls!=2{t.Fatal("rate-limited attempt",did,err,f.calls)}
	var outcome string
	var retry time.Time
	var attempts int
	if err=writer.QueryRow(ctx,`SELECT outcome,next_attempt_at,attempts FROM catalog_fetch_state`).Scan(&outcome,&retry,&attempts);err!=nil||outcome!="RATE_LIMITED"||!retry.Equal(until)||attempts!=1{t.Fatal("persisted provider cooldown",outcome,retry,attempts,err)}
	// New fetcher/connection models a worker restart with no process-local timer.
	restarted,err:=pgx.Connect(ctx,u.String())
	if err!=nil{t.Fatal(err)}
	defer restarted.Close(ctx)
	freshFetcher:=&fakeWikidataFetcher{at:now}
	did,err=catalogsync.SyncWikidata(ctx,restarted,freshFetcher,[]string{"Q1"},time.Hour,now.Add(2*time.Hour))
	if err!=nil||did||freshFetcher.calls!=0{t.Fatal("restart bypassed cooldown",did,err,freshFetcher.calls)}
	// A process death after admission also retains the minimum interval.
	if _,err=owner.Exec(ctx,`UPDATE catalog_fetch_state SET outcome='STARTED',next_attempt_at=$1`,now.Add(time.Hour));err!=nil{t.Fatal(err)}
	did,err=catalogsync.SyncWikidata(ctx,restarted,freshFetcher,[]string{"Q1"},time.Hour,now.Add(30*time.Minute))
	if err!=nil||did||freshFetcher.calls!=0{t.Fatal("crash admission did not delay fetch",did,err,freshFetcher.calls)}
	if _,err=owner.Exec(ctx,`GRANT SELECT(email) ON users TO `+pgx.Identifier{role}.Sanitize());err!=nil{t.Fatal(err)}
	if err=catalogsync.CheckWriter(ctx,writer);err==nil{t.Fatal("private column grant accepted")}
}
