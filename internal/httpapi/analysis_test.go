package httpapi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/khordestan/anime-intelligence/internal/modules/catalog"
	"github.com/khordestan/anime-intelligence/internal/platform/postgres"
)

func TestAnalysisBudgetDoesNotReserveWhenLocalSlotsAreFull(t *testing.T) {
	slots := make(chan struct{}, 1)
	slots <- struct{}{}
	reserved := 0
	called := 0
	selectFields := boundedAnalysisSelector(slots, func(context.Context) (postgres.AnalysisBudgetResult, error) {
		reserved++
		return postgres.AnalysisBudgetGranted, nil
	}, func(context.Context, string, []string) ([]string, string, error) {
		called++
		return []string{"episodeCount"}, "test", nil
	})
	if _, _, err := selectFields(context.Background(), "How long?", nil); !errors.Is(err, catalog.ErrAnalysisCapacity) || reserved != 0 {
		t.Fatalf("full process spent a shared allowance: %v, %d", err, reserved)
	}
	<-slots
	if _, _, err := selectFields(context.Background(), "How long?", nil); err != nil || reserved != 1 || called != 1 || len(slots) != 0 {
		t.Fatalf("selector failed: %v, reserves=%d calls=%d slots=%d", err, reserved, called, len(slots))
	}
}

func TestAnalysisStageMetricsSeparateQuotasAndErrors(t *testing.T) {
	metrics := &RequestMetrics{}
	failure := errors.New("synthetic sensitive details")
	for _, fixture := range []struct {
		localFull   bool
		budget      postgres.AnalysisBudgetResult
		budgetErr   error
		providerErr error
	}{
		{localFull: true},
		{budget: postgres.AnalysisOwnerLimit},
		{budget: postgres.AnalysisDeploymentLimit},
		{budgetErr: failure},
		{budget: postgres.AnalysisBudgetGranted, providerErr: failure},
		{budget: postgres.AnalysisBudgetGranted},
	} {
		slots := make(chan struct{}, 1)
		if fixture.localFull {
			slots <- struct{}{}
		}
		selected := boundedAnalysisSelector(slots, func(context.Context) (postgres.AnalysisBudgetResult, error) {
			return fixture.budget, fixture.budgetErr
		}, func(context.Context, string, []string) ([]string, string, error) {
			return []string{"episodeCount"}, "synthetic-model", fixture.providerErr
		})
		handler := metrics.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _, _ = selected(r.Context(), "synthetic private question", []string{"episodeCount"})
			w.WriteHeader(http.StatusNoContent)
		}))
		handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("POST", "/api/v1/analysis/catalog/assisted", nil))
	}
	scrape := httptest.NewRecorder()
	metrics.Handler().ServeHTTP(scrape, httptest.NewRequest("GET", "/metrics", nil))
	for _, stage := range []string{"local_limit", "owner_budget_limit", "deployment_budget_limit", "budget_error", "provider_error", "provider_returned"} {
		if !strings.Contains(scrape.Body.String(), `anime_assisted_analysis_outcomes_total{outcome="`+stage+`"} 1`+"\n") {
			t.Errorf("missing or incorrect analysis counter %q", stage)
		}
	}
	if strings.Contains(scrape.Body.String(), "synthetic") {
		t.Fatal("sensitive question, model, or error content in metrics")
	}
}
