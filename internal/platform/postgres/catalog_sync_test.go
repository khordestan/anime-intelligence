package postgres_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/khordestan/anime-intelligence/internal/modules/catalog/feed"
	"github.com/khordestan/anime-intelligence/internal/platform/catalogsync"
)

func TestCatalogImportPersistence(t *testing.T) {
	db,owner,_,runtimeDSN:=testDB(t)
	ctx:=context.Background()
	now:=time.Now().UTC().Truncate(time.Microsecond)
	s:=feed.Snapshot{Version:1,Provider:"synthetic-feed",ObservedAt:now.Add(-3*time.Hour),Titles:[]feed.Title{{
		ExternalID:"sky-1",Title:"Synthetic Sky",MediaType:"TV",Status:"RELEASING",
		Genres:[]string{"Adventure"},Relations:[]feed.Relation{{ExternalID:"sky-2",Type:"SEQUEL"}},
		Episodes:[]feed.Episode{{Number:1,Track:"SUB"}},
	}}}
	if _,err:=catalogsync.Import(ctx,owner,s,now);!errors.Is(err,catalogsync.ErrSource) {t.Fatal("unapproved provider accepted",err)}
	if _,err:=owner.Exec(ctx,`INSERT INTO data_sources(provider_key,display_name,enabled,policy_url) VALUES('synthetic-feed','Synthetic fixture',true,'https://example.test/policy')`);err!=nil {t.Fatal(err)}
	first,err:=catalogsync.Import(ctx,owner,s,now)
	if err!=nil || first.Titles!=1 || first.ScheduleRevisions!=1 || first.ReleaseEvents!=0 || first.UnresolvedRelations!=1 {t.Fatalf("baseline: %+v %v",first,err)}
	var id string
	if err=owner.QueryRow(ctx,`SELECT anime_id::text FROM anime_external_ids WHERE provider='synthetic-feed' AND external_id='sky-1'`).Scan(&id);err!=nil {t.Fatal(err)}
	a,ok,err:=db.Repository("").GetAnime(ctx,id)
	if err!=nil || !ok || a.Title!="Synthetic Sky" || a.EpisodeCountKnown {t.Fatal("canonical import not visible",err)}
	at:=now.Add(-2*time.Hour)
	s.ObservedAt=now.Add(-time.Hour)
	s.Titles[0].Episodes[0].ReleasedAt=&at
	second,err:=catalogsync.Import(ctx,owner,s,now)
	if err!=nil || second.ScheduleRevisions!=1 || second.ReleaseEvents!=1 {t.Fatalf("release transition: %+v %v",second,err)}
	if _,err=catalogsync.Import(ctx,owner,s,now);!errors.Is(err,catalogsync.ErrStale) {t.Fatal("repeated observation accepted",err)}
	s.ObservedAt=now.Add(-30*time.Minute)
	s.Titles=nil
	if _,err=catalogsync.Import(ctx,owner,s,now);err!=nil {t.Fatal("partial empty feed",err)}
	corrected:=at.Add(time.Minute)
	s.ObservedAt=now.Add(-20*time.Minute)
	s.Titles=[]feed.Title{{ExternalID:"sky-1",Title:"Synthetic Sky Revised",MediaType:"TV",Status:"RELEASING",Episodes:[]feed.Episode{{Number:1,Track:"SUB",ReleasedAt:&corrected}}}}
	result,err:=catalogsync.Import(ctx,owner,s,now)
	if err!=nil || result.ReleaseEvents!=0 || result.ScheduleRevisions!=1 {t.Fatalf("partial/correction duplicated alert: %+v %v",result,err)}
	var events,revisions int
	if err=owner.QueryRow(ctx,`SELECT count(*) FROM domain_events WHERE aggregate_id=$1`,id).Scan(&events);err!=nil || events!=1 {t.Fatal("event ledger",events,err)}
	if err=owner.QueryRow(ctx,`SELECT count(*) FROM airing_schedule WHERE anime_id=$1`,id).Scan(&revisions);err!=nil || revisions!=3 {t.Fatal("revision history",revisions,err)}
	// Disappearing confirmation must never erase an established release.
	s.ObservedAt=now.Add(-10*time.Minute);s.Titles[0].Episodes[0].ReleasedAt=nil
	result,err=catalogsync.Import(ctx,owner,s,now)
	if err!=nil || result.ScheduleRevisions!=0 || result.ReleaseEvents!=0 {t.Fatal("missing confirmation retracted release",err)}
	// Runtime credentials cannot mutate catalog state, even via this library.
	raw,err:=pgx.Connect(ctx,runtimeDSN)
	if err!=nil {t.Fatal(err)}
	defer raw.Close(ctx)
	s.ObservedAt=now
	if _,err=catalogsync.Import(ctx,raw,s,now);!errors.Is(err,catalogsync.ErrImport) {t.Fatal("restricted role imported data",err)}
	if _,err=owner.Exec(ctx,`UPDATE data_sources SET enabled=false WHERE provider_key='synthetic-feed'`);err!=nil {t.Fatal(err)}
	if _,err=catalogsync.Import(ctx,owner,s,now);!errors.Is(err,catalogsync.ErrSource) {t.Fatal("disabled provider accepted",err)}
}
func TestCatalogImportGraphRollbackAndResolution(t *testing.T) {
	_,owner,_,_:=testDB(t)
	ctx:=context.Background()
	now:=time.Now().UTC().Truncate(time.Microsecond)
	if _,err:=owner.Exec(ctx,`INSERT INTO data_sources(provider_key,display_name,enabled,policy_url) VALUES('synthetic-graph','Synthetic graph',true,'https://example.test/policy')`);err!=nil {t.Fatal(err)}
	s:=feed.Snapshot{Version:1,Provider:"synthetic-graph",ObservedAt:now.Add(-time.Hour),Titles:[]feed.Title{{
		ExternalID:"one",Title:"Synthetic One",MediaType:"TV",Status:"ANNOUNCED",Relations:[]feed.Relation{{ExternalID:"two",Type:"SEQUEL"}},
	}}}
	if _,err:=catalogsync.Import(ctx,owner,s,now);err!=nil {t.Fatal(err)}
	s.ObservedAt=now
	s.Titles=[]feed.Title{{ExternalID:"two",Title:"Synthetic Two",MediaType:"TV",Status:"ANNOUNCED",Relations:[]feed.Relation{{ExternalID:"one",Type:"SEQUEL"}}}}
	if _,err:=catalogsync.Import(ctx,owner,s,now);!errors.Is(err,catalogsync.ErrConflict) {t.Fatal("cycle across separate snapshots accepted",err)}
	var count int
	if err:=owner.QueryRow(ctx,`SELECT count(*) FROM anime_external_ids WHERE provider='synthetic-graph' AND external_id='two'`).Scan(&count);err!=nil || count!=0 {t.Fatal("failed graph import partially committed",count,err)}
	s.Titles[0].Relations[0].Type="PREQUEL"
	result,err:=catalogsync.Import(ctx,owner,s,now)
	if err!=nil || result.UnresolvedRelations!=0 {t.Fatal("retry after rollback did not resolve retained relation",err)}
	if err=owner.QueryRow(ctx,`SELECT count(*) FROM anime_relations WHERE source_id=(SELECT id FROM data_sources WHERE provider_key='synthetic-graph')`).Scan(&count);err!=nil || count!=2 {t.Fatal("reciprocal graph not persisted",count,err)}
}
func TestCatalogImportAuthorityAndHistoricalBaseline(t *testing.T) {
	_,owner,_,_:=testDB(t)
	ctx:=context.Background()
	now:=time.Now().UTC().Truncate(time.Microsecond)
	for _,provider:=range []string{"source-a","source-b"} {
		if _,err:=owner.Exec(ctx,`INSERT INTO data_sources(provider_key,display_name,enabled,policy_url) VALUES($1,'Synthetic authority',true,'https://example.test/policy')`,provider);err!=nil {t.Fatal(err)}
	}
	at:=now.Add(-2*time.Hour)
	s:=feed.Snapshot{Version:1,Provider:"source-a",ObservedAt:now.Add(-time.Hour),Titles:[]feed.Title{{ExternalID:"one",Title:"Synthetic Authority",MediaType:"TV",Status:"FINISHED",Episodes:[]feed.Episode{{Number:1,Track:"RAW",ReleasedAt:&at}}}}}
	out,err:=catalogsync.Import(ctx,owner,s,now)
	if err!=nil || out.ReleaseEvents!=0 {t.Fatal("historical first release generated alert",err)}
	var id string
	if err=owner.QueryRow(ctx,`SELECT anime_id::text FROM anime_external_ids WHERE provider='source-a' AND external_id='one'`).Scan(&id);err!=nil {t.Fatal(err)}
	if _,err=owner.Exec(ctx,`INSERT INTO anime_external_ids(anime_id,provider,external_id) VALUES($1,'source-b','other')`,id);err!=nil {t.Fatal(err)}
	s.Provider="source-b";s.ObservedAt=now;s.Titles[0].ExternalID="other";s.Titles[0].Title="Conflicting source title"
	if _,err=catalogsync.Import(ctx,owner,s,now);!errors.Is(err,catalogsync.ErrConflict) {t.Fatal("secondary source overwrote authority",err)}
	var title string
	if err=owner.QueryRow(ctx,`SELECT canonical_title FROM anime WHERE id=$1`,id).Scan(&title);err!=nil || title!="Synthetic Authority" {t.Fatal("authority conflict changed canonical data",err)}
}

func TestPartialProviderUpdatesUseEachTitlesObservationTime(t *testing.T) {
	_,owner,_,_:=testDB(t)
	ctx:=context.Background()
	now:=time.Now().UTC().Truncate(time.Microsecond)
	if _,err:=owner.Exec(ctx,`INSERT INTO data_sources(provider_key,display_name,enabled,policy_url) VALUES('partial-clock','Synthetic partial clock',true,'https://example.test/policy')`);err!=nil {t.Fatal(err)}
	s:=feed.Snapshot{Version:1,Provider:"partial-clock",ObservedAt:now.Add(-4*time.Hour),Titles:[]feed.Title{
		{ExternalID:"one",Title:"Synthetic Clock One",MediaType:"TV",Status:"RELEASING",Episodes:[]feed.Episode{{Number:1,Track:"SUB"}}},
		{ExternalID:"two",Title:"Synthetic Clock Two",MediaType:"TV",Status:"RELEASING"},
	}}
	if _,err:=catalogsync.Import(ctx,owner,s,now);err!=nil {t.Fatal(err)}
	s.ObservedAt=now.Add(-time.Hour)
	s.Titles=s.Titles[1:]
	if _,err:=catalogsync.Import(ctx,owner,s,now);err!=nil {t.Fatal(err)}
	at:=now.Add(-2*time.Hour)
	s.ObservedAt=now
	s.Titles=[]feed.Title{{ExternalID:"one",Title:"Synthetic Clock One",MediaType:"TV",Status:"RELEASING",Episodes:[]feed.Episode{{Number:1,Track:"SUB",ReleasedAt:&at}}}}
	result,err:=catalogsync.Import(ctx,owner,s,now)
	if err!=nil || result.ReleaseEvents!=1 {t.Fatal("another title's newer observation hid an eligible release",result,err)}
}
