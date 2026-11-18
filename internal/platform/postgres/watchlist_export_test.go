package postgres_test

import (
 "context"
 "encoding/json"
 "io"
 "log/slog"
 "net/http/httptest"
 "strings"
 "testing"

 "github.com/khordestan/anime-intelligence/internal/httpapi"
 "github.com/khordestan/anime-intelligence/internal/platform/postgres"
)

func TestWatchListExportPreservesProgressAndOwnerScope(t *testing.T) {
 db, _, _, _ := testDB(t)
 ctx := context.Background()
 one, err := db.Register(ctx, "watch-list-one@example.test", "synthetic-watch-list-password")
 if err != nil {
  t.Fatal(err)
 }
 two, err := db.Register(ctx, "watch-list-two@example.test", "synthetic-watch-list-password")
 if err != nil {
  t.Fatal(err)
 }
 router := httpapi.NewPostgresRouter(slog.New(slog.NewTextHandler(io.Discard, nil)), db)
 get := func(token string, status int) *httptest.ResponseRecorder {
  t.Helper()
  req := httptest.NewRequest("GET", "/api/v1/tracker/export", nil)
  if token != "" {
   req.Header.Set("Authorization", "Bearer "+token)
  }
  response := httptest.NewRecorder()
  router.ServeHTTP(response, req)
  if response.Code != status {
   t.Fatalf("status=%d want=%d body=%s", response.Code, status, response.Body.String())
  }
  return response
 }
 get("", 401)
 get(one.Token, 403)
 verifyAccount(t, db, one.Token)
 verifyAccount(t, db, two.Token)
 const firstID = "10000000-0000-4000-8000-000000000001"
 const secondID = "10000000-0000-4000-8000-000000000002"
 repo := db.Repository(one.Token)
 first, err := repo.Add(ctx, firstID, "WATCHING", 2)
 if err != nil {
  t.Fatal(err)
 }
 if _, _, err := repo.SetEpisodeProgress(ctx, first.ID, []int32{1,4}); err != nil {
  t.Fatal(err)
 }
 if _, _, err := repo.Rate(ctx, firstID, 5); err != nil {
  t.Fatal(err)
 }
 if _, err := repo.Add(ctx, secondID, "ON_HOLD", 3); err != nil {
  t.Fatal(err)
 }
 response := get(one.Token, 200)
 var data postgres.WatchListExport
 if err := json.Unmarshal(response.Body.Bytes(), &data); err != nil || data.Version != 1 || len(data.Entries) != 2 {
  t.Fatalf("unexpected export: %s err=%v", response.Body.String(), err)
 }
 exact, count := data.Entries[0], data.Entries[1]
 if exact.AnimeID != firstID || exact.WatchStatus != "WATCHING" || exact.EpisodesWatched != 2 ||
  exact.Rating == nil || *exact.Rating != 5 || len(exact.WatchedEpisodeNumbers) != 2 || exact.WatchedEpisodeNumbers[1] != 4 {
  t.Fatalf("exact progress mismatch: %+v", exact)
 }
 if count.AnimeID != secondID || count.WatchStatus != "ON_HOLD" || count.EpisodesWatched != 3 ||
  count.WatchedEpisodeNumbers != nil || count.Rating != nil {
  t.Fatalf("count progress mismatch: %+v", count)
 }
 if response.Header().Get("Cache-Control") != "no-store" ||
  !strings.Contains(response.Header().Get("Content-Disposition"), "anime-intelligence-watch-list.json") ||
  strings.Contains(response.Body.String(), one.User.Email) || strings.Contains(response.Body.String(), one.User.ID) {
  t.Fatal("watch list response exposed account metadata or incorrect headers")
 }
 blank := get(two.Token, 200)
 if err := json.Unmarshal(blank.Body.Bytes(), &data); err != nil || data.Entries == nil || len(data.Entries) != 0 {
  t.Fatalf("foreign watch list leaked: %s err=%v", blank.Body.String(), err)
 }
}
