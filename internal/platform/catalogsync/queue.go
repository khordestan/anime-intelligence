package catalogsync

import (
 "context"
 "crypto/rand"
 "crypto/sha256"
 "errors"
 "regexp"
 "sort"
 "time"

 "github.com/jackc/pgx/v5"
 "github.com/khordestan/anime-intelligence/internal/modules/catalog/feed"
 "github.com/khordestan/anime-intelligence/internal/providers/wikidata"
 "github.com/khordestan/anime-intelligence/internal/providers/tvmaze"
)

const fetchLease = 3*time.Minute
const sourceSpacing = time.Minute
var fetchID = regexp.MustCompile(`^Q[1-9][0-9]{0,11}$`)

type fetchClaim struct {
 source string
 provider string
 scope string
 canonicalID string
 id string
 digest []byte
 interval time.Duration
 attempts int
}

// SyncQueuedWikidata schedules only explicitly configured IDs and processes at
// most one due title. Stored jobs survive restarts; removed IDs remain dormant.
// No transaction is held during HTTP. The shared collector lock coordinates
// this queue with legacy collectors; leases fence acknowledgements after crashes.
func SyncQueuedWikidata(ctx context.Context,conn *pgx.Conn,fetcher Fetcher,ids []string,interval time.Duration,now time.Time)(bool,error){
 return SyncQueuedProvider(ctx,conn,"wikidata",fetcher,ids,interval,now)
}
// SyncQueuedProvider supports only reviewed Wikidata and TVmaze adapters. No
// dynamic endpoint, arbitrary provider or automatic anime classification is allowed.
func SyncQueuedProvider(ctx context.Context,conn *pgx.Conn,provider string,fetcher Fetcher,ids []string,interval time.Duration,now time.Time)(bool,error){
 return SyncQueuedProviderWithOptions(ctx,conn,provider,fetcher,ids,interval,now,ImportOptions{})
}
// SyncQueuedProviderWithOptions pins each durable job to its reviewed import
// scope. Configuration changes never silently promote schedule-only work.
func SyncQueuedProviderWithOptions(ctx context.Context,conn *pgx.Conn,provider string,fetcher Fetcher,ids []string,interval time.Duration,now time.Time,options ImportOptions)(bool,error){
 if options.Preview||options.Archive{return false,ErrWorker}
 scope:="FULL";if options.SchedulesOnly{scope="SCHEDULES"}

 if provider!="wikidata"&&provider!="tvmaze"{return false,ErrWorker}
 if fetcher==nil || conn==nil || now.IsZero() || now.Unix()<0 || now.Year()>9999 ||
 interval<time.Hour || interval>7*24*time.Hour || len(ids)==0 || len(ids)>20 {return false,ErrWorker}
 ordered:=append([]string(nil),ids...)
 sort.Strings(ordered)
 for i,id:=range ordered {if !validFetchID(provider,id)||(i>0&&id==ordered[i-1]) {return false,ErrWorker}}
 if err:=CheckWriter(ctx,conn);err!=nil{return false,ErrWorker}
 var locked bool
 if err:=conn.QueryRow(ctx,`SELECT pg_try_advisory_lock(742019686)`).Scan(&locked);err!=nil{return false,ErrWorker}
 if !locked{return false,nil}
 defer func(){cleanup,cancel:=context.WithTimeout(context.Background(),5*time.Second);defer cancel();_,_=conn.Exec(cleanup,`SELECT pg_advisory_unlock(742019686)`)}()
 claim,err:=claimFetch(ctx,conn,provider,scope,ordered,interval,now.UTC().Truncate(time.Microsecond))
 if err!=nil{return false,err}
 if claim==nil{return false,nil}
 snapshot,fetchErr:=fetcher.Fetch(ctx,[]string{claim.id})
 if fetchErr!=nil{
  outcome:="FETCH_FAILED"
  until:=time.Now().UTC().Add(fetchRetry(claim.attempts))
  var limited *wikidata.RateLimitError
  if errors.As(fetchErr,&limited){
   outcome="RATE_LIMITED"
   retry:=limited.Until
   if retry.IsZero(){retry=time.Now().UTC().Add(limited.Wait)}
   if retry.After(until){until=retry}
  }
  var tvLimited *tvmaze.RateLimitError
  if errors.As(fetchErr,&tvLimited){
   outcome="RATE_LIMITED"
   retry:=tvLimited.Until
   if retry.IsZero(){retry=time.Now().UTC().Add(tvLimited.Wait)}
   if retry.After(until){until=retry}
  }
  _=failFetch(conn,*claim,outcome,until)
  return false,ErrWorker
 }
 now=time.Now().UTC()
 if snapshot.Provider!=provider || len(snapshot.Titles)!=1 ||
 snapshot.Titles[0].ExternalID!=claim.id || snapshot.Validate(now)!=nil {
  _=failFetch(conn,*claim,"FETCH_FAILED",now.Add(fetchRetry(claim.attempts)))
  return false,ErrWorker
 }
 snapshot.ObservedAt=snapshot.ObservedAt.UTC().Truncate(time.Microsecond)
 if err=completeFetch(ctx,conn,*claim,snapshot);err!=nil{
  _=failFetch(conn,*claim,"IMPORT_FAILED",time.Now().UTC().Add(fetchRetry(claim.attempts)))
  return false,ErrWorker
 }
 return true,nil
}

func rollbackFetch(tx pgx.Tx){
 cleanup,cancel:=context.WithTimeout(context.Background(),5*time.Second)
 defer cancel()
 _=tx.Rollback(cleanup)
}

// Five claims per refresh cycle, including expired leases. Successful refreshes
// reset that budget. Retries are persisted at 15/30/60/120 minutes.
func fetchRetry(attempt int)time.Duration{
 if attempt<1{attempt=1}
 if attempt>4{attempt=4}
 return 15*time.Minute*time.Duration(1<<uint(attempt-1))
}

func claimFetch(ctx context.Context,conn *pgx.Conn,provider,scope string,ids []string,interval time.Duration,now time.Time)(*fetchClaim,error){
 tx,err:=conn.Begin(ctx)
 if err!=nil{return nil,ErrWorker}
 defer rollbackFetch(tx)
 if _,err=tx.Exec(ctx,"SET LOCAL statement_timeout='2s'");err!=nil{return nil,ErrWorker}
 if _,err=tx.Exec(ctx,`SELECT set_config('app.catalog_import_scope',$1,true)`,scope);err!=nil{return nil,ErrWorker}
 var source string
 err=tx.QueryRow(ctx,`SELECT id::text FROM data_sources WHERE provider_key=$1
 AND enabled AND COALESCE(policy_url,'')<>'' FOR UPDATE`,provider).Scan(&source)
 if errors.Is(err,pgx.ErrNoRows){return nil,ErrSource}
 if err!=nil{return nil,ErrWorker}
 seconds:=int((interval+time.Second-1)/time.Second)
 for _,id:=range ids{
  _,err=tx.Exec(ctx,`INSERT INTO catalog_fetch_jobs(source_id,external_id,interval_seconds,next_attempt_at,updated_at,import_scope)
 VALUES($1,$2,$3,$4,$4,$5) ON CONFLICT(source_id,external_id) DO UPDATE
 SET interval_seconds=excluded.interval_seconds
 WHERE catalog_fetch_jobs.interval_seconds<>excluded.interval_seconds
 AND catalog_fetch_jobs.import_scope=excluded.import_scope`,source,id,seconds,now,scope)
  if err!=nil{return nil,ErrWorker}
 }
 var mismatched bool
 if err=tx.QueryRow(ctx,`SELECT EXISTS(SELECT 1 FROM catalog_fetch_jobs
 WHERE source_id=$1 AND external_id=ANY($2::text[]) AND import_scope<>$3)`,source,ids,scope).Scan(&mismatched);err!=nil||mismatched{return nil,ErrWorker}
 if scope=="SCHEDULES"{
  var mapped int
  if err=tx.QueryRow(ctx,`SELECT count(*) FROM anime_external_ids WHERE provider=$1 AND external_id=ANY($2::text[])`,provider,ids).Scan(&mapped);err!=nil||mapped!=len(ids){return nil,ErrWorker}
 }
 var next,observed *time.Time
 err=tx.QueryRow(ctx,`SELECT f.next_attempt_at,s.observed_at FROM data_sources d
 LEFT JOIN catalog_fetch_state f ON f.source_id=d.id LEFT JOIN catalog_sync_state s ON s.source_id=d.id
 WHERE d.id=$1`,source).Scan(&next,&observed)
 if err!=nil || (observed!=nil&&observed.After(now)){return nil,ErrWorker}
 if next!=nil&&now.Before(*next){if err=tx.Commit(ctx);err!=nil{return nil,ErrWorker};return nil,nil}
 // A crash on the final claim must not leave an immortal LEASED job.
 _,err=tx.Exec(ctx,`UPDATE catalog_fetch_jobs SET state='FAILED',lease_digest=NULL,lease_until=NULL,updated_at=$3
 WHERE source_id=$1 AND external_id=ANY($2::text[]) AND state='LEASED' AND lease_until<=$3 AND attempts>=5`,source,ids,now)
 if err!=nil{return nil,ErrWorker}
 claim:=&fetchClaim{source:source,provider:provider,scope:scope}
 var intervalSeconds int
 err=tx.QueryRow(ctx,`SELECT external_id,interval_seconds,attempts FROM catalog_fetch_jobs
 WHERE source_id=$1 AND external_id=ANY($2::text[]) AND state<>'FAILED'
 AND ((state='LEASED' AND lease_until<=$3 AND attempts<5)
 OR (state<>'LEASED' AND next_attempt_at<=$3 AND attempts<5))
 ORDER BY next_attempt_at,external_id LIMIT 1 FOR UPDATE SKIP LOCKED`,source,ids,now).Scan(&claim.id,&intervalSeconds,&claim.attempts)
 if errors.Is(err,pgx.ErrNoRows){if err=tx.Commit(ctx);err!=nil{return nil,ErrWorker};return nil,nil}
 if err!=nil{return nil,ErrWorker}
 if scope=="SCHEDULES"{
  if err=tx.QueryRow(ctx,`SELECT anime_id::text FROM anime_external_ids WHERE provider=$1 AND external_id=$2`,provider,claim.id).Scan(&claim.canonicalID);err!=nil{return nil,ErrWorker}
 }
 secret:=make([]byte,32)
 if _,err=rand.Read(secret);err!=nil{return nil,ErrWorker}
 digest:=sha256.Sum256(secret)
 claim.digest=digest[:]
 claim.attempts++
 claim.interval=time.Duration(intervalSeconds)*time.Second
 _,err=tx.Exec(ctx,`UPDATE catalog_fetch_jobs SET state='LEASED',attempts=$3,lease_digest=$4,
 lease_until=$5,next_attempt_at=$5,updated_at=$6 WHERE source_id=$1 AND external_id=$2`,
 source,claim.id,claim.attempts,claim.digest,now.Add(fetchLease),now)
 if err!=nil{return nil,ErrWorker}
 // Admission survives process death and paces every worker on this database.
 _,err=tx.Exec(ctx,`INSERT INTO catalog_fetch_state(source_id,last_attempt_at,next_attempt_at,outcome,attempts)
 VALUES($1,$2,$3,'STARTED',1) ON CONFLICT(source_id) DO UPDATE SET
 last_attempt_at=excluded.last_attempt_at,next_attempt_at=excluded.next_attempt_at,outcome='STARTED',
 attempts=LEAST(catalog_fetch_state.attempts+1,1000000000)`,source,now,now.Add(fetchLease))
 if err!=nil{return nil,ErrWorker}
 if err=tx.Commit(ctx);err!=nil{return nil,ErrWorker}
 return claim,nil
}

// Lock order is import advisory lock, source, then job. Import's repeated source
// lock is reentrant in this transaction. An expired/replaced claim cannot write
// facts, move the high-water mark or acknowledge a newer claim.
func completeFetch(ctx context.Context,conn *pgx.Conn,claim fetchClaim,snapshot feed.Snapshot)error{
 tx,err:=conn.Begin(ctx)
 if err!=nil{return ErrWorker}
 defer rollbackFetch(tx)
 if _,err=tx.Exec(ctx,"SET LOCAL statement_timeout='2s'");err!=nil{return ErrWorker}
 if _,err=tx.Exec(ctx,`SELECT set_config('app.catalog_import_scope',$1,true)`,claim.scope);err!=nil{return ErrWorker}
 if _,err=tx.Exec(ctx,`SELECT pg_advisory_xact_lock(742019684)`);err!=nil{return ErrWorker}
 var source string
 err=tx.QueryRow(ctx,`SELECT id::text FROM data_sources WHERE id=$1 AND provider_key=$2
 AND enabled AND COALESCE(policy_url,'')<>'' FOR UPDATE`,claim.source,claim.provider).Scan(&source)
 if err!=nil{return ErrWorker}
 var id string
 err=tx.QueryRow(ctx,`SELECT external_id FROM catalog_fetch_jobs WHERE source_id=$1 AND external_id=$2
 AND state='LEASED' AND lease_digest=$3 AND import_scope=$4 AND lease_until>clock_timestamp() FOR UPDATE`,
 claim.source,claim.id,claim.digest,claim.scope).Scan(&id)
 if err!=nil{return ErrWorker}
 if claim.scope=="SCHEDULES"{
  var mapped string
  if err=tx.QueryRow(ctx,`SELECT anime_id::text FROM anime_external_ids WHERE provider=$1 AND external_id=$2 AND anime_id=$3 FOR UPDATE`,claim.provider,claim.id,claim.canonicalID).Scan(&mapped);err!=nil{return ErrWorker}
 }
 if _,err=applyTx(ctx,tx,snapshot,ImportOptions{SchedulesOnly:claim.scope=="SCHEDULES"});err!=nil{return ErrWorker}
 tag,err:=tx.Exec(ctx,`UPDATE catalog_fetch_jobs SET state='SUCCESS',attempts=0,lease_digest=NULL,
 lease_until=NULL,next_attempt_at=clock_timestamp()+($4::bigint*interval '1 second'),updated_at=clock_timestamp()
 WHERE source_id=$1 AND external_id=$2 AND state='LEASED' AND lease_digest=$3 AND import_scope=$5 AND lease_until>clock_timestamp()`,
 claim.source,claim.id,claim.digest,int64(claim.interval/time.Second),claim.scope)
 if err!=nil||tag.RowsAffected()!=1{return ErrWorker}
 _,err=tx.Exec(ctx,`UPDATE catalog_fetch_state SET outcome='SUCCESS',
 next_attempt_at=clock_timestamp()+interval '1 minute' WHERE source_id=$1`,claim.source)
 if err!=nil{return ErrWorker}
 if err=tx.Commit(ctx);err!=nil{return ErrWorker}
 return nil
}

// A bounded independent cleanup context persists failure/cooldown even when the
// fetch context was cancelled. Only allowlisted outcomes enter storage.
func failFetch(conn *pgx.Conn,claim fetchClaim,outcome string,until time.Time)error{
 if outcome!="FETCH_FAILED"&&outcome!="RATE_LIMITED"&&outcome!="IMPORT_FAILED"{return ErrWorker}
 ctx,cancel:=context.WithTimeout(context.Background(),5*time.Second)
 defer cancel()
 tx,err:=conn.Begin(ctx)
 if err!=nil{return ErrWorker}
 defer rollbackFetch(tx)
 if _,err=tx.Exec(ctx,"SET LOCAL statement_timeout='2s'");err!=nil{return ErrWorker}
 if _,err=tx.Exec(ctx,`SELECT set_config('app.catalog_import_scope',$1,true)`,claim.scope);err!=nil{return ErrWorker}
 var source string
 if err=tx.QueryRow(ctx,`SELECT id::text FROM data_sources WHERE id=$1 FOR UPDATE`,claim.source).Scan(&source);err!=nil{return ErrWorker}
 // Ordinary title failures do not delay other due titles for the title's
 // backoff; a provider Retry-After suppresses all titles, including new ones.
 sourceUntil:=time.Now().UTC().Add(sourceSpacing)
 if outcome=="RATE_LIMITED"&&until.After(sourceUntil){sourceUntil=until}
 _,err=tx.Exec(ctx,`UPDATE catalog_fetch_state SET outcome=$2,
 next_attempt_at=GREATEST(next_attempt_at,$3) WHERE source_id=$1`,claim.source,outcome,sourceUntil)
 if err!=nil{return ErrWorker}
 _,err=tx.Exec(ctx,`UPDATE catalog_fetch_jobs SET state=CASE WHEN attempts>=5 THEN 'FAILED' ELSE 'RETRY' END,
 lease_digest=NULL,lease_until=NULL,next_attempt_at=$4,updated_at=clock_timestamp()
 WHERE source_id=$1 AND external_id=$2 AND state='LEASED' AND lease_digest=$3 AND import_scope=$5 AND lease_until>clock_timestamp()`,
 claim.source,claim.id,claim.digest,until,claim.scope)
 if err!=nil{return ErrWorker}
 if err=tx.Commit(ctx);err!=nil{return ErrWorker}
 return nil
}

func validFetchID(provider,id string)bool{
 switch provider{
 case "wikidata":return fetchID.MatchString(id)
 case "tvmaze":_,_,err:=tvmaze.ParseID(id);return err==nil
 default:return false
 }
}
