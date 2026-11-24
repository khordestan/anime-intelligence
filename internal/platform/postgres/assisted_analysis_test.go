package postgres_test

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/khordestan/anime-intelligence/internal/httpapi"
)

func TestAssistedAnalysisRequiresVerifiedSession(t *testing.T) {
	db, _, _, _ := testDB(t)
	ctx := context.Background()
	account, err := db.Register(ctx, "analyst@example.test", "synthetic-analyst-password")
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	h := httpapi.NewPostgresRouterWithAnalyst(slog.New(slog.NewTextHandler(io.Discard, nil)), db, nil, nil,
		func(_ context.Context, _ string, _ []string) ([]string, string, error) {
			calls++
			return []string{"episodeCount"}, "synthetic-model", nil
		})
	request := func(token string, status int) {
		t.Helper()
		body := `{"question":"How long?","animeIds":["10000000-0000-4000-8000-000000000001"]}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/analysis/catalog/assisted", strings.NewReader(body))
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		res := httptest.NewRecorder()
		h.ServeHTTP(res, req)
		if res.Code != status || (status == 200 && !strings.Contains(res.Body.String(), `"model":"synthetic-model"`)) {
			t.Fatalf("status=%d body=%s", res.Code, res.Body)
		}
	}
	request("", 401)
	request(account.Token, 403)
	verifyAccount(t, db, account.Token)
	for range 3 {
		request(account.Token, 200)
	}
	request(account.Token, 429)
	if calls != 3 {
		t.Fatalf("provider calls = %d, want 3", calls)
	}
}
