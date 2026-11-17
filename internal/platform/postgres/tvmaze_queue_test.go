package postgres_test

import (
 "context"
 "errors"
 "os"
 "testing"
 "time"

 "github.com/khordestan/anime-intelligence/internal/modules/catalog/feed"
 "github.com/khordestan/anime-intelligence/internal/platform/catalogsync"
 "github.com/khordestan/anime-intelligence/internal/providers/tvmaze"
)

type tvmazeQueueFetcher struct{
 calls int
 err error
 wrongProvider bool
}
func(f *tvmazeQueueFetcher)Fetch(_ context.Context,ids []string)(feed.Snapshot,error){
 f.calls++
 if f.err!=nil{return feed.Snapshot{},f.err}
 at:=time.Now().UTC()
 past:=at.Add(-24*time.Hour)
 provider:="tvmaze"
 if f.wrongProvider{provider="wikidata"}
 return feed.Snapshot{Version:1,Provider:provider,ObservedAt:at,
 Titles:[]feed.Title{{ExternalID:ids[0],Title:"Synthetic reviewed anime — Season 2",MediaType:"TV",Status:"UNKNOWN",
 Episodes:[]feed.Episode{{Number:1,Track:"UNKNOWN",ScheduledAt:&past}}}}},nil
}
func TestTVmazeQueueApprovalIsolationAndUnconfirmedSchedules(t *testing.T){
 owner,writer,_,_:=queuedDB(t)
 ctx:=context.Background()
 approveQueue(t,owner)
 queueExec(t,owner,`INSERT INTO data_sources(provider_key,display_name,enabled,policy_url)
 VALUES('tvmaze','Synthetic reviewed TVmaze',false,'https://www.tvmaze.com/api')`)
 f:=&tvmazeQueueFetcher{}
 ids:=[]string{"show-101-season-501"}
 sync:=func()(bool,error){return catalogsync.SyncQueuedProvider(ctx,writer,"tvmaze",f,ids,time.Hour,time.Now().UTC())}
 if did,err:=sync();!errors.Is(err,catalogsync.ErrSource)||did||f.calls!=0{t.Fatal("unapproved source fetched",did,err)}
 queueExec(t,owner,`UPDATE data_sources SET enabled=true WHERE provider_key='tvmaze'; UPDATE data_sources SET enabled=false WHERE provider_key='wikidata'`)
 if did,err:=sync();err!=nil||!did||f.calls!=1{t.Fatal("TVmaze-specific approval",did,err)}
 var id string
 if err:=owner.QueryRow(ctx,`SELECT anime_id::text FROM anime_external_ids WHERE provider='tvmaze' AND external_id=$1`,ids[0]).Scan(&id);err!=nil{t.Fatal(err)}
 var count int
 if err:=owner.QueryRow(ctx,`SELECT count(*) FROM airing_schedule WHERE anime_id=$1 AND release_track='UNKNOWN' AND scheduled_at IS NOT NULL AND actual_released_at IS NULL`,id).Scan(&count);err!=nil||count!=1{t.Fatal("schedule became availability",count,err)}
 queueExec(t,owner,`UPDATE catalog_fetch_state SET last_attempt_at=now()-interval '2 days',next_attempt_at=now()-interval '1 day' WHERE source_id=(SELECT id FROM data_sources WHERE provider_key='tvmaze');
 UPDATE catalog_fetch_jobs SET next_attempt_at=now()-interval '1 day' WHERE source_id=(SELECT id FROM data_sources WHERE provider_key='tvmaze')`)
 if did,err:=sync();err!=nil||!did||f.calls!=2{t.Fatal("second schedule import",did,err)}
 if err:=owner.QueryRow(ctx,`SELECT count(*) FROM domain_events WHERE aggregate_id=$1`,id).Scan(&count);err!=nil||count!=0{t.Fatal("air dates generated release events",count,err)}
 queueExec(t,owner,`UPDATE data_sources SET enabled=true WHERE provider_key='wikidata'`)
 wiki:=&queuedFetcher{}
 if did,err:=catalogsync.SyncQueuedWikidata(ctx,writer,wiki,[]string{"Q701"},time.Hour,time.Now().UTC());err!=nil||!did{t.Fatal("provider state crossed sources",did,err)}
 if err:=owner.QueryRow(ctx,`SELECT count(DISTINCT source_id) FROM catalog_fetch_jobs`).Scan(&count);err!=nil||count!=2{t.Fatal("job identities collided",count,err)}
 // A returned provider must match the configured source, even for valid IDs.
 queueExec(t,owner,`UPDATE catalog_fetch_state SET last_attempt_at=now()-interval '2 days',next_attempt_at=now()-interval '1 day' WHERE source_id=(SELECT id FROM data_sources WHERE provider_key='tvmaze');
 UPDATE catalog_fetch_jobs SET next_attempt_at=now()-interval '1 day' WHERE source_id=(SELECT id FROM data_sources WHERE provider_key='tvmaze')`)
 f.wrongProvider=true
 if did,err:=sync();err==nil||did{t.Fatal("provider spoofing accepted",did,err)}
 f.wrongProvider=false
 // Rate limiting one provider must not suppress a different provider's queue.
 until:=time.Now().UTC().Add(24*time.Hour).Truncate(time.Microsecond)
 queueExec(t,owner,`UPDATE catalog_fetch_state SET last_attempt_at=now()-interval '2 days',next_attempt_at=now()-interval '1 day' WHERE source_id=(SELECT id FROM data_sources WHERE provider_key='tvmaze');
 UPDATE catalog_fetch_jobs SET next_attempt_at=now()-interval '1 day' WHERE source_id=(SELECT id FROM data_sources WHERE provider_key='tvmaze')`)
 f.err=&tvmaze.RateLimitError{Until:until}
 if _,err:=sync();err==nil{t.Fatal("rate limit accepted")}
 var next time.Time
 if err:=owner.QueryRow(ctx,`SELECT next_attempt_at FROM catalog_fetch_state WHERE source_id=(SELECT id FROM data_sources WHERE provider_key='tvmaze')`).Scan(&next);err!=nil||next.Before(until){t.Fatal("TVmaze cooldown not persisted",next,err)}
 prior:=f.calls
 ids=[]string{"show-102-season-502"};f.err=nil
 if did,err:=sync();err!=nil||did||f.calls!=prior{t.Fatal("new ID bypassed TVmaze cooldown",did,err)}
 queueExec(t,owner,`UPDATE catalog_fetch_state SET last_attempt_at=now()-interval '2 days',next_attempt_at=now()-interval '1 day' WHERE source_id=(SELECT id FROM data_sources WHERE provider_key='wikidata')`)
 if did,err:=catalogsync.SyncQueuedWikidata(ctx,writer,wiki,[]string{"Q702"},time.Hour,time.Now().UTC());err!=nil||!did{t.Fatal("TVmaze cooldown blocked Wikidata",did,err)}
 for _,tc:=range []struct{provider,id string}{{"tvmaze","Q1"},{"wikidata","show-101-season-501"},{"unknown","show-101-season-501"}}{
  if did,err:=catalogsync.SyncQueuedProvider(ctx,writer,tc.provider,f,[]string{tc.id},time.Hour,time.Now().UTC());err==nil||did{t.Fatal("cross-provider ID accepted",tc,did,err)}
 }
}
func TestTVmazeQueueMigrationRollbackPreservesWork(t *testing.T){
 owner,_,_,_:=queuedDB(t)
 ctx:=context.Background()
 approveQueue(t,owner)
 queueExec(t,owner,`INSERT INTO data_sources(provider_key,display_name,enabled,policy_url) VALUES('tvmaze','Synthetic migration source',false,'https://www.tvmaze.com/api')`)
 queueExec(t,owner,`INSERT INTO catalog_fetch_jobs(source_id,external_id,interval_seconds,next_attempt_at,updated_at)
 SELECT id,'show-101-season-501',3600,now(),now() FROM data_sources WHERE provider_key='tvmaze'`)
 down,err:=os.ReadFile("../../../migrations/000017_tvmaze_fetch_jobs.down.sql");if err!=nil{t.Fatal(err)}
 if _,err=owner.Exec(ctx,string(down));err==nil{t.Fatal("downgrade discarded TVmaze identity validation")}
 var def string
 if err=owner.QueryRow(ctx,`SELECT pg_get_constraintdef(oid) FROM pg_constraint WHERE conrelid='catalog_fetch_jobs'::regclass AND conname='catalog_fetch_jobs_external_id_check'`).Scan(&def);err!=nil||def==""{t.Fatal("failed downgrade removed constraint",def,err)}
 var count int
 if err=owner.QueryRow(ctx,`SELECT count(*) FROM catalog_fetch_jobs`).Scan(&count);err!=nil||count!=1{t.Fatal("failed downgrade lost work",count,err)}
 queueExec(t,owner,`DELETE FROM catalog_fetch_jobs WHERE external_id='show-101-season-501'`)
 queueExec(t,owner,string(down))
 if _,err=owner.Exec(ctx,`INSERT INTO catalog_fetch_jobs(source_id,external_id,interval_seconds,next_attempt_at,updated_at) SELECT id,'show-101-season-501',3600,now(),now() FROM data_sources WHERE provider_key='tvmaze'`);err==nil{t.Fatal("downgrade left expanded IDs enabled")}
 up,err:=os.ReadFile("../../../migrations/000017_tvmaze_fetch_jobs.up.sql");if err!=nil{t.Fatal(err)}
 queueExec(t,owner,string(up))
}
