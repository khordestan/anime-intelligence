package postgres_test

import (
 "context"
 "errors"
 "os"
 "testing"
 "time"

 "github.com/jackc/pgx/v5"
 "github.com/khordestan/anime-intelligence/internal/modules/catalog/feed"
 "github.com/khordestan/anime-intelligence/internal/platform/catalogsync"
)

func scopeFixture(t *testing.T,owner *pgx.Conn)(string,string,time.Time){
 t.Helper()
 ctx:=context.Background()
 for _,provider:=range []string{"scope-primary","scope-secondary","scope-third"}{
  queueExec(t,owner,`INSERT INTO data_sources(provider_key,display_name,enabled,policy_url) VALUES($1,$1,true,'https://example.test/review')`,provider)
 }
 base:=time.Now().UTC().Truncate(time.Microsecond).Add(-12*time.Hour)
 total:=12
 s:=feed.Snapshot{Version:1,Provider:"scope-primary",ObservedAt:base,Titles:[]feed.Title{
  {ExternalID:"one",Title:"Synthetic Authoritative One",Synopsis:"Synthetic primary synopsis",MediaType:"TV",Status:"RELEASING",EpisodeCount:&total,Genres:[]string{"Adventure"}},
  {ExternalID:"two",Title:"Synthetic Authoritative Two",MediaType:"TV",Status:"UNKNOWN"},
 }}
 if _,err:=catalogsync.Import(ctx,owner,s,time.Now());err!=nil{t.Fatal(err)}
 var one,two string
 if err:=owner.QueryRow(ctx,`SELECT anime_id::text FROM anime_external_ids WHERE provider='scope-primary' AND external_id='one'`).Scan(&one);err!=nil{t.Fatal(err)}
 if err:=owner.QueryRow(ctx,`SELECT anime_id::text FROM anime_external_ids WHERE provider='scope-primary' AND external_id='two'`).Scan(&two);err!=nil{t.Fatal(err)}
 for _,provider:=range []string{"scope-secondary","scope-third"}{
  queueExec(t,owner,`INSERT INTO anime_external_ids(anime_id,provider,external_id) VALUES($1,$2,'one'),($3,$2,'two')`,one,provider,two)
 }
 return one,two,base
}
func scopeSnapshot(provider,external string,at time.Time,episodes ...feed.Episode)feed.Snapshot{
 return feed.Snapshot{Version:1,Provider:provider,ObservedAt:at,Titles:[]feed.Title{{
 ExternalID:external,Title:"Synthetic ignored name",Synopsis:"Synthetic ignored secondary synopsis",MediaType:"UNKNOWN",Status:"ANNOUNCED",
 Genres:[]string{"Ignored secondary category"},Episodes:episodes,
 }}}
}
func TestScheduleOnlyImportPreservesMetadataAndPerTitleBaselines(t *testing.T){
 db,owner,_,_:=testDB(t)
 ctx:=context.Background()
 one,_,base:=scopeFixture(t,owner)
 firstRelease:=base.Add(30*time.Minute)
 s:=scopeSnapshot("scope-secondary","one",base.Add(time.Hour),feed.Episode{Number:1,Track:"SUB",ReleasedAt:&firstRelease})
 s.Titles[0].Relations=[]feed.Relation{{ExternalID:"two",Type:"SEQUEL"}}
 opts:=catalogsync.ImportOptions{SchedulesOnly:true,Preview:true}
 preview,err:=catalogsync.ImportWithOptions(ctx,owner,s,time.Now(),opts)
 if err!=nil||!preview.Preview||!preview.SchedulesOnly||preview.ScheduleRevisions!=1||preview.ReleaseEvents!=0{t.Fatal("scoped preview",preview,err)}
 var count int
 for _,table:=range []string{"airing_schedule","catalog_schedule_observations"}{
  if err=owner.QueryRow(ctx,`SELECT count(*) FROM `+pgx.Identifier{table}.Sanitize()).Scan(&count);err!=nil||count!=0{t.Fatal("preview persisted",table,count,err)}
 }
 opts.Preview=false
 result,err:=catalogsync.ImportWithOptions(ctx,owner,s,time.Now(),opts)
 if err!=nil||result.ReleaseEvents!=0||result.SeasonAnnouncements!=0||!result.SchedulesOnly{t.Fatal("first title was not baselined",result,err)}
 // An unrelated title advances the source watermark, but not this title's
 // observation. Its later-confirmed episode must compare its own baseline.
 unrelated:=scopeSnapshot("scope-secondary","two",base.Add(2*time.Hour))
 if _,err=catalogsync.ImportWithOptions(ctx,owner,unrelated,time.Now(),opts);err!=nil{t.Fatal(err)}
 secondRelease:=base.Add(90*time.Minute)
 s.ObservedAt=base.Add(3*time.Hour)
 s.Titles[0].Episodes=append(s.Titles[0].Episodes,feed.Episode{Number:2,Track:"SUB",ReleasedAt:&secondRelease})
 result,err=catalogsync.ImportWithOptions(ctx,owner,s,time.Now(),opts)
 if err!=nil||result.ReleaseEvents!=1{t.Fatal("per-title release comparison",result,err)}
 s.ObservedAt=base.Add(4*time.Hour)
 corrected:=secondRelease.Add(time.Minute);s.Titles[0].Episodes[1].ReleasedAt=&corrected
 result,err=catalogsync.ImportWithOptions(ctx,owner,s,time.Now(),opts)
 if err!=nil||result.ReleaseEvents!=0{t.Fatal("correction duplicated event",result,err)}
 s.ObservedAt=base.Add(5*time.Hour);s.Titles[0].Episodes[1].ReleasedAt=nil
 result,err=catalogsync.ImportWithOptions(ctx,owner,s,time.Now(),opts)
 if err!=nil||result.ReleaseEvents!=0{t.Fatal("missing confirmation retracted release",result,err)}
 details,ok,err:=db.Repository("").CatalogDetails(ctx,one)
 if err!=nil||!ok||details.Provider!="scope-primary"||details.Anime.Title!="Synthetic Authoritative One"||
 details.Anime.Synopsis!="Synthetic primary synopsis"||details.Anime.EpisodeCount!=12||len(details.Genres)!=1||details.Genres[0]!="Adventure"||len(details.Relations)!=0{t.Fatal("secondary overwrote metadata",details,err)}
 if len(details.Episodes)!=2||details.Episodes[0].Provider!="scope-secondary"||details.Episodes[1].ReleasedAt==nil{t.Fatal("schedule provenance/confirmation lost",details.Episodes)}
 if err=owner.QueryRow(ctx,`SELECT count(*) FROM catalog_metadata m JOIN data_sources s ON s.id=m.source_id WHERE s.provider_key='scope-secondary'`).Scan(&count);err!=nil||count!=0{t.Fatal("secondary claimed metadata authority",count,err)}
 if err=owner.QueryRow(ctx,`SELECT count(*) FROM catalog_schedule_observations`).Scan(&count);err!=nil||count!=2{t.Fatal("independent observations missing",count,err)}
 if _,err=catalogsync.ImportWithOptions(ctx,owner,s,time.Now(),opts);!errors.Is(err,catalogsync.ErrStale){t.Fatal("stale scoped observation accepted",err)}
 opts.Archive=true
 if _,err=catalogsync.ImportWithOptions(ctx,nil,s,time.Now(),opts);!errors.Is(err,feed.ErrInvalid){t.Fatal("ignored metadata archive accepted",err)}
}
func TestScheduleOnlyConflictsRollBackTheWholeBatch(t *testing.T){
 _,owner,_,_:=testDB(t)
 ctx:=context.Background()
 one,two,base:=scopeFixture(t,owner)
 opts:=catalogsync.ImportOptions{SchedulesOnly:true}
 s:=scopeSnapshot("scope-secondary","one",base.Add(time.Hour),feed.Episode{Number:1,Track:"UNKNOWN"})
 missing:=s.Titles[0];missing.ExternalID="unmapped";s.Titles=append(s.Titles,missing)
 if _,err:=catalogsync.ImportWithOptions(ctx,owner,s,time.Now(),opts);!errors.Is(err,catalogsync.ErrConflict){t.Fatal("unmapped identity created",err)}
 var count int
 for _,table:=range []string{"airing_schedule","catalog_schedule_observations"}{
  if err:=owner.QueryRow(ctx,`SELECT count(*) FROM `+pgx.Identifier{table}.Sanitize()).Scan(&count);err!=nil||count!=0{t.Fatal("late conflict retained writes",table,count,err)}
 }
 s.Titles=s.Titles[:1];s.Titles[0].Episodes[0].Number=13
 if _,err:=catalogsync.ImportWithOptions(ctx,owner,s,time.Now(),opts);!errors.Is(err,catalogsync.ErrConflict){t.Fatal("known episode total exceeded",err)}
 s.Titles[0].Episodes[0].Number=1
 if _,err:=catalogsync.ImportWithOptions(ctx,owner,s,time.Now(),opts);err!=nil{t.Fatal(err)}
 rival:=scopeSnapshot("scope-third","one",base.Add(2*time.Hour),feed.Episode{Number:1,Track:"UNKNOWN"})
 if _,err:=catalogsync.ImportWithOptions(ctx,owner,rival,time.Now(),opts);!errors.Is(err,catalogsync.ErrConflict){t.Fatal("schedule authority transferred",err)}
 if err:=owner.QueryRow(ctx,`SELECT count(*) FROM catalog_schedule_observations o JOIN data_sources s ON s.id=o.source_id WHERE s.provider_key='scope-third'`).Scan(&count);err!=nil||count!=0{t.Fatal("conflicting source retained observation",count,err)}
 // Primary metadata cannot invalidate retained schedules from another source.
 zero:=0
 primary:=scopeSnapshot("scope-primary","one",base.Add(3*time.Hour));primary.Titles[0].EpisodeCount=&zero
 if _,err:=catalogsync.Import(ctx,owner,primary,time.Now());!errors.Is(err,catalogsync.ErrConflict){t.Fatal("metadata invalidated foreign schedules",err)}
 queueExec(t,owner,`UPDATE anime SET episode_count=0 WHERE id=$1`,two)
 empty:=scopeSnapshot("scope-secondary","two",base.Add(4*time.Hour),feed.Episode{Number:1,Track:"SUB"})
 if _,err:=catalogsync.ImportWithOptions(ctx,owner,empty,time.Now(),opts);!errors.Is(err,catalogsync.ErrConflict){t.Fatal("known zero treated as unknown",err)}
 var total int
 if err:=owner.QueryRow(ctx,`SELECT episode_count FROM anime WHERE id=$1`,one).Scan(&total);err!=nil||total!=12{t.Fatal("failed metadata import retained changes",total,err)}
}
type scheduleQueueFetcher struct{calls int;before func()}
func(f *scheduleQueueFetcher)Fetch(_ context.Context,ids []string)(feed.Snapshot,error){
 f.calls++;if f.before!=nil{f.before()}
 at:=time.Now().UTC();past:=at.Add(-time.Hour)
 return scopeSnapshot("tvmaze",ids[0],at,feed.Episode{Number:1,Track:"UNKNOWN",ScheduledAt:&past}),nil
}
func TestScheduleOnlyQueuePinsModeMappingAndAtomicAcknowledgement(t *testing.T){
 owner,writer,_,runtimeDSN:=queuedDB(t)
 ctx:=context.Background()
 one,two,_:=scopeFixture(t,owner)
 queueExec(t,owner,`INSERT INTO data_sources(provider_key,display_name,enabled,policy_url) VALUES('tvmaze','Synthetic scoped source',true,'https://www.tvmaze.com/api')`)
 f:=&scheduleQueueFetcher{}
 ids:=[]string{"show-101-season-501"}
 options:=catalogsync.ImportOptions{SchedulesOnly:true}
 sync:=func()(bool,error){return catalogsync.SyncQueuedProviderWithOptions(ctx,writer,"tvmaze",f,ids,time.Hour,time.Now().UTC(),options)}
 if did,err:=sync();err==nil||did||f.calls!=0{t.Fatal("fetch before identity mapping",did,err,f.calls)}
 queueExec(t,owner,`INSERT INTO anime_external_ids(anime_id,provider,external_id) VALUES($1,'tvmaze',$2)`,one,ids[0])
 if did,err:=sync();err!=nil||!did||f.calls!=1{t.Fatal("secondary queue import",did,err,f.calls)}
 var scope,title string
 if err:=owner.QueryRow(ctx,`SELECT import_scope FROM catalog_fetch_jobs`).Scan(&scope);err!=nil||scope!="SCHEDULES"{t.Fatal("scope not persisted",scope,err)}
 if err:=owner.QueryRow(ctx,`SELECT canonical_title FROM anime WHERE id=$1`,one).Scan(&title);err!=nil||title!="Synthetic Authoritative One"{t.Fatal("queue replaced metadata",title,err)}
 if did,err:=catalogsync.SyncQueuedProvider(ctx,writer,"tvmaze",f,ids,time.Hour,time.Now().UTC());err==nil||did||f.calls!=1{t.Fatal("scope silently changed",did,err,f.calls)}
 for _,query:=range []string{
  `UPDATE catalog_fetch_jobs SET state='LEASED',attempts=1,lease_digest=decode(repeat('01',32),'hex'),lease_until=now()+interval '3 minutes'`,
  `UPDATE catalog_fetch_jobs SET state='SUCCESS'`,
 }{if _,err:=writer.Exec(ctx,query);err==nil{t.Fatal("legacy worker wrote scoped work without explicit context")}}
 runtime,err:=pgx.Connect(ctx,runtimeDSN);if err!=nil{t.Fatal(err)};defer runtime.Close(ctx)
 if _,err=runtime.Exec(ctx,`SELECT * FROM catalog_schedule_observations`);err==nil{t.Fatal("API read operator observations")}
 if _,err=writer.Exec(ctx,`DELETE FROM catalog_schedule_observations`);err==nil{t.Fatal("writer erased observation baseline")}
 // Rebind during HTTP to model an operator correction. The old claim must
 // not target a different canonical title with the same external ID.
 queueDue(t,owner)
 queueExec(t,owner,`BEGIN; SET LOCAL app.catalog_import_scope='SCHEDULES'; UPDATE catalog_fetch_jobs SET next_attempt_at=now()-interval '1 day'; COMMIT;`)
 f.before=func(){queueExec(t,owner,`UPDATE anime_external_ids SET anime_id=$1 WHERE provider='tvmaze'`,two)}
 if did,err:=sync();err==nil||did{t.Fatal("changed canonical mapping accepted",did,err)}
 var n int
 if err=owner.QueryRow(ctx,`SELECT count(*) FROM airing_schedule WHERE anime_id=$1`,two).Scan(&n);err!=nil||n!=0{t.Fatal("stale claim wrote new canonical target",n,err)}
 f.before=nil
 // A late acknowledgement failure rolls back the new observation and schedule.
 queueDue(t,owner)
 queueExec(t,owner,`BEGIN; SET LOCAL app.catalog_import_scope='SCHEDULES'; UPDATE catalog_fetch_jobs SET next_attempt_at=now()-interval '1 day'; COMMIT;`)
 queueExec(t,owner,`CREATE FUNCTION reject_scoped_ack() RETURNS trigger LANGUAGE plpgsql AS $$
 BEGIN IF NEW.state='SUCCESS' THEN RAISE EXCEPTION 'synthetic scoped acknowledgement failure'; END IF; RETURN NEW; END $$;
 CREATE TRIGGER reject_scoped_ack BEFORE UPDATE ON catalog_fetch_jobs FOR EACH ROW EXECUTE FUNCTION reject_scoped_ack()`)
 if did,err:=sync();err==nil||did{t.Fatal("late acknowledgement succeeded",did,err)}
 queueExec(t,owner,`DROP TRIGGER reject_scoped_ack ON catalog_fetch_jobs; DROP FUNCTION reject_scoped_ack()`)
 if err=owner.QueryRow(ctx,`SELECT count(*) FROM catalog_schedule_observations WHERE anime_id=$1`,two).Scan(&n);err!=nil||n!=0{t.Fatal("late failure retained observation",n,err)}
 if err=owner.QueryRow(ctx,`SELECT count(*) FROM airing_schedule WHERE anime_id=$1`,two).Scan(&n);err!=nil||n!=0{t.Fatal("late failure retained schedules",n,err)}
 var state string
 if err=owner.QueryRow(ctx,`SELECT state FROM catalog_fetch_jobs`).Scan(&state);err!=nil||state!="RETRY"{t.Fatal("scoped failure stranded claim",state,err)}
}
func TestScheduleScopeDowngradePreservesRetainedWork(t *testing.T){
 owner,_,_,_:=queuedDB(t)
 ctx:=context.Background()
 _,_,base:=scopeFixture(t,owner)
 s:=scopeSnapshot("scope-secondary","one",base.Add(time.Hour))
 if _,err:=catalogsync.ImportWithOptions(ctx,owner,s,time.Now(),catalogsync.ImportOptions{SchedulesOnly:true});err!=nil{t.Fatal(err)}
 down,err:=os.ReadFile("../../../migrations/000018_catalog_schedule_scope.down.sql");if err!=nil{t.Fatal(err)}
 if _,err=owner.Exec(ctx,string(down));err==nil{t.Fatal("retained observation discarded")}
 queueExec(t,owner,`DELETE FROM catalog_schedule_observations`)
 queueExec(t,owner,`BEGIN; SET LOCAL app.catalog_import_scope='SCHEDULES'; INSERT INTO catalog_fetch_jobs(source_id,external_id,interval_seconds,next_attempt_at,updated_at,import_scope)
 SELECT id,'Q100',3600,now(),now(),'SCHEDULES' FROM data_sources WHERE provider_key='scope-secondary'; COMMIT;`)
 if _,err=owner.Exec(ctx,string(down));err==nil{t.Fatal("retained job lost its scope")}
 var n int
 if err=owner.QueryRow(ctx,`SELECT count(*) FROM catalog_fetch_jobs WHERE import_scope='SCHEDULES'`).Scan(&n);err!=nil||n!=1{t.Fatal("failed downgrade changed scope",n,err)}
 queueExec(t,owner,`DELETE FROM catalog_fetch_jobs`)
 queueExec(t,owner,string(down))
 up,err:=os.ReadFile("../../../migrations/000018_catalog_schedule_scope.up.sql");if err!=nil{t.Fatal(err)}
 queueExec(t,owner,string(up))
}
