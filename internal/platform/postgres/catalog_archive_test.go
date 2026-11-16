package postgres_test

import (
 "context"
 "encoding/json"
 "errors"
 "testing"
 "time"

 "github.com/jackc/pgx/v5"
 "github.com/khordestan/anime-intelligence/internal/modules/catalog/feed"
 "github.com/khordestan/anime-intelligence/internal/platform/catalogsync"
)
func TestCatalogArchivePreviewHistoryAndAtomicFailure(t *testing.T){
 db,owner,_,runtimeDSN:=testDB(t);ctx:=context.Background()
 now:=time.Now().UTC().Truncate(time.Microsecond)
 if _,err:=owner.Exec(ctx,`INSERT INTO data_sources(provider_key,display_name,enabled,policy_url) VALUES('archive-fixture','Synthetic archive',true,'https://example.test/policy')`);err!=nil{t.Fatal(err)}
 snapshot:=feed.Snapshot{Version:1,Provider:"archive-fixture",ObservedAt:now.Add(-3*time.Hour),Titles:[]feed.Title{{ExternalID:"one",Title:"Synthetic Baseline",MediaType:"TV",Status:"RELEASING",Episodes:[]feed.Episode{{Number:1,Track:"SUB"}}}}}
 if result,err:=catalogsync.Import(ctx,owner,snapshot,now);err!=nil||result.ArchivedRecords!=0||result.Preview{t.Fatal("default import changed",result,err)}
 var id string
 if err:=owner.QueryRow(ctx,`SELECT anime_id::text FROM anime_external_ids WHERE provider='archive-fixture' AND external_id='one'`).Scan(&id);err!=nil{t.Fatal(err)}
 released:=now.Add(-150*time.Minute)
 snapshot.ObservedAt=now.Add(-2*time.Hour)
 snapshot.Titles[0].Title="Synthetic Archived One";snapshot.Titles[0].Synopsis="Synthetic original synopsis"
 snapshot.Titles[0].Episodes[0].ReleasedAt=&released
 preview,err:=catalogsync.ImportWithOptions(ctx,owner,snapshot,now,catalogsync.ImportOptions{Preview:true,Archive:true})
 if err!=nil||!preview.Preview||preview.ArchivedRecords!=1||preview.ReleaseEvents!=1{t.Fatal("full transactional preview",preview,err)}
 assertState:=func(title string,observed time.Time,archives,events int){
  t.Helper()
  a,found,err:=db.Repository("").GetAnime(ctx,id)
  if err!=nil||!found||a.Title!=title{t.Fatal("catalog mutation boundary",a,err)}
  var at time.Time;var count int
  if err=owner.QueryRow(ctx,`SELECT observed_at FROM catalog_sync_state WHERE source_id=(SELECT id FROM data_sources WHERE provider_key='archive-fixture')`).Scan(&at);err!=nil||!at.Equal(observed){t.Fatal("source high-water boundary",at,err)}
  if err=owner.QueryRow(ctx,`SELECT count(*) FROM source_records WHERE record_type='CATALOG_METADATA'`).Scan(&count);err!=nil||count!=archives{t.Fatal("archive mutation boundary",count,err)}
  if err=owner.QueryRow(ctx,`SELECT count(*) FROM domain_events WHERE aggregate_id=$1`,id).Scan(&count);err!=nil||count!=events{t.Fatal("event mutation boundary",count,err)}
 }
 assertState("Synthetic Baseline",now.Add(-3*time.Hour),0,0)
 first,err:=catalogsync.ImportWithOptions(ctx,owner,snapshot,now,catalogsync.ImportOptions{Archive:true})
 if err!=nil||first.Preview||first.ArchivedRecords!=1||first.ReleaseEvents!=1{t.Fatal("archived import",first,err)}
 snapshot.ObservedAt=now.Add(-time.Hour);snapshot.Titles[0].Title="Synthetic Archived Two";snapshot.Titles[0].Synopsis="Synthetic revised synopsis"
 if _,err=catalogsync.ImportWithOptions(ctx,owner,snapshot,now,catalogsync.ImportOptions{Archive:true});err!=nil{t.Fatal(err)}
 assertState("Synthetic Archived Two",snapshot.ObservedAt,2,1)
 var old []byte
 if err=owner.QueryRow(ctx,`SELECT payload FROM source_records WHERE record_type='CATALOG_METADATA' ORDER BY payload->>'observedAt' LIMIT 1`).Scan(&old);err!=nil{t.Fatal(err)}
 var record struct{Kind string `json:"kind"`;AnimeID string `json:"animeId"`;ObservedAt time.Time `json:"observedAt"`;Title feed.Title `json:"title"`}
 if json.Unmarshal(old,&record)!=nil||record.Kind!="CATALOG_METADATA"||record.AnimeID!=id||record.Title.Title!="Synthetic Archived One"||record.Title.Synopsis!="Synthetic original synopsis"||!record.ObservedAt.Equal(now.Add(-2*time.Hour)){t.Fatal("earlier selected observation lost")}
 if _,err=catalogsync.ImportWithOptions(ctx,owner,snapshot,now,catalogsync.ImportOptions{Archive:true});!errors.Is(err,catalogsync.ErrStale){t.Fatal("stale archived replay accepted",err)}
 // Fail late, after the first title's facts/event/archive and a second ID were
 // inserted. The shared transaction must retain neither partial catalog data.
 if _,err=owner.Exec(ctx,`CREATE FUNCTION synthetic_archive_fail() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.record_type='CATALOG_METADATA' AND NEW.external_id='late' THEN RAISE EXCEPTION 'synthetic archive failure'; END IF; RETURN NEW; END $$;
 CREATE TRIGGER synthetic_archive_fail BEFORE INSERT ON source_records FOR EACH ROW EXECUTE FUNCTION synthetic_archive_fail()`);err!=nil{t.Fatal(err)}
 snapshot.ObservedAt=now.Add(-30*time.Minute);snapshot.Titles[0].Title="Synthetic Rolled Back"
 secondRelease:=now.Add(-40*time.Minute)
 snapshot.Titles[0].Episodes=append(snapshot.Titles[0].Episodes,feed.Episode{Number:2,Track:"SUB",ReleasedAt:&secondRelease})
 snapshot.Titles=append(snapshot.Titles,feed.Title{ExternalID:"late",Title:"Synthetic Late Failure",MediaType:"TV",Status:"UNKNOWN"})
 if _,err=catalogsync.ImportWithOptions(ctx,owner,snapshot,now,catalogsync.ImportOptions{Archive:true});!errors.Is(err,catalogsync.ErrImport){t.Fatal("archive failure not atomic",err)}
 assertState("Synthetic Archived Two",now.Add(-time.Hour),2,1)
 var count int
 if err=owner.QueryRow(ctx,`SELECT count(*) FROM anime_external_ids WHERE provider='archive-fixture' AND external_id='late'`).Scan(&count);err!=nil||count!=0{t.Fatal("failed batch retained new identity",err)}
 if err=owner.QueryRow(ctx,`SELECT count(*) FROM airing_schedule WHERE anime_id=$1 AND episode_number=2`,id).Scan(&count);err!=nil||count!=0{t.Fatal("failed batch retained schedule",err)}
 if _,err=owner.Exec(ctx,`DROP TRIGGER synthetic_archive_fail ON source_records; DROP FUNCTION synthetic_archive_fail()`);err!=nil{t.Fatal(err)}
 snapshot.Titles=snapshot.Titles[:1]
 if _,err=catalogsync.Import(ctx,owner,snapshot,now);err!=nil{t.Fatal("non-archive import",err)}
 assertState("Synthetic Rolled Back",snapshot.ObservedAt,2,2)
 raw,err:=pgx.Connect(ctx,runtimeDSN);if err!=nil{t.Fatal(err)};defer raw.Close(ctx)
 if _,err=raw.Exec(ctx,`SELECT payload FROM source_records`);err==nil{t.Fatal("API role can read archived provider payloads")}
}
