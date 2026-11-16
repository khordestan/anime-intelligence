package postgres_test

import(
 "context"
 "errors"
 "net/url"
 "os"
 "reflect"
 "strings"
 "testing"
 "time"

 "github.com/jackc/pgx/v5"
 "github.com/khordestan/anime-intelligence/internal/modules/catalog/feed"
 "github.com/khordestan/anime-intelligence/internal/platform/catalogsync"
)
func TestCatalogHistoryCursorDiffAndReaderIsolation(t *testing.T){
 _,owner,_,dsn:=testDB(t);ctx:=context.Background()
 u,err:=url.Parse(dsn);if err!=nil{t.Fatal(err)}
 role:=u.User.Username()+"_history";quoted:=pgx.Identifier{role}.Sanitize()
 script,err:=os.ReadFile("../../../deployments/catalog-history-role.sql");if err!=nil{t.Fatal(err)}
 if _,err=owner.Exec(ctx,strings.ReplaceAll(string(script),"anime_catalog_history",quoted));err!=nil{t.Fatal(err)}
 t.Cleanup(func(){_,_=owner.Exec(ctx,`DROP OWNED BY `+quoted);_,_=owner.Exec(ctx,`DROP ROLE `+quoted)})
 password,_:=u.User.Password()
 if _,err=owner.Exec(ctx,`ALTER ROLE `+quoted+` PASSWORD '`+password+`'`);err!=nil{t.Fatal(err)}
 u.User=url.UserPassword(role,password)
 reader,err:=pgx.Connect(ctx,u.String());if err!=nil{t.Fatal(err)};t.Cleanup(func(){_=reader.Close(ctx)})
 if err=catalogsync.CheckWriter(ctx,reader);err!=nil{t.Fatal("restricted history reader rejected",err)}
 if err=catalogsync.CheckWriter(ctx,owner);err==nil{t.Fatal("migration credentials accepted")}
 if _,err=owner.Exec(ctx,`INSERT INTO data_sources(provider_key,display_name,enabled,policy_url) VALUES('history-fixture','Synthetic history',true,'https://example.test/policy'),('history-other','Other synthetic',true,'https://example.test/policy')`);err!=nil{t.Fatal(err)}
 now:=time.Now().UTC().Truncate(time.Microsecond)
 snapshot:=feed.Snapshot{Version:1,Provider:"history-fixture",ObservedAt:now.Add(-3*time.Hour),Titles:[]feed.Title{{ExternalID:"one",Title:"Synthetic First",MediaType:"TV",Status:"UNKNOWN"}}}
 options:=catalogsync.ImportOptions{Archive:true}
 if _,err=catalogsync.ImportWithOptions(ctx,owner,snapshot,now,options);err!=nil{t.Fatal(err)}
 snapshot.ObservedAt=now.Add(-2*time.Hour);snapshot.Titles[0].Synopsis="Synthetic changed prose"
 if _,err=catalogsync.ImportWithOptions(ctx,owner,snapshot,now,options);err!=nil{t.Fatal(err)}
 snapshot.ObservedAt=now.Add(-time.Hour);snapshot.Titles[0].Title="Synthetic Latest"
 if _,err=catalogsync.ImportWithOptions(ctx,owner,snapshot,now,options);err!=nil{t.Fatal(err)}
 // Non-metadata audits and another provider's exact same external ID are excluded.
 if _,err=owner.Exec(ctx,`INSERT INTO source_records(source_id,external_id,record_type,payload,content_hash) SELECT id,'one','IDENTITY_MAP','{}'::jsonb,'synthetic-audit' FROM data_sources WHERE provider_key='history-fixture'`);err!=nil{t.Fatal(err)}
 snapshot.Provider="history-other";snapshot.ObservedAt=now;snapshot.Titles[0].Title="Synthetic Other"
 if _,err=catalogsync.ImportWithOptions(ctx,owner,snapshot,now,options);err!=nil{t.Fatal(err)}
 result,err:=catalogsync.History(ctx,reader,"history-fixture","one",1,nil)
 if err!=nil||len(result.Entries)!=1||!result.Truncated||result.NextBefore==nil||result.Entries[0].Baseline||!reflect.DeepEqual(result.Entries[0].ChangedFields,[]string{"title"})||!result.Entries[0].ObservedAt.Equal(now.Add(-time.Hour)){t.Fatal("latest page comparison",result,err)}
 second,err:=catalogsync.History(ctx,reader,"history-fixture","one",1,result.NextBefore)
 if err!=nil||len(second.Entries)!=1||!second.Truncated||!reflect.DeepEqual(second.Entries[0].ChangedFields,[]string{"synopsis"}){t.Fatal("page boundary comparison",second,err)}
 third,err:=catalogsync.History(ctx,reader,"history-fixture","one",1,second.NextBefore)
 if err!=nil||len(third.Entries)!=1||third.Truncated||third.NextBefore!=nil||!third.Entries[0].Baseline||len(third.Entries[0].ChangedFields)!=0{t.Fatal("oldest retained baseline",third,err)}
 empty,err:=catalogsync.History(ctx,reader,"history-fixture","missing",20,nil)
 if err!=nil||empty.Entries==nil||len(empty.Entries)!=0||empty.Truncated{t.Fatal("empty exact ID",empty,err)}
 all,err:=catalogsync.History(ctx,reader,"history-fixture","one",100,nil)
 if err!=nil||len(all.Entries)!=3||all.Truncated{t.Fatal("history isolation",all,err)}
 for _,limit:=range []int{0,101}{if _,err=catalogsync.History(ctx,nil,"history-fixture","one",limit,nil);!errors.Is(err,catalogsync.ErrHistory){t.Fatal("invalid limit accepted")}}
 for _,query:=range []string{`SELECT email FROM users`,`SELECT count(*) FROM watch_cycles`,`SELECT policy_url FROM data_sources`,`INSERT INTO source_records(source_id,external_id,record_type,payload,content_hash) SELECT id,'one','CATALOG_METADATA','{}'::jsonb,'forbidden' FROM data_sources`,`UPDATE data_sources SET enabled=false`,`DELETE FROM source_records`}{
  if _,err=reader.Exec(ctx,query);err==nil{t.Fatal("excess history reader privilege")}
 }
 // Corrupt archives fail with a fixed error, never emitting stored contents.
 if _,err=owner.Exec(ctx,`INSERT INTO source_records(source_id,external_id,record_type,payload,content_hash) SELECT id,'one','CATALOG_METADATA','{"observedAt":"invalid","synopsis":"Synthetic untrusted prose"}'::jsonb,'synthetic-corrupt' FROM data_sources WHERE provider_key='history-fixture'`);err!=nil{t.Fatal(err)}
 if _,err=catalogsync.History(ctx,reader,"history-fixture","one",20,nil);!errors.Is(err,catalogsync.ErrHistory){t.Fatal("malformed observation accepted",err)}
}
