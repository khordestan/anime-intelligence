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
 "github.com/khordestan/anime-intelligence/internal/providers/wikidata"
)

type queuedFetcher struct {
 calls []string
 before func()
 err error
 wrongID bool
}
func(f *queuedFetcher)Fetch(_ context.Context,ids []string)(feed.Snapshot,error){
 if len(ids)!=1{return feed.Snapshot{},errors.New("expected one title")}
 f.calls=append(f.calls,ids[0])
 if f.before!=nil{f.before()}
 if f.err!=nil{return feed.Snapshot{},f.err}
 id:=ids[0]
 if f.wrongID{id="Q999"}
 return feed.Snapshot{Version:1,Provider:"wikidata",ObservedAt:time.Now().UTC(),
 Titles:[]feed.Title{{ExternalID:id,Title:"Synthetic queued "+id,MediaType:"TV",Status:"UNKNOWN"}}},nil
}
func queuedDB(t *testing.T)(*pgx.Conn,*pgx.Conn,string,string){
 t.Helper()
 _,owner,_,runtimeDSN:=testDB(t)
 ctx:=context.Background()
 u,err:=url.Parse(runtimeDSN);if err!=nil{t.Fatal(err)}
 role:=u.User.Username()+"_queue"
 script,err:=os.ReadFile("../../../deployments/catalog-writer-role.sql");if err!=nil{t.Fatal(err)}
 if _,err=owner.Exec(ctx,strings.ReplaceAll(string(script),"anime_catalog_writer",pgx.Identifier{role}.Sanitize()));err!=nil{t.Fatal(err)}
 t.Cleanup(func(){_,_=owner.Exec(ctx,`DROP OWNED BY `+pgx.Identifier{role}.Sanitize());_,_=owner.Exec(ctx,`DROP ROLE `+pgx.Identifier{role}.Sanitize())})
 password,_:=u.User.Password()
 if _,err=owner.Exec(ctx,`ALTER ROLE `+pgx.Identifier{role}.Sanitize()+` PASSWORD '`+password+`'`);err!=nil{t.Fatal(err)}
 u.User=url.UserPassword(role,password)
 writer,err:=pgx.Connect(ctx,u.String());if err!=nil{t.Fatal(err)}
 t.Cleanup(func(){_=writer.Close(ctx)})
 return owner,writer,u.String(),runtimeDSN
}
func queueExec(t *testing.T,conn *pgx.Conn,sql string,args ...any){
 t.Helper();if _,err:=conn.Exec(context.Background(),sql,args...);err!=nil{t.Fatal(err)}
}
func queueDue(t *testing.T,owner *pgx.Conn){
 t.Helper()
 queueExec(t,owner,`UPDATE catalog_fetch_state SET last_attempt_at=now()-interval '2 days',next_attempt_at=now()-interval '1 day'`)
}
func approveQueue(t *testing.T,owner *pgx.Conn){
 t.Helper()
 queueExec(t,owner,`INSERT INTO data_sources(provider_key,display_name,enabled,policy_url)
 VALUES('wikidata','Synthetic queued source',true,'https://example.test/policy')`)
}

func TestCatalogQueueDurablePerTitleSchedulingAndPrivileges(t *testing.T){
 owner,writer,dsn,runtimeDSN:=queuedDB(t)
 ctx:=context.Background()
 f:=&queuedFetcher{}
 sync:=func(conn *pgx.Conn,ids ...string)(bool,error){
  return catalogsync.SyncQueuedWikidata(ctx,conn,f,ids,time.Hour,time.Now().UTC())
 }
 if _,err:=sync(writer,"Q1");!errors.Is(err,catalogsync.ErrSource)||len(f.calls)!=0{t.Fatal("fetch before approval",err)}
 approveQueue(t,owner)
 contender,err:=pgx.Connect(ctx,dsn);if err!=nil{t.Fatal(err)}
 defer contender.Close(ctx)
 // The fetch holds no source/job row transaction: another connection can lock both.
 f.before=func(){
  tx,err:=owner.Begin(ctx);if err!=nil{t.Fatal(err)}
  defer tx.Rollback(ctx)
  if _,err=tx.Exec(ctx,`SELECT id FROM data_sources FOR UPDATE NOWAIT`);err!=nil{t.Fatal("source locked across HTTP",err)}
  if _,err=tx.Exec(ctx,`SELECT external_id FROM catalog_fetch_jobs FOR UPDATE NOWAIT`);err!=nil{t.Fatal("job locked across HTTP",err)}
  if did,err:=sync(contender,"Q1","Q2");err!=nil||did{t.Fatal("concurrent collector was not excluded",did,err)}
 }
 ids:=[]string{"Q2","Q1"}
 if did,err:=sync(writer,ids...);err!=nil||!did||len(f.calls)!=1||f.calls[0]!="Q1"{t.Fatal("first title",did,err,f.calls)}
 if ids[0]!="Q2"{t.Fatal("caller IDs mutated")}
 var state string;var attempts int;var lease []byte;var due time.Time
 if err:=owner.QueryRow(ctx,`SELECT state,attempts,lease_digest,next_attempt_at FROM catalog_fetch_jobs WHERE external_id='Q1'`).Scan(&state,&attempts,&lease,&due);err!=nil||state!="SUCCESS"||attempts!=0||lease!=nil||due.Before(time.Now().Add(59*time.Minute)){t.Fatal("completion",state,attempts,due,err)}
 restarted,err:=pgx.Connect(ctx,dsn);if err!=nil{t.Fatal(err)}
 defer restarted.Close(ctx)
 if did,err:=sync(restarted,"Q1","Q2");err!=nil||did||len(f.calls)!=1{t.Fatal("restart bypassed cooldown",did,err)}
 queueDue(t,owner)
 if did,err:=sync(restarted,"Q1","Q2","Q3");err!=nil||!did||f.calls[1]!="Q2"{t.Fatal("pending job lost/new title delayed by source observation",did,err,f.calls)}
 queueDue(t,owner)
 // An excluded due title is dormant, not silently processed from the queue.
 queueExec(t,owner,`UPDATE catalog_fetch_jobs SET next_attempt_at=now()-interval '1 day' WHERE external_id='Q1'`)
 if did,err:=sync(restarted,"Q3");err!=nil||!did||f.calls[2]!="Q3"{t.Fatal("configured allowlist",did,err,f.calls)}
 queueDue(t,owner)
 if did,err:=sync(restarted,"Q1");err!=nil||!did||f.calls[3]!="Q1"{t.Fatal("per-title due refresh",did,err,f.calls)}
 runtime,err:=pgx.Connect(ctx,runtimeDSN);if err!=nil{t.Fatal(err)}
 defer runtime.Close(ctx)
 if _,err=runtime.Exec(ctx,`SELECT * FROM catalog_fetch_jobs`);err==nil{t.Fatal("API role read queue")}
 if _,err=writer.Exec(ctx,`SELECT email FROM users`);err==nil{t.Fatal("worker read accounts")}
 if _,err=writer.Exec(ctx,`DELETE FROM catalog_fetch_jobs`);err==nil{t.Fatal("worker deleted jobs")}
 if _,err=writer.Exec(ctx,`UPDATE data_sources SET enabled=false`);err==nil{t.Fatal("worker activated sources")}
 if _,err=sync(owner,"Q1");err==nil{t.Fatal("administrator accepted")}
 if _,err=sync(runtime,"Q1");err==nil{t.Fatal("runtime accepted")}
}

func TestCatalogQueueRetriesCooldownAndCrashRecovery(t *testing.T){
 owner,writer,dsn,_:=queuedDB(t)
 ctx:=context.Background()
 approveQueue(t,owner)
 reset:=func(){queueExec(t,owner,`DELETE FROM catalog_fetch_jobs; DELETE FROM catalog_fetch_state`)}
 sync:=func(conn *pgx.Conn,f *queuedFetcher,id string)(bool,error){
  return catalogsync.SyncQueuedWikidata(ctx,conn,f,[]string{id},time.Hour,time.Now().UTC())
 }
 t.Run("bounded failures and poison isolation",func(t *testing.T){
  reset()
  f:=&queuedFetcher{err:errors.New("synthetic response must not be retained")}
  for attempt:=1;attempt<=5;attempt++{
   queueDue(t,owner)
   queueExec(t,owner,`UPDATE catalog_fetch_jobs SET next_attempt_at=now()-interval '1 day'`)
   if did,err:=sync(writer,f,"Q10");err==nil||did{t.Fatal("failed fetch succeeded",did,err)}
   var state string;var count int;var due time.Time;var digest []byte
   if err:=owner.QueryRow(ctx,`SELECT state,attempts,next_attempt_at,lease_digest FROM catalog_fetch_jobs`).Scan(&state,&count,&due,&digest);err!=nil{t.Fatal(err)}
   expected:="RETRY";if attempt==5{expected="FAILED"}
   if state!=expected||count!=attempt||digest!=nil||due.Before(time.Now().Add(14*time.Minute)){t.Fatal("retry state",state,count,due)}
  }
  queueDue(t,owner)
  if did,err:=sync(writer,f,"Q10");err!=nil||did||len(f.calls)!=5{t.Fatal("exhausted job fetched",did,err)}
  f.err=nil
  if did,err:=catalogsync.SyncQueuedWikidata(ctx,writer,f,[]string{"Q10","Q11"},time.Hour,time.Now().UTC());err!=nil||!did||f.calls[5]!="Q11"{t.Fatal("poison job blocked healthy title",did,err,f.calls)}
 })
 t.Run("provider cooldown survives a restart and newly added ID",func(t *testing.T){
  reset()
  until:=time.Now().UTC().Add(24*time.Hour).Truncate(time.Microsecond)
  f:=&queuedFetcher{err:&wikidata.RateLimitError{Until:until}}
  if _,err:=sync(writer,f,"Q20");err==nil{t.Fatal("rate limit accepted")}
  var sourceDue,jobDue time.Time;var outcome string
  if err:=owner.QueryRow(ctx,`SELECT f.next_attempt_at,j.next_attempt_at,f.outcome FROM catalog_fetch_state f JOIN catalog_fetch_jobs j USING(source_id)`).Scan(&sourceDue,&jobDue,&outcome);err!=nil||sourceDue.Before(until)||jobDue.Before(until)||outcome!="RATE_LIMITED"{t.Fatal("cooldown",sourceDue,jobDue,outcome,err)}
  restarted,err:=pgx.Connect(ctx,dsn);if err!=nil{t.Fatal(err)};defer restarted.Close(ctx)
  f.err=nil
  if did,err:=sync(restarted,f,"Q21");err!=nil||did||len(f.calls)!=1{t.Fatal("new ID/restart bypassed cooldown",did,err,f.calls)}
 })
 t.Run("expired claims consume the retry budget",func(t *testing.T){
  reset()
  f:=&queuedFetcher{err:errors.New("synthetic fetch failure")}
  _,_=sync(writer,f,"Q30")
  queueExec(t,owner,`UPDATE catalog_fetch_jobs SET state='LEASED',attempts=4,lease_digest=decode(repeat('01',32),'hex'),lease_until=now()-interval '1 minute',next_attempt_at=now()-interval '1 minute'`)
  queueDue(t,owner);f.err=nil
  if did,err:=sync(writer,f,"Q30");err!=nil||!did||len(f.calls)!=2{t.Fatal("crash recovery",did,err,f.calls)}
  queueExec(t,owner,`UPDATE catalog_fetch_jobs SET state='LEASED',attempts=5,lease_digest=decode(repeat('02',32),'hex'),lease_until=now()-interval '1 minute',next_attempt_at=now()-interval '1 minute'`)
  queueDue(t,owner)
  if did,err:=sync(writer,f,"Q30");err!=nil||did||len(f.calls)!=2{t.Fatal("final crashed claim retried",did,err)}
  var state string
  if err:=owner.QueryRow(ctx,`SELECT state FROM catalog_fetch_jobs`).Scan(&state);err!=nil||state!="FAILED"{t.Fatal("final crash not terminal",state,err)}
 })
}

func TestCatalogQueueAtomicImportAndLeaseFences(t *testing.T){
 owner,writer,_,_:=queuedDB(t)
 ctx:=context.Background()
 approveQueue(t,owner)
 cases:=[]struct{name string;id string;before func();wrong bool}{
  {"expired lease","Q40",func(){queueExec(t,owner,`UPDATE catalog_fetch_jobs SET lease_until=now()-interval '1 second' WHERE external_id='Q40'`)},false},
  {"replaced lease","Q41",func(){queueExec(t,owner,`UPDATE catalog_fetch_jobs SET lease_digest=decode(repeat('03',32),'hex') WHERE external_id='Q41'`)},false},
  {"approval revoked during HTTP","Q42",func(){queueExec(t,owner,`UPDATE data_sources SET enabled=false WHERE provider_key='wikidata'`)},false},
  {"unexpected response title","Q43",nil,true},
 }
 for _,tc:=range cases{t.Run(tc.name,func(t *testing.T){
  queueExec(t,owner,`DELETE FROM catalog_fetch_jobs; DELETE FROM catalog_fetch_state; UPDATE data_sources SET enabled=true WHERE provider_key='wikidata'`)
  f:=&queuedFetcher{before:tc.before,wrongID:tc.wrong}
  if did,err:=catalogsync.SyncQueuedWikidata(ctx,writer,f,[]string{tc.id},time.Hour,time.Now().UTC());err==nil||did{t.Fatal("invalid claim imported",did,err)}
  var count int
  if err:=owner.QueryRow(ctx,`SELECT count(*) FROM anime_external_ids WHERE provider='wikidata'`).Scan(&count);err!=nil||count!=0{t.Fatal("uncommitted facts leaked",count,err)}
  if err:=owner.QueryRow(ctx,`SELECT count(*) FROM catalog_sync_state`).Scan(&count);err!=nil||count!=0{t.Fatal("high-water mark leaked",count,err)}
 })}
 t.Run("late acknowledgement failure rolls back the complete import",func(t *testing.T){
  queueExec(t,owner,`DELETE FROM catalog_fetch_jobs; DELETE FROM catalog_fetch_state; UPDATE data_sources SET enabled=true WHERE provider_key='wikidata'`)
  queueExec(t,owner,`CREATE FUNCTION reject_queue_success() RETURNS trigger LANGUAGE plpgsql AS $$
 BEGIN IF NEW.state='SUCCESS' THEN RAISE EXCEPTION 'synthetic acknowledgement failure'; END IF; RETURN NEW; END $$;
 CREATE TRIGGER reject_queue_success BEFORE UPDATE ON catalog_fetch_jobs FOR EACH ROW EXECUTE FUNCTION reject_queue_success()`)
  defer queueExec(t,owner,`DROP TRIGGER reject_queue_success ON catalog_fetch_jobs; DROP FUNCTION reject_queue_success()`)
  f:=&queuedFetcher{}
  if did,err:=catalogsync.SyncQueuedWikidata(ctx,writer,f,[]string{"Q50"},time.Hour,time.Now().UTC());err==nil||did{t.Fatal("late failure succeeded",did,err)}
  var count int;var state string
  if err:=owner.QueryRow(ctx,`SELECT count(*) FROM anime_external_ids WHERE provider='wikidata'`).Scan(&count);err!=nil||count!=0{t.Fatal("late failure retained canonical facts",count,err)}
  if err:=owner.QueryRow(ctx,`SELECT count(*) FROM catalog_metadata`).Scan(&count);err!=nil||count!=0{t.Fatal("late failure retained metadata",count,err)}
  if err:=owner.QueryRow(ctx,`SELECT count(*) FROM catalog_sync_state`).Scan(&count);err!=nil||count!=0{t.Fatal("late failure moved high-water mark",count,err)}
  if err:=owner.QueryRow(ctx,`SELECT state FROM catalog_fetch_jobs`).Scan(&state);err!=nil||state!="RETRY"{t.Fatal("late failure was not retryable",state,err)}
 })
 t.Run("cancelled fetch persists a retry",func(t *testing.T){
  queueExec(t,owner,`DELETE FROM catalog_fetch_jobs; DELETE FROM catalog_fetch_state`)
  cancelled,cancel:=context.WithCancel(ctx);defer cancel()
  f:=&queuedFetcher{before:cancel,err:context.Canceled}
  if did,err:=catalogsync.SyncQueuedWikidata(cancelled,writer,f,[]string{"Q60"},time.Hour,time.Now().UTC());err==nil||did{t.Fatal("cancelled fetch succeeded",did,err)}
  var state string
  if err:=owner.QueryRow(ctx,`SELECT state FROM catalog_fetch_jobs`).Scan(&state);err!=nil||state!="RETRY"{t.Fatal("cancellation stranded lease",state,err)}
 })
}

func TestCatalogQueueSkipsReviewedRetiredIDs(t *testing.T) {
 owner, writer, _, runtimeDSN := queuedDB(t)
 ctx := context.Background()
 approveQueue(t, owner)
 const animeID = "10000000-0000-4000-8000-000000000001"
 var mappingID string
 err := owner.QueryRow(ctx, `INSERT INTO anime_external_ids(anime_id,provider,external_id)
 VALUES($1,'wikidata','Q1') RETURNING id::text`, animeID).Scan(&mappingID)
 if err != nil {
  t.Fatal(err)
 }
 retractor, _ := identityRetractor(t, owner, runtimeDSN)
 now := time.Now().UTC()
 review := catalogidentity.RetractionReview{
  Version: 1, Provider: "wikidata", ReviewedAt: now,
  Reference: "https://example.test/retired-queue-id",
  Retractions: []catalogidentity.Retraction{{ExternalID: "Q1", AnimeID: animeID, ExpectedMappingID: mappingID}},
 }
 if _, err := catalogidentity.Retract(ctx, retractor, review, now, true); err != nil {
  t.Fatal(err)
 }

 fetcher := &queuedFetcher{}
 if did, err := catalogsync.SyncQueuedWikidata(ctx, writer, fetcher, []string{"Q1"}, time.Hour, now); err != nil || did || len(fetcher.calls) != 0 {
  t.Fatalf("retired-only cycle: did=%v err=%v calls=%v", did, err, fetcher.calls)
 }
 if did, err := catalogsync.SyncQueuedWikidata(ctx, writer, fetcher, []string{"Q1", "Q2"}, time.Hour, now); err != nil || !did || len(fetcher.calls) != 1 || fetcher.calls[0] != "Q2" {
  t.Fatalf("mixed cycle: did=%v err=%v calls=%v", did, err, fetcher.calls)
 }
 var jobs int
 if err := owner.QueryRow(ctx, "SELECT count(*) FROM catalog_fetch_jobs WHERE external_id='Q1'").Scan(&jobs); err != nil || jobs != 0 {
  t.Fatalf("retired job rows: count=%d err=%v", jobs, err)
 }
}
