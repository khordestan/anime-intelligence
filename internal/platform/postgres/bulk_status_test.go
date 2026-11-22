package postgres_test

import (
 "bytes"
 "context"
 "encoding/json"
 "io"
 "log/slog"
 "net/http/httptest"
 "sort"
 "testing"

 "github.com/khordestan/anime-intelligence/internal/httpapi"
 "github.com/khordestan/anime-intelligence/internal/platform/store"
)

func TestBulkTrackerStatusRollbackAndIsolation(t *testing.T) {
 db, _, _, _ := testDB(t)
 ctx := context.Background()
 one, err := db.Register(ctx, "bulk-one@example.test", "synthetic-bulk-status-password")
 if err != nil {
  t.Fatal(err)
 }
 two, err := db.Register(ctx, "bulk-two@example.test", "synthetic-bulk-status-password")
 if err != nil {
  t.Fatal(err)
 }
 verifyAccount(t, db, one.Token)
 a := db.Repository(one.Token)
 first, err := a.Add(ctx, "10000000-0000-4000-8000-000000000001", "WATCHING", 2)
 if err != nil {
  t.Fatal(err)
 }
 second, err := a.Add(ctx, "10000000-0000-4000-8000-000000000002", "WATCHING", 0)
 if err != nil {
  t.Fatal(err)
 }
 if _, _, err := a.SetEpisodeProgress(ctx, first.ID, []int32{1, 4}); err != nil {
  t.Fatal(err)
 }
 router := httpapi.NewPostgresRouter(slog.New(slog.NewTextHandler(io.Discard, nil)), db)
 request := func(path, token string, fields map[string]any, want int) *httptest.ResponseRecorder {
  t.Helper()
  body, err := json.Marshal(fields)
  if err != nil {
   t.Fatal(err)
  }
  req := httptest.NewRequest("PATCH", path, bytes.NewReader(body))
  req.Header.Set("Authorization", "Bearer "+token)
  response := httptest.NewRecorder()
  router.ServeHTTP(response, req)
  if response.Code != want {
   t.Fatalf("status=%d want=%d body=%s", response.Code, want, response.Body.String())
  }
  return response
 }
 call := func(token string, targets []store.BulkStatusTarget, want int) *httptest.ResponseRecorder {
  t.Helper()
  return request("/api/v1/tracker/bulk-status", token, map[string]any{"watchStatus": "ON_HOLD", "entries": targets}, want)
 }
 targets := []store.BulkStatusTarget{{ID: first.ID, ExpectedCycle: 1}, {ID: second.ID, ExpectedCycle: 1}}
 sort.Slice(targets, func(i, j int) bool { return targets[i].ID < targets[j].ID })
 call(two.Token, targets, 403)
 verifyAccount(t, db, two.Token)
 call(two.Token, targets, 404)
 call(one.Token, []store.BulkStatusTarget{targets[0], targets[0]}, 400)
 stale := append([]store.BulkStatusTarget(nil), targets...)
 stale[1].ExpectedCycle = 2
 call(one.Token, stale, 409)
 rows, err := a.Library(ctx)
 if err != nil || len(rows) != 2 {
  t.Fatalf("library=%v err=%v", rows, err)
 }
 for _, entry := range rows {
  if entry.WatchStatus != "WATCHING" {
   t.Fatalf("failed batch persisted change to %s", entry.ID)
  }
 }
 response := call(one.Token, targets, 200)
 var body struct{ Items []store.Entry }
 if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil || len(body.Items) != 2 {
  t.Fatalf("response=%s err=%v", response.Body.String(), err)
 }
 for i, entry := range body.Items {
  if entry.ID != targets[i].ID || entry.WatchStatus != "ON_HOLD" || (entry.ID == first.ID && (!entry.ExactProgress || entry.EpisodesWatched != 2)) {
   t.Fatalf("unexpected updated entry: %+v", entry)
  }
 }

 rate := func(token string, targets []store.BulkStatusTarget, score, want int) *httptest.ResponseRecorder {
  t.Helper()
  return request("/api/v1/tracker/bulk-rating", token, map[string]any{"score": score, "entries": targets}, want)
 }
 rate(two.Token, targets, 5, 404)
 rate(one.Token, targets, 6, 400)
 rate(one.Token, stale, 4, 409)
 rows, err = a.Library(ctx)
 if err != nil || len(rows) != 2 || rows[0].Rating != 0 || rows[1].Rating != 0 {
  t.Fatalf("stale rating batch persisted: %v err=%v", rows, err)
 }
 response = rate(one.Token, targets, 4, 200)
 if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil || len(body.Items) != 2 {
  t.Fatalf("response=%s err=%v", response.Body.String(), err)
 }
 for i, entry := range body.Items {
  if entry.ID != targets[i].ID || entry.WatchStatus != "ON_HOLD" || entry.Rating != 4 ||
    (entry.ID == first.ID && (!entry.ExactProgress || entry.EpisodesWatched != 2)) {
   t.Fatalf("unexpected rated entry: %+v", entry)
  }
 }
 rows, err = a.Library(ctx)
 if err != nil || len(rows) != 2 || rows[0].Rating != 4 || rows[1].Rating != 4 {
  t.Fatalf("ratings did not persist: %v err=%v", rows, err)
 }
}
