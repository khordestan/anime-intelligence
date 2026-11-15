package httpapi

import (
 "io"
 "log/slog"
 "net/http"
 "net/http/httptest"
 "strings"
 "testing"
 "github.com/example/anime-intelligence/internal/platform/store"
)
func TestTrackerVerticalSlice(t *testing.T) {
 data,err:=store.New("");if err!=nil{t.Fatal(err)}
 handler:=NewRouter(slog.New(slog.NewTextHandler(io.Discard,nil)),data)
 req:=httptest.NewRequest(http.MethodPost,"/api/v1/tracker",strings.NewReader(`{"animeId":"10000000-0000-4000-8000-000000000001","watchStatus":"watching","episodesWatched":2}`))
 req.Header.Set("Content-Type","application/json");created:=httptest.NewRecorder();handler.ServeHTTP(created,req)
 if created.Code!=http.StatusCreated{t.Fatalf("create status=%d body=%s",created.Code,created.Body.String())}
 list:=httptest.NewRecorder();handler.ServeHTTP(list,httptest.NewRequest(http.MethodGet,"/api/v1/tracker",nil))
 if list.Code!=http.StatusOK||!strings.Contains(list.Body.String(),"Solo Leveling")||!strings.Contains(list.Body.String(),"WATCHING"){t.Fatalf("unexpected list response: %d %s",list.Code,list.Body.String())}
}
func TestTrackerRejectsInvalidStatus(t *testing.T) {
 data,_:=store.New("");handler:=NewRouter(slog.New(slog.NewTextHandler(io.Discard,nil)),data)
 req:=httptest.NewRequest(http.MethodPost,"/api/v1/tracker",strings.NewReader(`{"animeId":"10000000-0000-4000-8000-000000000001","watchStatus":"INVALID","episodesWatched":0}`))
 req.Header.Set("Content-Type","application/json");res:=httptest.NewRecorder();handler.ServeHTTP(res,req)
 if res.Code!=http.StatusBadRequest{t.Fatalf("want 400, got %d: %s",res.Code,res.Body.String())}
}
