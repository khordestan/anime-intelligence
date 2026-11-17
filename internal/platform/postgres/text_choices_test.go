package postgres_test

import (
 "context"
 "encoding/json"
 "errors"
 "os"
 "testing"
 "time"

 "github.com/khordestan/anime-intelligence/internal/platform/catalogsync"
 "github.com/khordestan/anime-intelligence/internal/platform/postgres"
 "github.com/khordestan/anime-intelligence/internal/platform/store"
)

func textReview(id,field,value string,revision int,at time.Time)catalogsync.FieldReview{
 raw,_:=json.Marshal(value)
 return scalarReview(id,field,string(raw),revision,at)
}
func TestTextChoicesRefreshRestoreSearchAndAuditRollback(t *testing.T){
 owner,writer,curator,_,runtimeDSN:=fieldReviewDB(t)
 ctx:=context.Background()
 id,primary,base:=fieldFixture(t,owner)
 review:=textReview(id,"title","Reviewed synthetic name",0,base.Add(time.Hour))
 for _,tc:=range []struct{field,value string}{{"nativeTitle","架空の名前"},{"synopsis","Reviewed paragraph.\nSecond paragraph."},{"status","HIATUS"}}{
  review.Selections=append(review.Selections,textReview(id,tc.field,tc.value,0,review.ReviewedAt).Selections[0])
 }
 db,err:=postgres.Open(ctx,runtimeDSN);if err!=nil{t.Fatal(err)};defer db.Close()
 repo:=db.Repository("")
 if result,err:=catalogsync.ReviewFields(ctx,curator,review,time.Now(),false);err!=nil||result.Applied||result.Selected!=4{t.Fatal("text preview",result,err)}
 details,_,err:=repo.CatalogDetails(ctx,id)
 if err!=nil||details.Anime.Title!="Synthetic Field Title"||len(details.FieldSources)!=0{t.Fatal("preview persisted",details,err)}
 if _,err=catalogsync.ReviewFields(ctx,curator,review,time.Now(),true);err!=nil{t.Fatal("text apply",err)}
 review.Selections[0].Scalar=json.RawMessage("\"Reviewed synthetic nam\\u0065\"")
 if result,err:=catalogsync.ReviewFields(ctx,curator,review,time.Now(),true);err!=nil||result.Unchanged!=4||result.AuditRecords!=0{t.Fatal("text replay",result,err)}
 details,_,err=repo.CatalogDetails(ctx,id)
 if err!=nil||details.Anime.Title!="Reviewed synthetic name"||details.Anime.NativeTitle!="架空の名前"||details.Anime.Synopsis!="Reviewed paragraph.\nSecond paragraph."||details.Anime.Status!="HIATUS"||len(details.FieldSources)!=4{t.Fatal("text projection",details,err)}
 for _,source:=range details.FieldSources{if source.Provider!="field-secondary"{t.Fatal("text attribution",source)}}
 matches,err:=repo.FilterCatalog(ctx,store.CatalogFilter{Query:"Reviewed synthetic name",Status:"HIATUS"},10,0)
 if err!=nil||len(matches)!=1||matches[0].ID!=id{t.Fatal("name/status query mismatch",matches,err)}
 primary.ObservedAt=base.Add(2*time.Hour)
 primary.Titles[0].Title="Newest primary name";primary.Titles[0].NativeTitle="Newest native"
 primary.Titles[0].Synopsis="Newest primary prose";primary.Titles[0].Status="FINISHED"
 if _,err=catalogsync.Import(ctx,writer,primary,time.Now());err!=nil{t.Fatal("text refresh",err)}
 details,_,err=repo.CatalogDetails(ctx,id)
 if err!=nil||details.Anime.Title!="Reviewed synthetic name"||details.Anime.Status!="HIATUS"{t.Fatal("refresh erased text choices",details,err)}
 queueExec(t,owner,`CREATE FUNCTION reject_text_audit() RETURNS trigger LANGUAGE plpgsql AS $$
 BEGIN IF NEW.field='synopsis' THEN RAISE EXCEPTION 'synthetic audit failure'; END IF; RETURN NEW; END $$;
 CREATE TRIGGER reject_text_audit BEFORE INSERT ON catalog_field_choice_audits FOR EACH ROW EXECUTE FUNCTION reject_text_audit()`)
 late:=textReview(id,"title","Must roll back",1,base.Add(3*time.Hour))
 late.Selections=append(late.Selections,textReview(id,"synopsis","Must roll back",1,late.ReviewedAt).Selections[0])
 if _,err=catalogsync.ReviewFields(ctx,curator,late,time.Now(),true);!errors.Is(err,catalogsync.ErrFieldApply){t.Fatal("audit failure accepted",err)}
 queueExec(t,owner,`DROP TRIGGER reject_text_audit ON catalog_field_choice_audits; DROP FUNCTION reject_text_audit()`)
 details,_,err=repo.CatalogDetails(ctx,id)
 if err!=nil||details.Anime.Title!="Reviewed synthetic name"{t.Fatal("failed batch retained name",details,err)}
 var n int
 if err=owner.QueryRow(ctx,`SELECT count(*) FROM catalog_field_choice_audits WHERE anime_id=$1`,id).Scan(&n);err!=nil||n!=4{t.Fatal("failed batch retained audits",n,err)}
 one:=1
 restore:=catalogsync.FieldReview{Version:1,ReviewedAt:base.Add(4*time.Hour),Reference:"https://example.test/text-restore"}
 for _,field:=range []string{"title","nativeTitle","synopsis","status"}{
  restore.Selections=append(restore.Selections,catalogsync.FieldSelection{AnimeID:id,Field:field,Mode:"BASE",ExpectedRevision:&one})
 }
 if result,err:=catalogsync.ReviewFields(ctx,curator,restore,time.Now(),true);err!=nil||result.Restored!=4{t.Fatal("text restore",result,err)}
 details,_,err=repo.CatalogDetails(ctx,id)
 if err!=nil||details.Anime.Title!="Newest primary name"||details.Anime.NativeTitle!="Newest native"||details.Anime.Synopsis!="Newest primary prose"||details.Anime.Status!="FINISHED"{t.Fatal("restore stale text",details,err)}
 if _,err=catalogsync.ReviewFields(ctx,curator,review,time.Now(),true);!errors.Is(err,catalogsync.ErrFieldConflict){t.Fatal("stale text resurrected",err)}
 down,err:=os.ReadFile("../../../migrations/000021_catalog_text_choices.down.sql");if err!=nil{t.Fatal(err)}
 if _,err=owner.Exec(ctx,string(down));err==nil{t.Fatal("downgrade erased BASE text history")}
 empty:=textReview(id,"nativeTitle","",2,base.Add(5*time.Hour))
 empty.Selections=append(empty.Selections,textReview(id,"synopsis","",2,empty.ReviewedAt).Selections[0])
 if _,err=catalogsync.ReviewFields(ctx,curator,empty,time.Now(),true);err!=nil{t.Fatal("empty optional text",err)}
 details,_,err=repo.CatalogDetails(ctx,id)
 if err!=nil||details.Anime.NativeTitle!=""||details.Anime.Synopsis!=""{t.Fatal("empty text not projected",details,err)}
 if _,err=owner.Exec(ctx,string(down));err==nil{t.Fatal("downgrade erased selected/restored text history")}
}

func TestReviewedStatusDoesNotCreateOrSuppressPrimaryAnnouncements(t *testing.T){
 for _,tc:=range []struct{base,selected string;want int}{
  {"UNKNOWN","ANNOUNCED",1},
  {"FINISHED","UNKNOWN",0},
 }{
  t.Run(tc.base+"_"+tc.selected,func(t *testing.T){
   owner,writer,curator,_,runtimeDSN:=fieldReviewDB(t)
   ctx:=context.Background()
   id,primary,base:=fieldFixture(t,owner)
   primary.ObservedAt=base.Add(time.Hour);primary.Titles[0].Status=tc.base
   if _,err:=catalogsync.Import(ctx,writer,primary,time.Now());err!=nil{t.Fatal(err)}
   review:=textReview(id,"status",tc.selected,0,base.Add(2*time.Hour))
   if _,err:=catalogsync.ReviewFields(ctx,curator,review,time.Now(),true);err!=nil{t.Fatal(err)}
   var n int
   if err:=owner.QueryRow(ctx,`SELECT count(*) FROM domain_events WHERE aggregate_id=$1`,id).Scan(&n);err!=nil||n!=0{t.Fatal("review emitted event",n,err)}
   primary.ObservedAt=base.Add(3*time.Hour);primary.Titles[0].Status="ANNOUNCED"
   result,err:=catalogsync.Import(ctx,writer,primary,time.Now())
   if err!=nil||result.SeasonAnnouncements!=tc.want{t.Fatal("primary event baseline contaminated",result,err)}
   primary.ObservedAt=base.Add(4*time.Hour)
   result,err=catalogsync.Import(ctx,writer,primary,time.Now())
   if err!=nil||result.SeasonAnnouncements!=0{t.Fatal("repeat announcement",result,err)}
   db,err:=postgres.Open(ctx,runtimeDSN);if err!=nil{t.Fatal(err)};defer db.Close()
   details,_,err:=db.Repository("").CatalogDetails(ctx,id)
   if err!=nil||details.Anime.Status!=tc.selected{t.Fatal("event logic overwrote display choice",details,err)}
   if err=owner.QueryRow(ctx,`SELECT count(*) FROM domain_events WHERE aggregate_id=$1 AND event_type='SEASON_ANNOUNCED'`,id).Scan(&n);err!=nil||n!=tc.want{t.Fatal("event ledger",n,err)}
  })
 }
}
