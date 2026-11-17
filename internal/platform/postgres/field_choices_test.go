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
 "github.com/khordestan/anime-intelligence/internal/platform/catalogsync"
 "github.com/khordestan/anime-intelligence/internal/platform/postgres"
 "github.com/khordestan/anime-intelligence/internal/platform/store"
)

func fieldReviewDB(t *testing.T)(*pgx.Conn,*pgx.Conn,*pgx.Conn,string,string){
 t.Helper()
 owner,writer,_,runtimeDSN:=queuedDB(t)
 ctx:=context.Background()
 u,err:=url.Parse(runtimeDSN);if err!=nil{t.Fatal(err)}
 role:=u.User.Username()+"_fields"
 script,err:=os.ReadFile("../../../deployments/catalog-field-curator-role.sql");if err!=nil{t.Fatal(err)}
 queueExec(t,owner,strings.ReplaceAll(string(script),"anime_catalog_field_curator",pgx.Identifier{role}.Sanitize()))
 t.Cleanup(func(){_,_=owner.Exec(ctx,`DROP OWNED BY `+pgx.Identifier{role}.Sanitize());_,_=owner.Exec(ctx,`DROP ROLE `+pgx.Identifier{role}.Sanitize())})
 password,_:=u.User.Password()
 queueExec(t,owner,`ALTER ROLE `+pgx.Identifier{role}.Sanitize()+` PASSWORD '`+password+`'`)
 u.User=url.UserPassword(role,password)
 curator,err:=pgx.Connect(ctx,u.String());if err!=nil{t.Fatal(err)};t.Cleanup(func(){_=curator.Close(ctx)})
 return owner,writer,curator,u.String(),runtimeDSN
}
func fieldFixture(t *testing.T,owner *pgx.Conn)(string,feed.Snapshot,time.Time){
 t.Helper()
 for _,provider:=range []string{"field-primary","field-secondary"}{
  queueExec(t,owner,`INSERT INTO data_sources(provider_key,display_name,enabled,policy_url) VALUES($1,$1,true,'https://example.test/policy')`,provider)
 }
 base:=time.Now().UTC().Truncate(time.Microsecond).Add(-10*time.Hour)
 s:=feed.Snapshot{Version:1,Provider:"field-primary",ObservedAt:base,Titles:[]feed.Title{{
 ExternalID:"one",Title:"Synthetic Field Title",MediaType:"TV",Status:"UNKNOWN",
 Genres:[]string{"Primary genre"},Studios:[]string{"Primary studio"},Synonyms:[]string{"Primary alias"},
 }}}
 if _,err:=catalogsync.Import(context.Background(),owner,s,time.Now());err!=nil{t.Fatal(err)}
 var id string
 if err:=owner.QueryRow(context.Background(),`SELECT anime_id::text FROM anime_external_ids WHERE provider='field-primary' AND external_id='one'`).Scan(&id);err!=nil{t.Fatal(err)}
 queueExec(t,owner,`INSERT INTO anime_external_ids(anime_id,provider,external_id) VALUES($1,'field-secondary','second-one')`,id)
 return id,s,base
}
func labelReview(id,field string,values []string,revision int,at time.Time)catalogsync.FieldReview{
 return catalogsync.FieldReview{Version:1,ReviewedAt:at,Reference:"https://example.test/field-review",
 Selections:[]catalogsync.FieldSelection{{AnimeID:id,Field:field,Mode:"SELECT",ExpectedRevision:&revision,
 Provider:"field-secondary",ExternalID:"second-one",Values:values,ObservedAt:at.Add(-time.Minute)}}}
}
func TestFieldChoicesPersistAcrossRefreshRestoreAndPublicQueries(t *testing.T){
 owner,writer,curator,_,runtimeDSN:=fieldReviewDB(t)
 ctx:=context.Background()
 id,primary,base:=fieldFixture(t,owner)
 review:=labelReview(id,"genres",[]string{"Selected genre"},0,base.Add(time.Hour))
 alias:=review.Selections[0];alias.Field="synonyms";alias.Values=[]string{"Reviewed alias"}
 review.Selections=append(review.Selections,alias)
 preview,err:=catalogsync.ReviewFields(ctx,curator,review,time.Now(),false)
 if err!=nil||preview.Applied||preview.Selected!=2||preview.AuditRecords!=2{t.Fatal("preview",preview,err)}
 var count int
 if err=owner.QueryRow(ctx,`SELECT count(*) FROM catalog_field_choices`).Scan(&count);err!=nil||count!=0{t.Fatal("preview persisted",count,err)}
 result,err:=catalogsync.ReviewFields(ctx,curator,review,time.Now(),true)
 if err!=nil||!result.Applied||result.Selected!=2{t.Fatal("apply",result,err)}
 replay,err:=catalogsync.ReviewFields(ctx,curator,review,time.Now(),true)
 if err!=nil||replay.Unchanged!=2||replay.AuditRecords!=0{t.Fatal("replay",replay,err)}
 db,err:=postgres.Open(ctx,runtimeDSN);if err!=nil{t.Fatal(err)};defer db.Close()
 repo:=db.Repository("")
 details,ok,err:=repo.CatalogDetails(ctx,id)
 if err!=nil||!ok||details.Provider!="field-primary"||len(details.Genres)!=1||details.Genres[0]!="Selected genre"||len(details.FieldSources)!=2{t.Fatal("public projection",details,err)}
 for _,source:=range details.FieldSources{if source.Provider!="field-secondary"||source.Revision!=1||source.Mode!="SELECT"{t.Fatal("field provenance",source)}}
 matches,err:=repo.FilterCatalog(ctx,store.CatalogFilter{Query:"Reviewed alias",Genre:"Selected genre"},10,0)
 if err!=nil||len(matches)!=1||matches[0].ID!=id{t.Fatal("filters/aliases diverged",matches,err)}
 facets,err:=repo.CatalogFacets(ctx,"genres","Selected",10)
 if err!=nil||len(facets.Items)!=1||facets.Items[0]!="Selected genre"{t.Fatal("facets diverged",facets,err)}
 // A real primary refresh changes the latest retained facts, without undoing
 // the explicit selected fields or attributing secondary labels to the source.
 primary.ObservedAt=base.Add(2*time.Hour)
 primary.Titles[0].Genres=[]string{"Refreshed primary genre"}
 primary.Titles[0].Synonyms=[]string{"Refreshed primary alias"}
 if _,err=catalogsync.Import(ctx,writer,primary,time.Now());err!=nil{t.Fatal("restricted primary refresh",err)}
 details,_,err=repo.CatalogDetails(ctx,id)
 if err!=nil||details.Genres[0]!="Selected genre"||details.Synonyms[0]!="Reviewed alias"{t.Fatal("refresh erased reviewed choice",details,err)}
 status,err:=catalogsync.Inspect(ctx,writer,time.Now(),24*time.Hour)
 if err!=nil||status.Coverage.Genres!=1{t.Fatal("primary diagnostics",status,err)}
 revision:=1
 restore:=catalogsync.FieldReview{Version:1,ReviewedAt:base.Add(3*time.Hour),Reference:"https://example.test/restore",
 Selections:[]catalogsync.FieldSelection{{AnimeID:id,Field:"genres",Mode:"BASE",ExpectedRevision:&revision}}}
 if result,err=catalogsync.ReviewFields(ctx,curator,restore,time.Now(),true);err!=nil||result.Restored!=1{t.Fatal("restore",result,err)}
 details,_,err=repo.CatalogDetails(ctx,id)
 if err!=nil||details.Genres[0]!="Refreshed primary genre"{t.Fatal("restore used stale base",details,err)}
 if details.FieldSources[0].Field!="genres"||details.FieldSources[0].Mode!="BASE"||details.FieldSources[0].Revision!=2||details.FieldSources[0].Provider!="field-primary"{t.Fatal("restore provenance",details.FieldSources)}
 if _,err=catalogsync.ReviewFields(ctx,curator,review,time.Now(),true);!errors.Is(err,catalogsync.ErrFieldConflict){t.Fatal("old review resurrected",err)}
 // An explicit empty label choice remains distinguishable from BASE.
 empty:=labelReview(id,"studios",[]string{},0,base.Add(4*time.Hour))
 if _,err=catalogsync.ReviewFields(ctx,curator,empty,time.Now(),true);err!=nil{t.Fatal("explicit empty selection",err)}
 details,_,err=repo.CatalogDetails(ctx,id)
 if err!=nil||len(details.Studios)!=0{t.Fatal("empty selection ignored",details,err)}
 status,err=catalogsync.Inspect(ctx,writer,time.Now(),24*time.Hour)
 if err!=nil||status.Coverage.Studios!=1{t.Fatal("diagnostics credited projected values to primary",status,err)}
}
func TestFieldChoiceConflictsAuditRollbackAndRoleBoundaries(t *testing.T){
 owner,writer,curator,dsn,runtimeDSN:=fieldReviewDB(t)
 ctx:=context.Background()
 id,_,base:=fieldFixture(t,owner)
 review:=labelReview(id,"genres",[]string{"Selected"},0,base.Add(time.Hour))
 wrong:=review.Selections[0];wrong.Field="studios";wrong.ExternalID="missing";wrong.Values=[]string{"Unmapped"}
 review.Selections=append(review.Selections,wrong)
 if _,err:=catalogsync.ReviewFields(ctx,curator,review,time.Now(),true);!errors.Is(err,catalogsync.ErrFieldConflict){t.Fatal("unmapped decision accepted",err)}
 var n int
 if err:=owner.QueryRow(ctx,`SELECT count(*) FROM catalog_field_choices`).Scan(&n);err!=nil||n!=0{t.Fatal("late batch conflict retained choice",n,err)}
 review.Selections=review.Selections[:1]
 queueExec(t,owner,`UPDATE data_sources SET enabled=false WHERE provider_key='field-secondary'`)
 if _,err:=catalogsync.ReviewFields(ctx,curator,review,time.Now(),true);!errors.Is(err,catalogsync.ErrSource){t.Fatal("unapproved choice accepted",err)}
 queueExec(t,owner,`UPDATE data_sources SET enabled=true WHERE provider_key='field-secondary'`)
 if _,err:=catalogsync.ReviewFields(ctx,owner,review,time.Now(),true);err==nil{t.Fatal("administrator credentials accepted")}
 if _,err:=catalogsync.ReviewFields(ctx,writer,review,time.Now(),true);err==nil{t.Fatal("factual writer curated choices")}
 // Independent curator connections compete for the same expected revision.
 other,err:=pgx.Connect(ctx,dsn);if err!=nil{t.Fatal(err)};defer other.Close(ctx)
 outcomes:=make(chan error,2)
 second:=labelReview(id,"genres",[]string{"Other reviewed value"},0,base.Add(2*time.Hour))
 go func(){_,err:=catalogsync.ReviewFields(ctx,curator,review,time.Now(),true);outcomes<-err}()
 go func(){_,err:=catalogsync.ReviewFields(ctx,other,second,time.Now(),true);outcomes<-err}()
 success,conflict:=0,0
 for range 2{err:=<-outcomes;if err==nil{success++}else if errors.Is(err,catalogsync.ErrFieldConflict){conflict++}else{t.Fatal(err)}}
 if success!=1||conflict!=1{t.Fatal("concurrent reviews",success,conflict)}
 // A late audit failure must also roll back earlier projected fields.
 queueExec(t,owner,`CREATE FUNCTION reject_field_audit() RETURNS trigger LANGUAGE plpgsql AS $$
 BEGIN IF NEW.field='studios' THEN RAISE EXCEPTION 'synthetic field audit failure'; END IF; RETURN NEW; END $$;
 CREATE TRIGGER reject_field_audit BEFORE INSERT ON catalog_field_choice_audits FOR EACH ROW EXECUTE FUNCTION reject_field_audit()`)
 late:=labelReview(id,"themes",[]string{"Synthetic hidden theme"},0,base.Add(3*time.Hour))
 studio:=late.Selections[0];studio.Field="studios";studio.Values=[]string{"Synthetic studio"};late.Selections=append(late.Selections,studio)
 if _,err=catalogsync.ReviewFields(ctx,curator,late,time.Now(),true);!errors.Is(err,catalogsync.ErrFieldApply){t.Fatal("late failure succeeded",err)}
 queueExec(t,owner,`DROP TRIGGER reject_field_audit ON catalog_field_choice_audits; DROP FUNCTION reject_field_audit()`)
 if err=owner.QueryRow(ctx,`SELECT count(*) FROM catalog_field_choices WHERE field IN ('themes','studios')`).Scan(&n);err!=nil||n!=0{t.Fatal("late audit failure retained state",n,err)}
 runtime,err:=pgx.Connect(ctx,runtimeDSN);if err!=nil{t.Fatal(err)};defer runtime.Close(ctx)
 for _,query:=range []string{`SELECT value FROM catalog_field_choices`,`SELECT reference FROM catalog_field_choices`,`SELECT * FROM catalog_field_choice_audits`,`SELECT * FROM catalog_base_metadata`}{
  if _,err=runtime.Exec(ctx,query);err==nil{t.Fatal("API read operator payload",query)}
 }
 for _,query:=range []string{`SELECT email FROM users`,`UPDATE data_sources SET enabled=false`,`UPDATE anime SET cover_image_url='Forbidden'`,`INSERT INTO anime(canonical_title) VALUES('Forbidden')`,`DELETE FROM catalog_field_choices`,`UPDATE catalog_field_choice_audits SET payload='{}'`,`UPDATE catalog_base_metadata SET payload='{}'`,`UPDATE airing_schedule SET schedule_status='UNKNOWN'`}{
  if _,err=curator.Exec(ctx,query);err==nil{t.Fatal("curator exceeded boundary",query)}
 }
 if _,err=writer.Exec(ctx,`UPDATE catalog_metadata SET payload=payload`);err==nil{t.Fatal("old binary bypassed projection context")}
 // Valid JSON alone is insufficient: a corrupt retained primary observation
 // cannot be laundered into a resolved payload by a label-field review.
 queueExec(t,owner,`UPDATE catalog_base_metadata SET payload=payload-'externalId' WHERE anime_id=$1`,id)
 corrupt:=labelReview(id,"demographics",[]string{"Synthetic audience"},0,base.Add(4*time.Hour))
 if _,err=catalogsync.ReviewFields(ctx,curator,corrupt,time.Now(),true);!errors.Is(err,catalogsync.ErrFieldApply){t.Fatal("corrupt base was reprojected",err)}
 if err=owner.QueryRow(ctx,`SELECT count(*) FROM catalog_field_choices WHERE field='demographics'`).Scan(&n);err!=nil||n!=0{t.Fatal("corrupt-base failure retained choice",n,err)}

}
func TestFieldChoiceMigrationRollbackPreservesDecisions(t *testing.T){
 owner,_,curator,_,_:=fieldReviewDB(t)
 ctx:=context.Background()
 id,_,base:=fieldFixture(t,owner)
 review:=labelReview(id,"genres",[]string{"Reviewed"},0,base.Add(time.Hour))
 if _,err:=catalogsync.ReviewFields(ctx,curator,review,time.Now(),true);err!=nil{t.Fatal(err)}
 down,err:=os.ReadFile("../../../migrations/000019_catalog_field_choices.down.sql");if err!=nil{t.Fatal(err)}
 if _,err=owner.Exec(ctx,string(down));err==nil{t.Fatal("downgrade erased review")}
 var n int
 if err=owner.QueryRow(ctx,`SELECT count(*) FROM catalog_field_choice_audits`).Scan(&n);err!=nil||n!=1{t.Fatal("failed downgrade lost audit",n,err)}
 // A restored-to-base decision still has a concurrency/history contract.
 revision:=1
 review.Selections=[]catalogsync.FieldSelection{{AnimeID:id,Field:"genres",Mode:"BASE",ExpectedRevision:&revision}}
 review.ReviewedAt=base.Add(2*time.Hour)
 if _,err=catalogsync.ReviewFields(ctx,curator,review,time.Now(),true);err!=nil{t.Fatal(err)}
 if _,err=owner.Exec(ctx,string(down));err==nil{t.Fatal("downgrade erased base revision history")}
}
