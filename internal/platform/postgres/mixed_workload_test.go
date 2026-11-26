package postgres_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/khordestan/anime-intelligence/internal/httpapi"
	"github.com/khordestan/anime-intelligence/internal/platform/postgres"
	"github.com/khordestan/anime-intelligence/internal/platform/store"
)

func TestConcurrentDashboardWorkload(t *testing.T) {
	t.Run("smoke", func(t *testing.T) { runConcurrentDashboardWorkload(t, 8, 4, 1000) })
	if os.Getenv("ANIME_LOAD_EXTENDED") == "1" {
		t.Run("extended", func(t *testing.T) { runConcurrentDashboardWorkload(t, 24, 8, 10000) })
	}
}

func runConcurrentDashboardWorkload(t *testing.T, owners, rounds, titles int) {
	db, admin, _, runtimeDSN := testDB(t)
	const title = "10000000-0000-4000-8000-000000000001"
	type account struct {
		token string
		entry store.Entry
	}

	setup := context.Background()
	if _, err := admin.Exec(setup, `INSERT INTO anime(id,canonical_title,media_type)
		SELECT ('20000000-0000-4000-8000-'||lpad(n::text,12,'0'))::uuid,
		       'Synthetic workload title '||n, 'TV'
		FROM generate_series(1,$1) n`, titles); err != nil {
		t.Fatal(err)
	}
	accounts := make([]account, owners)
	for i := range accounts {
		session, err := db.Register(setup, fmt.Sprintf("mixed-%d@example.test", i), "synthetic-mixed-workload-password")
		if err != nil {
			t.Fatal(err)
		}
		verifyAccount(t, db, session.Token)
		entry, err := db.Repository(session.Token).Add(setup, title, "WATCHING", 0)
		if err != nil {
			t.Fatal(err)
		}
		accounts[i] = account{session.Token, entry}
	}

	replica, err := postgres.Open(setup, runtimeDSN)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(replica.Close)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	servers := []*httptest.Server{
		httptest.NewServer(httpapi.NewPostgresRouter(logger, db)),
		httptest.NewServer(httpapi.NewPostgresRouter(logger, replica)),
	}
	for _, server := range servers {
		t.Cleanup(server.Close)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()

	var sampleMu sync.Mutex
	samples := map[string][]time.Duration{}
	call := func(replicaIndex int, method, path, token, body string) (int, []byte, error) {
		started := time.Now()
		defer func() {
			elapsed := time.Since(started)
			route, _, _ := strings.Cut(path, "?")
			sampleMu.Lock()
			samples[method+" "+route] = append(samples[method+" "+route], elapsed)
			sampleMu.Unlock()
		}()
		server := servers[replicaIndex]
		req, err := http.NewRequestWithContext(ctx, method, server.URL+path, strings.NewReader(body))
		if err != nil {
			return 0, nil, err
		}
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		if body != "" {
			req.Header.Set("Content-Type", "application/json")
		}
		response, err := server.Client().Do(req)
		if err != nil {
			return 0, nil, err
		}
		defer response.Body.Close()
		data, err := io.ReadAll(response.Body)
		return response.StatusCode, data, err
	}
	start := make(chan struct{})
	var clients sync.WaitGroup
	started := time.Now()
	for _, owner := range accounts {
		clients.Go(func() {
			<-start
			for count := range rounds {
				body := fmt.Sprintf(`{"entries":[{"id":%q,"expectedCycle":1,"expectedEpisodesWatched":%d}]}`, owner.entry.ID, count)
				var writes sync.WaitGroup
				var statuses [2]int
				for i := range statuses {
					writes.Go(func() {
						status, _, err := call(i, "PATCH", "/api/v1/tracker/bulk-progress", owner.token, body)
						if err != nil {
							t.Errorf("progress request: %v", err)
						}
						statuses[i] = status
					})
				}
				status, data, err := call(0, "GET", "/api/v1/anime?q=Synthetic+workload+title&limit=12", "", "")
				var catalog struct{ Items []store.Anime }
				if err != nil || status != 200 || json.Unmarshal(data, &catalog) != nil || len(catalog.Items) != 12 {
					t.Errorf("catalog response status=%d items=%d error=%v", status, len(catalog.Items), err)
				}
				writes.Wait()
				if statuses != [2]int{http.StatusOK, http.StatusConflict} && statuses != [2]int{http.StatusConflict, http.StatusOK} {
					t.Errorf("expected one committed update and one conflict, got %v", statuses)
					return
				}
				status, data, err = call(1, "GET", "/api/v1/tracker", owner.token, "")
				var library struct{ Items []store.Entry }
				if err != nil || status != http.StatusOK || json.Unmarshal(data, &library) != nil || len(library.Items) != 1 || library.Items[0].ID != owner.entry.ID || library.Items[0].EpisodesWatched != count+1 {
					t.Errorf("private library check failed at episode %d: status=%d items=%d error=%v", count+1, status, len(library.Items), err)
					return
				}
			}
		})
	}
	close(start)
	clients.Wait()
	t.Logf("completed mixed workload in %v with %d owners, %d rounds and %d catalog titles",
		time.Since(started), owners, rounds, titles)
	routes := []string{"GET /api/v1/anime", "GET /api/v1/tracker", "PATCH /api/v1/tracker/bulk-progress"}
	for _, route := range routes {
		durations := samples[route]
		expected := owners * rounds
		if strings.HasPrefix(route, "PATCH ") {
			expected *= 2
		}
		if len(durations) != expected {
			t.Errorf("%s recorded %d requests, expected %d", route, len(durations), expected)
			continue
		}
		slices.Sort(durations)
		t.Logf("%s: requests=%d p50=%v p95=%v", route, len(durations),
			durations[(50*len(durations)+99)/100-1], durations[(95*len(durations)+99)/100-1])
	}
}
