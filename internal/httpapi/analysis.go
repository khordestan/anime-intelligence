package httpapi

import (
	"context"
	"sync"
	"time"

	"github.com/khordestan/anime-intelligence/internal/modules/catalog"
)

type analysisBudget struct {
	mu       sync.Mutex
	until    time.Time
	perOwner map[string]int
	total    int
	active   int
}

func (b *analysisBudget) acquire(owner string, now time.Time) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	if !now.Before(b.until) {
		b.until = now.Add(time.Minute)
		b.perOwner = make(map[string]int)
		b.total = 0
	}
	if b.active >= 2 || b.total >= 12 || b.perOwner[owner] >= 3 {
		return false
	}
	b.active++
	b.total++
	b.perOwner[owner]++
	return true
}

func (b *analysisBudget) release() {
	b.mu.Lock()
	b.active--
	b.mu.Unlock()
}

func (b *analysisBudget) forOwner(owner string, selectFields catalog.CatalogFieldSelector) catalog.CatalogFieldSelector {
	return func(ctx context.Context, question string, allowed []string) ([]string, string, error) {
		if !b.acquire(owner, time.Now()) {
			return nil, "", catalog.ErrAnalysisCapacity
		}
		defer b.release()
		return selectFields(ctx, question, allowed)
	}
}
