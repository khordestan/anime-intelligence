package postgres_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/khordestan/anime-intelligence/internal/httpapi"
	"github.com/khordestan/anime-intelligence/internal/modules/catalog/feed"
	"github.com/khordestan/anime-intelligence/internal/platform/catalogsync"
	"github.com/khordestan/anime-intelligence/internal/platform/store"
)
func TestCatalogDetailsPublicAndConsistent(t *testing.T) {
	db,owner,_,_:=testDB(t)
	ctx:=context.Background()
	if _,err:=owner.Exec(ctx,`INSERT INTO data_sources(provider_key,display_name,enabled,policy_url) VALUES('details-fixture','Synthetic details',true,'https://example.test/policy')`);err!=nil {t.Fatal(err)}
	now:=time.Now().UTC().Truncate(time.Microsecond)
	s:=feed.Snapshot{Version:1,Provider:"details-fixture",ObservedAt:now,Titles:[]feed.Title{
		{ExternalID:"one",Title:"Synthetic Details One",MediaType:"TV",Status:"RELEASING",Synonyms:[]string{"Synthetic Alias"},Genres:[]string{"Adventure"},Tags:[]string{"Space"},Themes:[]string{"Synthetic theme"},Demographics:[]string{"Synthetic demographic"},Studios:[]string{"Synthetic Studio"},Relations:[]feed.Relation{{ExternalID:"two",Type:"SEQUEL"}},Episodes:[]feed.Episode{{Number:1,Track:"SUB",ScheduledAt:&now}}},
		{ExternalID:"two",Title:"Synthetic Details Two",MediaType:"TV",Status:"ANNOUNCED"},
	}}
	if _,err:=catalogsync.Import(ctx,owner,s,now);err!=nil {t.Fatal(err)}
	var id,target string
	if err:=owner.QueryRow(ctx,`SELECT anime_id::text FROM anime_external_ids WHERE provider='details-fixture' AND external_id='one'`).Scan(&id);err!=nil {t.Fatal(err)}
	if err:=owner.QueryRow(ctx,`SELECT anime_id::text FROM anime_external_ids WHERE provider='details-fixture' AND external_id='two'`).Scan(&target);err!=nil {t.Fatal(err)}
	matches,err:=db.Repository("").Catalog(ctx,"Synthetic Alias",24,0)
	if err!=nil || len(matches)!=1 || matches[0].ID!=id {t.Fatal("imported alias search",matches,err)}
	h:=httpapi.NewPostgresRouter(slog.New(slog.NewTextHandler(io.Discard,nil)),db)
	w:=httptest.NewRecorder()
	h.ServeHTTP(w,httptest.NewRequest("GET","/api/v1/anime/"+id+"/details",nil))
	var out store.CatalogDetails
	if w.Code!=200 || json.Unmarshal(w.Body.Bytes(),&out)!=nil {t.Fatalf("detail response: %d %s",w.Code,w.Body.String())}
	if out.Anime.ID!=id || out.Provider!="details-fixture" || out.ObservedAt==nil || !out.ObservedAt.Equal(now) || len(out.Genres)!=1 || len(out.Synonyms)!=1 || len(out.Tags)!=1 || len(out.Themes)!=1 || out.Themes[0]!="Synthetic theme" || len(out.Demographics)!=1 || out.Demographics[0]!="Synthetic demographic" || len(out.Studios)!=1 || len(out.Relations)!=1 || out.Relations[0].AnimeID!=target || out.Relations[0].Title!="Synthetic Details Two" || out.Relations[0].Status!="ANNOUNCED" || out.Relations[0].MediaType!="TV" || len(out.Episodes)!=1 || out.Episodes[0].Revision!=1 || out.Episodes[0].ReleasedAt!=nil {t.Fatalf("incomplete details: %+v",out)}
	reverse,found,reverseErr:=db.Repository("").CatalogDetails(ctx,target)
	if reverseErr!=nil||!found||len(reverse.Relations)!=1||reverse.Relations[0].AnimeID!=id||reverse.Relations[0].Type!="PREQUEL"||reverse.Relations[0].Title!="Synthetic Details One"{t.Fatal("one-sided season relation lost reverse navigation",reverse,reverseErr)}
	if w.Header().Get("Cache-Control")!="no-store" {t.Fatal("catalog response cache boundary lost")}
	for path,want:=range map[string]int{"/api/v1/anime/not-a-uuid/details":400,"/api/v1/anime/00000000-0000-4000-8000-000000000000/details":404} {
		w:=httptest.NewRecorder();h.ServeHTTP(w,httptest.NewRequest("GET",path,nil));if w.Code!=want {t.Fatal(path,w.Code)}
	}
	out,ok,err:=db.Repository("").CatalogDetails(ctx,"10000000-0000-4000-8000-000000000001")
	if err!=nil || !ok || out.Provider!="" || out.Genres==nil || out.Themes==nil || out.Demographics==nil || out.Episodes==nil || out.Relations==nil {t.Fatal("seed details should have empty arrays",err)}
}

func TestCatalogContinuityDetailsDeduplicateReciprocalRelations(t *testing.T){
 db,owner,_,_:=testDB(t)
 ctx:=context.Background()
 now:=time.Now().UTC().Truncate(time.Microsecond)
 if _,err:=owner.Exec(ctx,`INSERT INTO data_sources(provider_key,display_name,enabled,policy_url) VALUES('continuity-details','Synthetic continuity',true,'https://example.test/policy')`);err!=nil{t.Fatal(err)}
 snapshot:=feed.Snapshot{Version:1,Provider:"continuity-details",ObservedAt:now,Titles:[]feed.Title{
 {ExternalID:"one",Title:"Synthetic Continuity One",MediaType:"TV",Status:"FINISHED",Relations:[]feed.Relation{{ExternalID:"two",Type:"SEQUEL"},{ExternalID:"side",Type:"SIDE_STORY"}}},
 {ExternalID:"two",Title:"Synthetic Continuity Two",MediaType:"MOVIE",Status:"ANNOUNCED",Relations:[]feed.Relation{{ExternalID:"one",Type:"PREQUEL"}}},
 {ExternalID:"side",Title:"Synthetic Side Story",MediaType:"OVA",Status:"UNKNOWN"},
 }}
 if _,err:=catalogsync.Import(ctx,owner,snapshot,now);err!=nil{t.Fatal(err)}
 ids:=map[string]string{}
 for _,key:=range []string{"one","two","side"}{
  var id string
  if err:=owner.QueryRow(ctx,`SELECT anime_id::text FROM anime_external_ids WHERE provider='continuity-details' AND external_id=$1`,key).Scan(&id);err!=nil{t.Fatal(err)}
  ids[key]=id
 }
 for _,fixture:=range []struct{key string;count int}{{"one",2},{"two",1},{"side",0}}{
  details,ok,err:=db.Repository("").CatalogDetails(ctx,ids[fixture.key])
  if err!=nil||!ok||len(details.Relations)!=fixture.count{t.Fatal("reciprocal duplicate or invented side-story inverse",fixture,details,err)}
  if fixture.key=="one"&&(details.Relations[0].AnimeID!=ids["two"]||details.Relations[0].Type!="SEQUEL"||details.Relations[0].MediaType!="MOVIE"){t.Fatal("named related media",details.Relations)}
 }
}
