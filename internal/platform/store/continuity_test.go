package store_test

import(
 "context"
 "encoding/json"
 "io"
 "log/slog"
 "net/http/httptest"
 "testing"

 "github.com/khordestan/anime-intelligence/internal/httpapi"
 "github.com/khordestan/anime-intelligence/internal/platform/store"
)
func TestLocalContinuityDoesNotInventLinks(t *testing.T){
 s,err:=store.New("");if err!=nil{t.Fatal(err)}
 h:=httpapi.NewRouter(slog.New(slog.NewTextHandler(io.Discard,nil)),s)
 w:=httptest.NewRecorder();h.ServeHTTP(w,httptest.NewRequest("GET","/api/v1/anime/10000000-0000-4000-8000-000000000001/continuity",nil))
 var graph store.Continuity
 if w.Code!=200||json.Unmarshal(w.Body.Bytes(),&graph)!=nil||len(graph.Nodes)!=1||len(graph.Edges)!=0||graph.Edges==nil||graph.Truncated||graph.Limit!=50||!graph.Nodes[0].EpisodeCountKnown{t.Fatal("local graph invented imported relationships",w.Code,graph)}
 graph,ok,err:=(store.Local{Store:s}).CatalogContinuity(context.Background(),"00000000-0000-4000-8000-000000000000")
 if err!=nil||ok||graph.Nodes==nil||graph.Edges==nil{t.Fatal("missing local title")}
}
