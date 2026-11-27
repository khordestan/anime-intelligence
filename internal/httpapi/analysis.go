package httpapi

import (
	"context"
	"errors"

	"github.com/khordestan/anime-intelligence/internal/modules/catalog"
	"github.com/khordestan/anime-intelligence/internal/platform/postgres"
)

func boundedAnalysisSelector(slots chan struct{}, reserve func(context.Context) (postgres.AnalysisBudgetResult, error), selectFields catalog.CatalogFieldSelector) catalog.CatalogFieldSelector {
	return func(ctx context.Context, question string, allowed []string) ([]string, string, error) {
		metrics, _ := ctx.Value(requestMetricsContextKey{}).(*RequestMetrics)
		select {
		case slots <- struct{}{}:
			defer func() { <-slots }()
		default:
			metrics.recordAnalysis(analysisLocalLimit)
			return nil, "", catalog.ErrAnalysisCapacity
		}
		budget, err := reserve(ctx)
		if err != nil {
			metrics.recordAnalysis(analysisBudgetError)
			return nil, "", err
		}
		switch budget {
		case postgres.AnalysisBudgetGranted:
		case postgres.AnalysisOwnerLimit:
			metrics.recordAnalysis(analysisOwnerLimit)
			return nil, "", catalog.ErrAnalysisCapacity
		case postgres.AnalysisDeploymentLimit:
			metrics.recordAnalysis(analysisDeploymentLimit)
			return nil, "", catalog.ErrAnalysisCapacity
		default:
			metrics.recordAnalysis(analysisBudgetError)
			return nil, "", errors.New("unknown assisted analysis budget result")
		}
		fields, model, err := selectFields(ctx, question, allowed)
		if err != nil {
			metrics.recordAnalysis(analysisProviderError)
		} else {
			metrics.recordAnalysis(analysisProviderReturned)
		}
		return fields, model, err
	}
}
