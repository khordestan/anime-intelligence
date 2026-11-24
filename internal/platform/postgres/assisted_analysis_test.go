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
	db, owner, _, _ := testDB(t)
	ctx := context.Background()
	account, err := db.Register(ctx, "analyst@example.test", "synthetic-analyst-password")
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	selector := func(_ context.Context, _ string, _ []string) ([]string, string, error) {
		calls++
		return []string{"episodeCount"}, "synthetic-model", nil
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	first := httpapi.NewPostgresRouterWithAnalyst(logger, db, nil, nil, selector)
	second := httpapi.NewPostgresRouterWithAnalyst(logger, db, nil, nil, selector)
	request := func(handler http.Handler, token string, status int) {
		t.Helper()
		body := `{"question":"How long?","animeIds":["10000000-0000-4000-8000-000000000001"]}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/analysis/catalog/assisted", strings.NewReader(body))
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		res := httptest.NewRecorder()
		handler.ServeHTTP(res, req)
		if res.Code != status || (status == 200 && !strings.Contains(res.Body.String(), `"model":"synthetic-model"`)) {
			t.Fatalf("status=%d body=%s", res.Code, res.Body)
		}
	}
	request(first, "", 401)
	request(first, account.Token, 403)
	verifyAccount(t, db, account.Token)
	request(first, account.Token, 200)
	request(second, account.Token, 200)
	request(first, account.Token, 200)
	request(second, account.Token, 429)

	var global int
	if err := owner.QueryRow(ctx, `SELECT requests FROM catalog_analysis_global_budget`).Scan(&global); err != nil || global != 3 {
		t.Fatalf("denied owner consumed global usage: %d %v", global, err)
	}
	other, err := db.Register(ctx, "analyst-other@example.test", "synthetic-analyst-password")
	if err != nil {
		t.Fatal(err)
	}
	verifyAccount(t, db, other.Token)
	exported, err := db.ExportAccount(ctx, other.Token)
	if err != nil || string(exported.Data["analysisUsage"]) != "[]" {
		t.Fatalf("foreign budget was visible: %s %v", exported.Data["analysisUsage"], err)
	}
	if _, err := owner.Exec(ctx, `UPDATE catalog_analysis_global_budget
		SET window_start=date_trunc('minute', now()),requests=11`); err != nil {
		t.Fatal(err)
	}
	request(second, other.Token, 200)
	request(first, other.Token, 429)
	if calls != 4 {
		t.Fatalf("provider calls = %d, want 4", calls)
	}
	exported, err = db.ExportAccount(ctx, other.Token)
	if err != nil || !strings.Contains(string(exported.Data["analysisUsage"]), other.User.ID) {
		t.Fatalf("own usage missing from export: %v", err)
	}
	if err := db.DeleteAccount(ctx, other.Token, "synthetic-analyst-password"); err != nil {
		t.Fatal(err)
	}
	var rows int
	if err := owner.QueryRow(ctx, `SELECT count(*) FROM catalog_analysis_owner_budget WHERE user_id=$1`, other.User.ID).Scan(&rows); err != nil || rows != 0 {
		t.Fatalf("usage leaked after account deletion: %d %v", rows, err)
	}
}
