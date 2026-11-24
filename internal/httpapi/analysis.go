package httpapi

import (
	"context"

	"github.com/khordestan/anime-intelligence/internal/modules/catalog"
)

func boundedAnalysisSelector(slots chan struct{}, reserve func(context.Context) (bool, error), selectFields catalog.CatalogFieldSelector) catalog.CatalogFieldSelector {
	return func(ctx context.Context, question string, allowed []string) ([]string, string, error) {
		select {
		case slots <- struct{}{}:
			defer func() { <-slots }()
		default:
			return nil, "", catalog.ErrAnalysisCapacity
		}
		ok, err := reserve(ctx)
		if err != nil {
			return nil, "", err
		}
		if !ok {
			return nil, "", catalog.ErrAnalysisCapacity
		}
		return selectFields(ctx, question, allowed)
	}
}
