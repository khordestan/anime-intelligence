package postgres_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/khordestan/anime-intelligence/internal/httpapi"
	"github.com/khordestan/anime-intelligence/internal/platform/store"
)

func TestConcurrentDashboardWorkload(t *testing.T) {
	db, admin, _, _ := testDB(t)
	const title = "10000000-0000-4000-8000-000000000001"
	const owners, rounds = 8, 4
	type account struct {
		token string
		entry store.Entry
	}

	setup := context.Background()
	if _, err := admin.Exec(setup, `INSERT INTO anime(id,canonical_title,media_type)
		SELECT ('20000000-0000-4000-8000-'||lpad(n::text,12,'0'))::uuid,
		       'Synthetic workload title '||n, 'TV'
		FROM generate_series(1,1000) n`); err != nil {
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

	srv := httptest.NewServer(httpapi.NewPostgresRouter(slog.New(slog.NewTextHandler(io.Discard, nil)), db))
	t.Cleanup(srv.Close)
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()

	call := func(method, path, token, body string) (int, []byte, error) {
		req, err := http.NewRequestWithContext(ctx, method, srv.URL+path, strings.NewReader(body))
		if err != nil {
			return 0, nil, err
		}
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		if body != "" {
			req.Header.Set("Content-Type", "application/json")
		}
		response, err := srv.Client().Do(req)
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
						status, _, err := call("PATCH", "/api/v1/tracker/bulk-progress", owner.token, body)
						if err != nil {
							t.Errorf("progress request: %v", err)
						}
						statuses[i] = status
					})
				}
				status, data, err := call("GET", "/api/v1/anime?q=Synthetic+workload+title&limit=12", "", "")
				var catalog struct{ Items []store.Anime }
				if err != nil || status != 200 || json.Unmarshal(data, &catalog) != nil || len(catalog.Items) != 12 {
					t.Errorf("catalog response status=%d items=%d error=%v", status, len(catalog.Items), err)
				}
				writes.Wait()
				if statuses != [2]int{http.StatusOK, http.StatusConflict} && statuses != [2]int{http.StatusConflict, http.StatusOK} {
					t.Errorf("expected one committed update and one conflict, got %v", statuses)
					return
				}
				status, data, err = call("GET", "/api/v1/tracker", owner.token, "")
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
	t.Logf("completed mixed workload in %v with %d owners and %d rounds", time.Since(started), owners, rounds)
}
