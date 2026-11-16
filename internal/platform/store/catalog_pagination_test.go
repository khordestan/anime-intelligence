package store_test
import("encoding/json";"io";"log/slog";"net/http/httptest";"testing";"github.com/khordestan/anime-intelligence/internal/httpapi";"github.com/khordestan/anime-intelligence/internal/platform/store")
func TestCatalogPageNavigationHints(t *testing.T){
 s,err:=store.New("");if err!=nil{t.Fatal(err)}
 h:=httpapi.NewRouter(slog.New(slog.NewTextHandler(io.Discard,nil)),s)
 for _,spec:=range []struct{query string;next,previous *int}{
 {"?limit=1",intPointer(1),nil},
 {"?limit=1&offset=1",intPointer(2),intPointer(0)},
 {"?limit=3&offset=1",intPointer(4),intPointer(0)},
 {"?limit=100",nil,nil},
 {"?genre=Absent&limit=1",nil,nil},
 {"?limit=1&offset=100000",nil,intPointer(99999)},
 }{
  w:=httptest.NewRecorder();h.ServeHTTP(w,httptest.NewRequest("GET","/api/v1/anime"+spec.query,nil))
  var out struct{Next *int `json:"nextOffset"`;Previous *int `json:"previousOffset"`}
  if w.Code!=200||json.Unmarshal(w.Body.Bytes(),&out)!=nil{t.Fatal("page response",w.Code)}
  equal:=func(a,b *int)bool{return a==nil&&b==nil||a!=nil&&b!=nil&&*a==*b}
  if !equal(out.Next,spec.next)||!equal(out.Previous,spec.previous){t.Fatal("navigation bounds",spec.query,out)}
 }
}
func intPointer(v int)*int{return &v}
