package store_test
import("io";"log/slog";"net/http/httptest";"testing";"github.com/khordestan/anime-intelligence/internal/httpapi";"github.com/khordestan/anime-intelligence/internal/platform/store")
func TestCatalogRangeHTTPValidation(t *testing.T){
 s,err:=store.New("");if err!=nil{t.Fatal(err)}
 h:=httpapi.NewRouter(slog.New(slog.NewTextHandler(io.Discard,nil)),s)
 for query,want:=range map[string]int{
 "?minYear=2024&maxYear=2024&q=Solo":200,"?minYear=2000&maxYear=1990":400,
 "?minYear=1899":400,"?maxYear=10000":400,"?minYear=":400,"?minYear=2000&minYear=2001":400,
 "?minYear=2000.5":400,"?minEpisodes=-1":400,"?minEpisodes=24&maxEpisodes=12":400,"?sort=NEWEST":200,"?sort=unsupported":400,
 }{
 w:=httptest.NewRecorder();h.ServeHTTP(w,httptest.NewRequest("GET","/api/v1/anime"+query,nil));if w.Code!=want{t.Fatal(query,w.Code,want)}
 }
}
