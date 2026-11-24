package httpapi

import (
	"context"
	"errors"
	"testing"

	"github.com/khordestan/anime-intelligence/internal/modules/catalog"
)

func TestAnalysisBudgetDoesNotReserveWhenLocalSlotsAreFull(t *testing.T) {
	slots := make(chan struct{}, 1)
	slots <- struct{}{}
	reserved := 0
	called := 0
	selectFields := boundedAnalysisSelector(slots, func(context.Context) (bool, error) {
		reserved++
		return true, nil
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
