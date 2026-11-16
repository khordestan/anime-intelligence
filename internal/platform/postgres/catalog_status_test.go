package postgres_test

import (
	"context"
	"net/url"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/khordestan/anime-intelligence/internal/modules/catalog/feed"
	"github.com/khordestan/anime-intelligence/internal/platform/catalogsync"
)
func TestCatalogStatusFreshnessWithoutPrivateData(t *testing.T) {
	_,owner,_,runtimeDSN:=testDB(t)
	ctx:=context.Background()
	now:=time.Now().UTC().Truncate(time.Microsecond)
	for _,provider:=range []string{"stale","never","disabled","future"} {
		if _,err:=owner.Exec(ctx,`INSERT INTO data_sources(provider_key,display_name,enabled,policy_url) VALUES($1,'Synthetic status',$2,'https://example.test/policy')`,provider,provider!="disabled");err!=nil {t.Fatal(err)}
	}
	released:=now.Add(-3*time.Hour)
	scheduled:=now.Add(time.Hour)
	s:=feed.Snapshot{Version:1,Provider:"stale",ObservedAt:now.Add(-2*time.Hour),Titles:[]feed.Title{{ExternalID:"one",Title:"Synthetic Status",MediaType:"TV",Status:"RELEASING",Episodes:[]feed.Episode{{Number:1,Track:"SUB",ReleasedAt:&released},{Number:2,Track:"SUB",ScheduledAt:&scheduled}}}}}
	if _,err:=catalogsync.Import(ctx,owner,s,now);err!=nil {t.Fatal(err)}
	if _,err:=owner.Exec(ctx,`INSERT INTO catalog_sync_state(source_id,observed_at) SELECT id,$1 FROM data_sources WHERE provider_key='future'`,now.Add(time.Hour));err!=nil {t.Fatal(err)}
	u,err:=url.Parse(runtimeDSN)
	if err!=nil {t.Fatal(err)}
	if _,err=owner.Exec(ctx,`GRANT SELECT ON data_sources,catalog_sync_state,catalog_fetch_state TO `+pgx.Identifier{u.User.Username()}.Sanitize());err!=nil {t.Fatal(err)}
	reader,err:=pgx.Connect(ctx,runtimeDSN)
	if err!=nil {t.Fatal(err)}
	defer reader.Close(ctx)
	status,err:=catalogsync.Inspect(ctx,reader,now,time.Hour)
	if err!=nil || status.StaleSources!=3 || status.Titles!=9 || status.EpisodeSchedules!=2 || status.ConfirmedEpisodes!=1 || len(status.Sources)!=4 {t.Fatal("catalog freshness",status,err)}
	for _,source:=range status.Sources {
		switch source.Provider {
		case "stale":if !source.Stale || source.Titles!=1 || source.AgeSeconds==nil || *source.AgeSeconds!=7200 {t.Fatal("stale age",source)}
		case "never":if !source.NeverSynced || !source.Stale || source.AgeSeconds!=nil {t.Fatal("never synced",source)}
		case "disabled":if source.Stale || source.Enabled {t.Fatal("disabled source",source)}
		case "future":if !source.FutureObservation || !source.Stale || source.AgeSeconds==nil || *source.AgeSeconds!=0 {t.Fatal("future clock",source)}
		}
	}
	broader,err:=catalogsync.Inspect(ctx,reader,now,3*time.Hour)
	if err!=nil || broader.StaleSources!=2 {t.Fatal("freshness threshold did not distinguish a valid older observation",broader,err)}
	if _,err=catalogsync.Inspect(ctx,reader,now,0);err!=catalogsync.ErrStatus {t.Fatal("invalid freshness threshold",err)}
}

func TestCatalogStatusPartialFeedsAndMetadataCoverage(t *testing.T){
 _,owner,_,runtimeDSN:=testDB(t)
 ctx:=context.Background()
 now:=time.Now().UTC().Truncate(time.Microsecond)
 if _,err:=owner.Exec(ctx,`INSERT INTO data_sources(provider_key,display_name,enabled,policy_url) VALUES('partial-status','Synthetic partial status',true,'https://example.test/policy')`);err!=nil{t.Fatal(err)}
 zero:=0
 rich:=feed.Title{ExternalID:"rich",Title:"Synthetic Rich Metadata",MediaType:"TV",Status:"UNKNOWN",EpisodeCount:&zero,Synopsis:"Synthetic reference synopsis",Synonyms:[]string{"Synthetic alias"},Genres:[]string{"Synthetic genre"},Tags:[]string{"Synthetic tag"},Studios:[]string{"Synthetic studio"},Relations:[]feed.Relation{{ExternalID:"old",Type:"SEQUEL"}}}
 snapshot:=feed.Snapshot{Version:1,Provider:"partial-status",ObservedAt:now.Add(-2*time.Hour),Titles:[]feed.Title{
 {ExternalID:"old",Title:"Synthetic Old Metadata",MediaType:"TV",Status:"UNKNOWN"},rich,
 {ExternalID:"boundary",Title:"Synthetic Boundary Metadata",MediaType:"TV",Status:"UNKNOWN"},
 {ExternalID:"future",Title:"Synthetic Future Metadata",MediaType:"TV",Status:"UNKNOWN"},
 }}
 if _,err:=catalogsync.Import(ctx,owner,snapshot,now);err!=nil{t.Fatal(err)}
 snapshot.ObservedAt=now.Add(-30*time.Minute);snapshot.Titles=[]feed.Title{rich}
 if _,err:=catalogsync.Import(ctx,owner,snapshot,now);err!=nil{t.Fatal(err)}
 for external,stamp:=range map[string]time.Time{"boundary":now.Add(-time.Hour),"future":now.Add(time.Hour)}{
  if _,err:=owner.Exec(ctx,`UPDATE catalog_metadata SET observed_at=$1 WHERE anime_id=(SELECT anime_id FROM anime_external_ids WHERE provider='partial-status' AND external_id=$2)`,stamp,external);err!=nil{t.Fatal(err)}
 }
 u,err:=url.Parse(runtimeDSN)
 if err!=nil{t.Fatal(err)}
 if _,err=owner.Exec(ctx,`GRANT SELECT ON data_sources,catalog_sync_state,catalog_fetch_state TO `+pgx.Identifier{u.User.Username()}.Sanitize());err!=nil{t.Fatal(err)}
 reader,err:=pgx.Connect(ctx,runtimeDSN)
 if err!=nil{t.Fatal(err)}
 defer reader.Close(ctx)
 status,err:=catalogsync.Inspect(ctx,reader,now,time.Hour)
 if err!=nil||len(status.Sources)!=1{t.Fatal("partial inspection",status,err)}
 source:=status.Sources[0]
 if source.Stale||source.Titles!=4||source.StaleTitles!=2||source.FutureTitles!=1{t.Fatal("fresh partial provider hid old titles",source)}
 if source.OldestTitleObservation==nil||!source.OldestTitleObservation.Equal(now.Add(-2*time.Hour))||source.NewestTitleObservation==nil||!source.NewestTitleObservation.Equal(now.Add(time.Hour)){t.Fatal("title observation range",source)}
 if status.ObservedTitles!=4||status.UnobservedTitles!=8||status.Titles!=12||status.StaleTitles!=2||status.FutureTitles!=1{t.Fatal("catalog observation counts",status)}
 want:=catalogsync.MetadataCoverage{EpisodeTotals:1,Synopses:1,Synonyms:1,Genres:1,Tags:1,Studios:1,Relations:1}
 if source.Coverage!=want||status.Coverage!=want{t.Fatal("presence coverage/known zero",source.Coverage,status.Coverage)}
 broader,err:=catalogsync.Inspect(ctx,reader,now,3*time.Hour)
 if err!=nil||broader.StaleTitles!=1||broader.FutureTitles!=1{t.Fatal("title threshold/future distinction",broader,err)}
 if _,err=owner.Exec(ctx,`UPDATE data_sources SET enabled=false WHERE provider_key='partial-status'`);err!=nil{t.Fatal(err)}
 disabled,err:=catalogsync.Inspect(ctx,reader,now,time.Hour)
 if err!=nil||disabled.StaleSources!=0||disabled.StaleTitles!=2{t.Fatal("disabled source hid metadata age",disabled,err)}
}
