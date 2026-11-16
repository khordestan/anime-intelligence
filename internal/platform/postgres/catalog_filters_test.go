package postgres_test

import(
 "context"
 "encoding/json"
 "errors"
 "io"
 "log/slog"
 "net/http/httptest"
 "net/url"
 "testing"
 "time"

 "github.com/khordestan/anime-intelligence/internal/httpapi"
 "github.com/khordestan/anime-intelligence/internal/modules/catalog/feed"
 "github.com/khordestan/anime-intelligence/internal/platform/catalogsync"
 "github.com/khordestan/anime-intelligence/internal/platform/store"
)
func TestPublicCatalogFiltersCombineSourcedCategoriesAndPagination(t *testing.T){
 db,owner,_,_:=testDB(t);ctx:=context.Background();now:=time.Now().UTC()
 if _,err:=owner.Exec(ctx,`INSERT INTO data_sources(provider_key,display_name,enabled,policy_url) VALUES('filter-fixture','Synthetic filters',true,'https://example.test/policy')`);err!=nil{t.Fatal(err)}
 const literal="100%_';--"
 snapshot:=feed.Snapshot{Version:1,Provider:"filter-fixture",ObservedAt:now,Titles:[]feed.Title{
  {ExternalID:"a",Title:"Synthetic Filter Alpha",NativeTitle:"Synthetic Native Alias",MediaType:"TV",Status:"RELEASING",ReleaseYear:2026,Season:"FALL",Synonyms:[]string{"Hidden Synthetic Alias"},Genres:[]string{"Action",literal},Themes:[]string{"Strategy"},Demographics:[]string{"Seinen"},Studios:[]string{"Synthetic Works"}},
  {ExternalID:"b",Title:"Synthetic Filter Beta",MediaType:"MOVIE",Status:"FINISHED",ReleaseYear:2025,Season:"WINTER",Genres:[]string{"Action"},Themes:[]string{"Mystery"},Demographics:[]string{"Seinen"},Studios:[]string{"Other Works"}},
  {ExternalID:"c",Title:"Synthetic Filter Gamma",MediaType:"TV",Status:"RELEASING",ReleaseYear:2026,Season:"FALL",Genres:[]string{"Action"},Tags:[]string{"Strategy"}},
 }}
 if _,err:=catalogsync.Import(ctx,owner,snapshot,now);err!=nil{t.Fatal(err)}
 if _,err:=owner.Exec(ctx,`INSERT INTO anime(canonical_title) VALUES('Synthetic Filter Delta')`);err!=nil{t.Fatal(err)}
 repo:=db.Repository("")
 check:=func(f store.CatalogFilter,want ...string){
  t.Helper()
  if f.Query==""{f.Query="Synthetic Filter"}
  items,err:=repo.FilterCatalog(ctx,f,100,0)
  if err!=nil||len(items)!=len(want){t.Fatal("filter matches",f,len(items),want,err)}
  for i,title:=range want{if items[i].Title!=title{t.Fatal("filter order",items,want)}}
 }
 check(store.CatalogFilter{},"Synthetic Filter Alpha","Synthetic Filter Beta","Synthetic Filter Delta","Synthetic Filter Gamma")
 check(store.CatalogFilter{Genre:" action "},"Synthetic Filter Alpha","Synthetic Filter Beta","Synthetic Filter Gamma")
 check(store.CatalogFilter{Theme:"strategy"},"Synthetic Filter Alpha")
 check(store.CatalogFilter{Studio:"synthetic works"},"Synthetic Filter Alpha")
 check(store.CatalogFilter{Demographic:"seinen"},"Synthetic Filter Alpha","Synthetic Filter Beta")
 check(store.CatalogFilter{Genre:"Action",Theme:"Strategy",Studio:"Synthetic Works",Demographic:"Seinen",MediaType:"TV",Status:"RELEASING",ReleaseYear:2026,Season:"FALL"},"Synthetic Filter Alpha")
 check(store.CatalogFilter{MediaType:"MOVIE",Status:"FINISHED",ReleaseYear:2025,Season:"WINTER"},"Synthetic Filter Beta")
 check(store.CatalogFilter{Genre:literal},"Synthetic Filter Alpha")
 check(store.CatalogFilter{Genre:"Act"})
 check(store.CatalogFilter{Genre:"Missing"})
 check(store.CatalogFilter{Status:"UNKNOWN"},"Synthetic Filter Delta")
 check(store.CatalogFilter{Query:"Hidden Synthetic Alias",Theme:"Strategy"},"Synthetic Filter Alpha")
 check(store.CatalogFilter{Query:"Native Alias",MediaType:"TV"},"Synthetic Filter Alpha")
 check(store.CatalogFilter{Genre:"Action",MediaType:"OVA"})
 items,err:=repo.FilterCatalog(ctx,store.CatalogFilter{Query:"Synthetic Filter",Genre:"Action"},1,1)
 if err!=nil||len(items)!=1||items[0].Title!="Synthetic Filter Beta"{t.Fatal("filter-before-pagination",items,err)}
 if _,err=repo.FilterCatalog(ctx,store.CatalogFilter{Status:"bad"},24,0);!errors.Is(err,store.ErrCatalogFilter){t.Fatal("repository bypass accepted")}
 cancelled,cancel:=context.WithCancel(ctx);cancel()
 if _,err=repo.FilterCatalog(cancelled,store.CatalogFilter{},24,0);err==nil{t.Fatal("cancelled filter query succeeded")}
 h:=httpapi.NewPostgresRouter(slog.New(slog.NewTextHandler(io.Discard,nil)),db)
 query:=url.Values{"q":{"Hidden Synthetic Alias"},"genre":{"Action"},"theme":{"Strategy"},"studio":{"Synthetic Works"},"mediaType":{"TV"},"releaseYear":{"2026"}}
 w:=httptest.NewRecorder();h.ServeHTTP(w,httptest.NewRequest("GET","/api/v1/anime?"+query.Encode(),nil))
 var body struct{Items []store.Anime `json:"items"`;Count int `json:"count"`}
 if w.Code!=200||json.Unmarshal(w.Body.Bytes(),&body)!=nil||body.Count!=1||len(body.Items)!=1||body.Items[0].Title!="Synthetic Filter Alpha"||w.Header().Get("Cache-Control")!="no-store"{t.Fatal("anonymous filtered catalog",w.Code,w.Body.String())}
}
