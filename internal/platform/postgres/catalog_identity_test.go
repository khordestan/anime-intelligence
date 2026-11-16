package postgres_test

import (
 "context"
 "errors"
 "net/url"
 "os"
 "strings"
 "testing"
 "time"

 "github.com/jackc/pgx/v5"
 "github.com/khordestan/anime-intelligence/internal/modules/catalog/feed"
 "github.com/khordestan/anime-intelligence/internal/platform/catalogidentity"
 "github.com/khordestan/anime-intelligence/internal/platform/catalogsync"
)
func TestReviewedIdentityLinksAreAtomicAuditedAndDoNotMerge(t *testing.T){
 db,owner,_,runtimeDSN:=testDB(t);ctx:=context.Background()
 now:=time.Now().UTC().Truncate(time.Microsecond)
 const one="10000000-0000-4000-8000-000000000001"
 const two="10000000-0000-4000-8000-000000000002"
 s:=catalogidentity.Snapshot{Version:1,Provider:"identity-fixture",ReviewedAt:now,Reference:"https://example.test/review",Mappings:[]catalogidentity.Mapping{{ExternalID:"work-1",AnimeID:one}}}
 if _,err:=catalogidentity.Apply(ctx,owner,s,now,true);!errors.Is(err,catalogidentity.ErrSource){t.Fatal("unapproved source mapped",err)}
 if _,err:=owner.Exec(ctx,`INSERT INTO data_sources(provider_key,display_name,enabled,policy_url) VALUES('identity-fixture','Synthetic identity',true,'https://example.test/policy'),('identity-other','Other synthetic identity',true,'https://example.test/policy')`);err!=nil{t.Fatal(err)}
 preview,err:=catalogidentity.Apply(ctx,owner,s,now,false)
 if err!=nil||preview.Linked!=1||preview.AuditRecords!=1||preview.Applied{t.Fatal("preview result",preview,err)}
 var n int
 if err=owner.QueryRow(ctx,`SELECT count(*) FROM anime_external_ids WHERE provider='identity-fixture'`).Scan(&n);err!=nil||n!=0{t.Fatal("preview persisted",err)}
 applied,err:=catalogidentity.Apply(ctx,owner,s,now,true)
 if err!=nil||applied.Linked!=1||applied.AuditRecords!=1||!applied.Applied{t.Fatal("identity apply",applied,err)}
 replay,err:=catalogidentity.Apply(ctx,owner,s,now,true)
 if err!=nil||replay.Unchanged!=1||replay.AuditRecords!=0{t.Fatal("review replay",replay,err)}
 conflict:=s;conflict.Mappings=[]catalogidentity.Mapping{{ExternalID:"work-1",AnimeID:two}}
 if _,err=catalogidentity.Apply(ctx,owner,conflict,now,true);!errors.Is(err,catalogidentity.ErrConflict){t.Fatal("external identity remapped",err)}
 conflict.Mappings=[]catalogidentity.Mapping{{ExternalID:"replacement",AnimeID:one}}
 if _,err=catalogidentity.Apply(ctx,owner,conflict,now,true);!errors.Is(err,catalogidentity.ErrConflict){t.Fatal("canonical/provider identity replaced",err)}
 // Earlier new rows and audit records must roll back when a later record fails.
 conflict.Mappings=[]catalogidentity.Mapping{{ExternalID:"work-2",AnimeID:two},{ExternalID:"work-1",AnimeID:"10000000-0000-4000-8000-000000000003"}}
 if _,err=catalogidentity.Apply(ctx,owner,conflict,now,true);!errors.Is(err,catalogidentity.ErrConflict){t.Fatal("conflicting batch accepted",err)}
 if err=owner.QueryRow(ctx,`SELECT count(*) FROM source_records WHERE record_type='IDENTITY_MAP'`).Scan(&n);err!=nil||n!=1{t.Fatal("review rollback",n,err)}
 conflict.Mappings=[]catalogidentity.Mapping{{ExternalID:"missing",AnimeID:"00000000-0000-4000-8000-000000000000"}}
 if _,err=catalogidentity.Apply(ctx,owner,conflict,now,true);!errors.Is(err,catalogidentity.ErrTarget){t.Fatal("missing canonical target accepted",err)}
 // A reviewed mapping lets initial import enrich the existing canonical ID.
 snapshot:=feed.Snapshot{Version:1,Provider:s.Provider,ObservedAt:now,Titles:[]feed.Title{{ExternalID:"work-1",Title:"Synthetic Linked Identity",MediaType:"TV",Status:"UNKNOWN"}}}
 if _,err=catalogsync.Import(ctx,owner,snapshot,now);err!=nil{t.Fatal(err)}
 title,ok,err:=db.Repository("").GetAnime(ctx,one)
 if err!=nil||!ok||title.Title!="Synthetic Linked Identity"{t.Fatal("mapped ID was not reused",title,err)}
 other:=s;other.Provider="identity-other"
 if _,err=catalogidentity.Apply(ctx,owner,other,now,true);err!=nil{t.Fatal("cross-provider reviewed link",err)}
 snapshot.Provider="identity-other";snapshot.Titles[0].Title="Synthetic Forbidden Takeover"
 if _,err=catalogsync.Import(ctx,owner,snapshot,now);!errors.Is(err,catalogsync.ErrConflict){t.Fatal("identity link transferred metadata authority",err)}
 raw,err:=pgx.Connect(ctx,runtimeDSN);if err!=nil{t.Fatal(err)};defer raw.Close(ctx)
 if _,err=catalogidentity.Apply(ctx,raw,s,now,true);!errors.Is(err,catalogidentity.ErrApply){t.Fatal("API role mapped identities",err)}
}
func TestDedicatedIdentityMapperHasNoPrivateOrRewriteAccess(t *testing.T){
 _,owner,_,dsn:=testDB(t);ctx:=context.Background()
 u,err:=url.Parse(dsn);if err!=nil{t.Fatal(err)}
 role:=u.User.Username()+"_mapper";quoted:=pgx.Identifier{role}.Sanitize()
 script,err:=os.ReadFile("../../../deployments/catalog-mapper-role.sql");if err!=nil{t.Fatal(err)}
 if _,err=owner.Exec(ctx,strings.ReplaceAll(string(script),"anime_catalog_mapper",quoted));err!=nil{t.Fatal(err)}
 t.Cleanup(func(){_,_=owner.Exec(ctx,`DROP OWNED BY `+quoted);_,_=owner.Exec(ctx,`DROP ROLE `+quoted)})
 password,_:=u.User.Password()
 if _,err=owner.Exec(ctx,`ALTER ROLE `+quoted+` PASSWORD '`+password+`'`);err!=nil{t.Fatal(err)}
 u.User=url.UserPassword(role,password)
 mapper,err:=pgx.Connect(ctx,u.String());if err!=nil{t.Fatal(err)};t.Cleanup(func(){_=mapper.Close(ctx)})
 if err=catalogsync.CheckWriter(ctx,mapper);err!=nil{t.Fatal("safe mapper rejected",err)}
 if err=catalogsync.CheckWriter(ctx,owner);err==nil{t.Fatal("owner accepted by mapper CLI guard")}
 if _,err=owner.Exec(ctx,`INSERT INTO data_sources(provider_key,display_name,enabled,policy_url) VALUES('mapper-fixture','Synthetic mapper',true,'https://example.test/policy')`);err!=nil{t.Fatal(err)}
 now:=time.Now().UTC().Truncate(time.Microsecond)
 s:=catalogidentity.Snapshot{Version:1,Provider:"mapper-fixture",ReviewedAt:now,Reference:"https://example.test/review",Mappings:[]catalogidentity.Mapping{{ExternalID:"work-1",AnimeID:"10000000-0000-4000-8000-000000000001"}}}
 if result,err:=catalogidentity.Apply(ctx,mapper,s,now,true);err!=nil||result.Linked!=1{t.Fatal("restricted link",result,err)}
 for _,query:=range []string{`SELECT email FROM users`,`SELECT count(*) FROM watch_cycles`,`UPDATE anime_external_ids SET anime_id='10000000-0000-4000-8000-000000000002'`,`DELETE FROM anime_external_ids`,`UPDATE data_sources SET enabled=false`,`UPDATE anime SET canonical_title='rewritten'`,`SELECT payload FROM source_records`}{
  if _,err=mapper.Exec(ctx,query);err==nil{t.Fatal("excess mapper privilege")}
 }
 if _,err=owner.Exec(ctx,`GRANT SELECT(episodes_watched) ON watch_cycles TO `+quoted);err!=nil{t.Fatal(err)}
 if err=catalogsync.CheckWriter(ctx,mapper);err==nil{t.Fatal("private history column grant accepted")}
}
