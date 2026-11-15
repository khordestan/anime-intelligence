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
