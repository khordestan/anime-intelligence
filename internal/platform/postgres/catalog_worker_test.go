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
)
type fakeWikidataFetcher struct{calls int;at time.Time}
func(f *fakeWikidataFetcher)Fetch(context.Context,[]string)(feed.Snapshot,error){
	f.calls++
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
	if _,err=owner.Exec(ctx,`GRANT SELECT(email) ON users TO `+pgx.Identifier{role}.Sanitize());err!=nil{t.Fatal(err)}
	if err=catalogsync.CheckWriter(ctx,writer);err==nil{t.Fatal("private column grant accepted")}
}
