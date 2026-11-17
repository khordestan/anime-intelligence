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
 "github.com/khordestan/anime-intelligence/internal/platform/postgres"
)

func identityRetractor(t *testing.T,owner *pgx.Conn,runtimeDSN string)(*pgx.Conn,string){
 t.Helper();ctx:=context.Background()
 u,err:=url.Parse(runtimeDSN);if err!=nil{t.Fatal(err)}
 role:=u.User.Username()+"_retract";quoted:=pgx.Identifier{role}.Sanitize()
 script,err:=os.ReadFile("../../../deployments/catalog-retractor-role.sql");if err!=nil{t.Fatal(err)}
 queueExec(t,owner,strings.ReplaceAll(string(script),"anime_catalog_retractor",quoted))
 t.Cleanup(func(){_,_=owner.Exec(ctx,"DROP OWNED BY "+quoted);_,_=owner.Exec(ctx,"DROP ROLE "+quoted)})
 password,_:=u.User.Password()
 queueExec(t,owner,"ALTER ROLE "+quoted+" PASSWORD '"+password+"'")
 u.User=url.UserPassword(role,password)
 conn,err:=pgx.Connect(ctx,u.String());if err!=nil{t.Fatal(err)}
 t.Cleanup(func(){_=conn.Close(ctx)})
 if catalogsync.CheckWriter(ctx,conn)!=nil{t.Fatal("safe retractor rejected")}
 return conn,u.String()
}
func identityRetraction(t *testing.T,owner *pgx.Conn,id string,at time.Time)catalogidentity.RetractionReview{
 t.Helper();var mapping string
 if err:=owner.QueryRow(context.Background(),"SELECT id::text FROM anime_external_ids WHERE anime_id=$1 AND provider='field-secondary'",id).Scan(&mapping);err!=nil{t.Fatal(err)}
 return catalogidentity.RetractionReview{Version:1,Provider:"field-secondary",ReviewedAt:at,Reference:"https://example.test/identity-retirement",
 Retractions:[]catalogidentity.Retraction{{ExternalID:"Q99",AnimeID:id,ExpectedMappingID:mapping}}}
}
func TestIdentityRetractionPreviewReplayAndImportFence(t *testing.T){
 owner,writer,_,_,runtimeDSN:=fieldReviewDB(t)
 ctx:=context.Background()
 id,_,base:=fieldFixture(t,owner)
 queueExec(t,owner,"UPDATE anime_external_ids SET external_id='Q99' WHERE anime_id=$1 AND provider='field-secondary'",id)
 retractor,dsn:=identityRetractor(t,owner,runtimeDSN)
 review:=identityRetraction(t,owner,id,base.Add(time.Hour))
 stale:=review;stale.Retractions=append([]catalogidentity.Retraction(nil),review.Retractions...);stale.Retractions[0].ExpectedMappingID="00000000-0000-4000-8000-000000000000"
 if _,err:=catalogidentity.Retract(ctx,retractor,stale,time.Now(),true);!errors.Is(err,catalogidentity.ErrConflict){t.Fatal("stale mapping UUID accepted",err)}
 if result,err:=catalogidentity.Retract(ctx,retractor,review,time.Now(),false);err!=nil||result.Applied||result.Retracted!=1||result.AuditRecords!=1{t.Fatal("retraction preview",result,err)}
 var n int
 if err:=owner.QueryRow(ctx,"SELECT count(*) FROM catalog_identity_retractions").Scan(&n);err!=nil||n!=0{t.Fatal("preview persisted tombstone",n,err)}
 var found bool
 if err:=owner.QueryRow(ctx,"SELECT EXISTS(SELECT 1 FROM anime_external_ids WHERE id=$1)",review.Retractions[0].ExpectedMappingID).Scan(&found);err!=nil||!found{t.Fatal("preview removed link",err)}
 // Corrections remain possible after approval is withdrawn.
 queueExec(t,owner,"UPDATE data_sources SET enabled=false WHERE provider_key='field-secondary'")
 other,err:=pgx.Connect(ctx,dsn);if err!=nil{t.Fatal(err)};defer other.Close(ctx)
 type outcome struct{result catalogidentity.RetractionResult;err error}
 outcomes:=make(chan outcome,2)
 for _,conn:=range []*pgx.Conn{retractor,other}{
  go func(c *pgx.Conn){result,err:=catalogidentity.Retract(ctx,c,review,time.Now(),true);outcomes<-outcome{result,err}}(conn)
 }
 removed,replayed:=0,0
 for range 2{out:=<-outcomes;if out.err!=nil{t.Fatal(out.err)};removed+=out.result.Retracted;replayed+=out.result.Unchanged}
 if removed!=1||replayed!=1{t.Fatal("concurrent retraction/replay",removed,replayed)}
 if err=owner.QueryRow(ctx,"SELECT count(*) FROM catalog_identity_retractions").Scan(&n);err!=nil||n!=1{t.Fatal("duplicate tombstone",n,err)}
 db,err:=postgres.Open(ctx,runtimeDSN);if err!=nil{t.Fatal(err)};defer db.Close()
 if _,ok,err:=db.Repository("").GetAnimeByExternalID(ctx,"field-secondary","Q99");err!=nil||ok{t.Fatal("retired lookup still resolves",ok,err)}
 details,ok,err:=db.Repository("").CatalogDetails(ctx,id)
 if err!=nil||!ok||details.Provider!="field-primary"||len(details.ExternalIDs)!=1{t.Fatal("retraction damaged canonical title",details,err)}
 different:=review;different.Reference="https://example.test/different-review"
 if _,err=catalogidentity.Retract(ctx,retractor,different,time.Now(),true);!errors.Is(err,catalogidentity.ErrConflict){t.Fatal("changed replay accepted",err)}
 queueExec(t,owner,"UPDATE data_sources SET enabled=true WHERE provider_key='field-secondary'")
 mapping:=catalogidentity.Snapshot{Version:1,Provider:review.Provider,ReviewedAt:review.ReviewedAt,Reference:review.Reference,
 Mappings:[]catalogidentity.Mapping{{ExternalID:"Q99",AnimeID:id}}}
 if _,err=catalogidentity.Apply(ctx,owner,mapping,time.Now(),true);!errors.Is(err,catalogidentity.ErrConflict){t.Fatal("mapper resurrected retired ID",err)}
 before:=0
 if err=owner.QueryRow(ctx,"SELECT count(*) FROM anime").Scan(&before);err!=nil{t.Fatal(err)}
 snapshot:=feed.Snapshot{Version:1,Provider:review.Provider,ObservedAt:base.Add(2*time.Hour),Titles:[]feed.Title{{ExternalID:"Q99",Title:"Must not reappear",MediaType:"TV",Status:"UNKNOWN"}}}
 if _,err=catalogsync.Import(ctx,writer,snapshot,time.Now());err==nil{t.Fatal("import resurrected retired ID")}
 if err=owner.QueryRow(ctx,"SELECT count(*) FROM anime").Scan(&n);err!=nil||n!=before{t.Fatal("failed import retained orphan title",n,err)}
 if err=owner.QueryRow(ctx,"SELECT count(*) FROM catalog_sync_state s JOIN data_sources d ON d.id=s.source_id WHERE d.provider_key='field-secondary'").Scan(&n);err!=nil||n!=0{t.Fatal("failed import advanced watermark",n,err)}
 for _,query:=range []string{
  "UPDATE catalog_identity_retractions SET reference='rewritten'",
  "DELETE FROM catalog_identity_retractions","SELECT email FROM users",
  "UPDATE anime SET canonical_title='rewritten'","UPDATE data_sources SET enabled=false",
  "UPDATE airing_schedule SET schedule_status='UNKNOWN'","INSERT INTO anime(canonical_title) VALUES('Forbidden')",
 }{if _,err=retractor.Exec(ctx,query);err==nil{t.Fatal("retractor exceeded privileges",query)}}
 raw,err:=pgx.Connect(ctx,runtimeDSN);if err!=nil{t.Fatal(err)};defer raw.Close(ctx)
 if _,err=raw.Exec(ctx,"SELECT reference FROM catalog_identity_retractions");err==nil{t.Fatal("runtime read operator retractions")}
 down,err:=os.ReadFile("../../../migrations/000022_catalog_identity_retractions.down.sql");if err!=nil{t.Fatal(err)}
 if _,err=owner.Exec(ctx,string(down));err==nil{t.Fatal("downgrade erased retirement fence")}
}

func TestIdentityRetractionDependenciesAndBatchRollback(t *testing.T){
 for _,kind:=range []string{"primary","schedule-watermark","episode","relation","queue","archive","retained-claim","restored-field"}{
  t.Run(kind,func(t *testing.T){
   owner,_,curator,_,runtimeDSN:=fieldReviewDB(t);ctx:=context.Background()
   id,_,base:=fieldFixture(t,owner)
   queueExec(t,owner,"UPDATE anime_external_ids SET external_id='Q99' WHERE anime_id=$1 AND provider='field-secondary'",id)
   retractor,_:=identityRetractor(t,owner,runtimeDSN)
   review:=identityRetraction(t,owner,id,base.Add(3*time.Hour))
   var source string
   if err:=owner.QueryRow(ctx,"SELECT id::text FROM data_sources WHERE provider_key='field-secondary'").Scan(&source);err!=nil{t.Fatal(err)}
   switch kind{
   case "primary":
    review.Provider="field-primary";review.Retractions[0].ExternalID="one"
    if err:=owner.QueryRow(ctx,"SELECT id::text FROM anime_external_ids WHERE anime_id=$1 AND provider='field-primary'",id).Scan(&review.Retractions[0].ExpectedMappingID);err!=nil{t.Fatal(err)}
   case "schedule-watermark":
    queueExec(t,owner,"INSERT INTO catalog_schedule_observations(source_id,anime_id,observed_at) VALUES($1,$2,$3)",source,id,base)
   case "episode":
    queueExec(t,owner,"INSERT INTO airing_schedule(anime_id,episode_number,release_track,source_id) VALUES($1,1,'UNKNOWN',$2)",id,source)
   case "relation":
    queueExec(t,owner,"INSERT INTO anime_relations(source_anime_id,target_anime_id,relation_type,source_id) VALUES($1,'10000000-0000-4000-8000-000000000001','SIDE_STORY',$2)",id,source)
   case "queue":
    queueExec(t,owner,"INSERT INTO catalog_fetch_jobs(source_id,external_id,interval_seconds,next_attempt_at,updated_at) VALUES($1,'Q99',3600,$2,$2)",source,base)
   case "archive":
    queueExec(t,owner,"INSERT INTO source_records(source_id,external_id,record_type,payload,content_hash) VALUES($1,'Q99','METADATA','{}',repeat('a',64))",source)
   case "retained-claim":
    queueExec(t,owner,`INSERT INTO catalog_base_metadata(anime_id,source_id,observed_at,payload) VALUES('10000000-0000-4000-8000-000000000001',$1,$2,'{"relations":[{"externalId":"Q99","type":"SEQUEL"}]}')`,source,base)
   case "restored-field":
    selected:=labelReview(id,"genres",[]string{"Synthetic reviewed"},0,base.Add(time.Hour));selected.Selections[0].ExternalID="Q99"
    if _,err:=catalogsync.ReviewFields(ctx,curator,selected,time.Now(),true);err!=nil{t.Fatal(err)}
    one:=1;selected.ReviewedAt=base.Add(2*time.Hour)
    selected.Selections=[]catalogsync.FieldSelection{{AnimeID:id,Field:"genres",Mode:"BASE",ExpectedRevision:&one}}
    if _,err:=catalogsync.ReviewFields(ctx,curator,selected,time.Now(),true);err!=nil{t.Fatal(err)}
   }
   if _,err:=catalogidentity.Retract(ctx,retractor,review,time.Now(),true);!errors.Is(err,catalogidentity.ErrConflict){t.Fatal("used identity retired",kind,err)}
   var n int
   if err:=owner.QueryRow(ctx,"SELECT count(*) FROM catalog_identity_retractions").Scan(&n);err!=nil||n!=0{t.Fatal("dependency failure retained tombstone",n,err)}
  })
 }
 owner,_,_,_,runtimeDSN:=fieldReviewDB(t);ctx:=context.Background()
 id,_,base:=fieldFixture(t,owner)
 queueExec(t,owner,"UPDATE anime_external_ids SET external_id='Q99' WHERE anime_id=$1 AND provider='field-secondary'",id)
 retractor,_:=identityRetractor(t,owner,runtimeDSN)
 review:=identityRetraction(t,owner,id,base.Add(time.Hour))
 review.Retractions=append(review.Retractions,catalogidentity.Retraction{ExternalID:"missing",AnimeID:"10000000-0000-4000-8000-000000000001",ExpectedMappingID:"00000000-0000-4000-8000-000000000000"})
 if _,err:=catalogidentity.Retract(ctx,retractor,review,time.Now(),true);!errors.Is(err,catalogidentity.ErrConflict){t.Fatal("late conflict accepted",err)}
 var n int
 if err:=owner.QueryRow(ctx,"SELECT count(*) FROM catalog_identity_retractions").Scan(&n);err!=nil||n!=0{t.Fatal("late conflict retained retirement",n,err)}
 if err:=owner.QueryRow(ctx,"SELECT count(*) FROM anime_external_ids WHERE provider='field-secondary' AND external_id='Q99'").Scan(&n);err!=nil||n!=1{t.Fatal("late conflict removed earlier link",n,err)}
}
