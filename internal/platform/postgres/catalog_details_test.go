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
		{ExternalID:"one",Title:"Synthetic Details One",MediaType:"TV",Status:"RELEASING",Synonyms:[]string{"Synthetic Alias"},Genres:[]string{"Adventure"},Tags:[]string{"Space"},Studios:[]string{"Synthetic Studio"},Relations:[]feed.Relation{{ExternalID:"two",Type:"SEQUEL"}},Episodes:[]feed.Episode{{Number:1,Track:"SUB",ScheduledAt:&now}}},
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
	if out.Anime.ID!=id || out.Provider!="details-fixture" || out.ObservedAt==nil || !out.ObservedAt.Equal(now) || len(out.Genres)!=1 || len(out.Synonyms)!=1 || len(out.Tags)!=1 || len(out.Studios)!=1 || len(out.Relations)!=1 || out.Relations[0].AnimeID!=target || len(out.Episodes)!=1 || out.Episodes[0].Revision!=1 || out.Episodes[0].ReleasedAt!=nil {t.Fatalf("incomplete details: %+v",out)}
	if w.Header().Get("Cache-Control")!="no-store" {t.Fatal("catalog response cache boundary lost")}
	for path,want:=range map[string]int{"/api/v1/anime/not-a-uuid/details":400,"/api/v1/anime/00000000-0000-4000-8000-000000000000/details":404} {
		w:=httptest.NewRecorder();h.ServeHTTP(w,httptest.NewRequest("GET",path,nil));if w.Code!=want {t.Fatal(path,w.Code)}
	}
	out,ok,err:=db.Repository("").CatalogDetails(ctx,"10000000-0000-4000-8000-000000000001")
	if err!=nil || !ok || out.Provider!="" || out.Genres==nil || out.Episodes==nil || out.Relations==nil {t.Fatal("seed details should have empty arrays",err)}
}
