package store_test

import (
 "context"
 "errors"
 "io"
 "log/slog"
 "net/http/httptest"
 "strings"
 "testing"

 "github.com/khordestan/anime-intelligence/internal/httpapi"
 "github.com/khordestan/anime-intelligence/internal/platform/store"
)
func TestCatalogFilterValidationAndLocalUnknownMetadata(t *testing.T){
 f,err:=(store.CatalogFilter{Query:"  synthetic  ",Genre:"  Synthetic genre "}).Normalize()
 if err!=nil||f.Query!="synthetic"||f.Genre!="Synthetic genre"{t.Fatal(f,err)}
 for _,f:=range []store.CatalogFilter{{MediaType:"tv"},{Status:"WATCHING"},{Season:"AUTUMN"},{ReleaseYear:1899},{ReleaseYear:10000},{Query:strings.Repeat("q",2049)},{Genre:strings.Repeat("g",257)},{Theme:string([]byte{255})},{Query:"bad\x00query"},{Studio:"bad\x00studio"}}{
  if _,err:=f.Normalize();!errors.Is(err,store.ErrCatalogFilter){t.Fatal("invalid filter accepted",f)}
 }
 s,err:=store.New("");if err!=nil{t.Fatal(err)}
 local:=store.Local{Store:s}
 for _,f:=range []store.CatalogFilter{{Genre:"Action"},{Theme:"Strategy"},{Studio:"Synthetic"},{Season:"WINTER"}}{
  items,err:=local.FilterCatalog(context.Background(),f,24,0)
  if err!=nil||items==nil||len(items)!=0{t.Fatal("local metadata inferred",f,items,err)}
 }
 items,err:=local.FilterCatalog(context.Background(),store.CatalogFilter{Query:"solo",MediaType:"TV",Status:"FINISHED",ReleaseYear:2024},24,0)
 if err!=nil||len(items)!=1||items[0].Title!="Solo Leveling"{t.Fatal(items,err)}
 for _,bounds:=range [][2]int{{0,0},{101,0},{1,-1},{1,100001}}{
  if _,err:=local.FilterCatalog(context.Background(),store.CatalogFilter{},bounds[0],bounds[1]);!errors.Is(err,store.ErrCatalogFilter){t.Fatal("unbounded request accepted")}
 }
 h:=httpapi.NewRouter(slog.New(slog.NewTextHandler(io.Discard,nil)),s)
 for query,want:=range map[string]int{
  "?mediaType=tv":400,"?mediaType=TV&mediaType=MOVIE":400,"?releaseYear=0":400,"?releaseYear=no":400,"?genre=":400,"?theme=%20":400,"?q=x&q=y":400,"?genre=%FF":400,"?genre=%ZZ":400,
  "?q=Solo&mediaType=TV&releaseYear=2024":200,"?genre=Action":200,
 }{
  w:=httptest.NewRecorder();h.ServeHTTP(w,httptest.NewRequest("GET","/api/v1/anime"+query,nil))
  if w.Code!=want||w.Header().Get("Cache-Control")!="no-store"{t.Fatal("filter HTTP validation",query,w.Code)}
 }
}
