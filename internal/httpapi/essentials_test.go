package httpapi

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/khordestan/anime-intelligence/internal/modules/intelligence"
)

func TestGlobalEssentialsPublicRoute(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	const path = "/api/v1/recommendations/essentials"
	request := httptest.NewRequest(http.MethodGet, path, nil)
	unavailable := httptest.NewRecorder()
	NewPostgresRouter(logger, nil).ServeHTTP(unavailable, request)
	if unavailable.Code != http.StatusNotFound {
		t.Fatalf("disabled endpoint status = %d", unavailable.Code)
	}

	response := httptest.NewRecorder()
	handler := NewPostgresRouterWithEssentials(logger, nil, nil, []intelligence.EssentialsMatch{
		{AnimeID: "anime", Title: "Canonical title", EvidenceKinds: 1, PublisherCount: 1},
	})
	handler.ServeHTTP(response, request)
	var page struct {
		AlgorithmVersion string
		Items            []intelligence.EssentialsMatch
	}
	if response.Code != http.StatusOK || json.Unmarshal(response.Body.Bytes(), &page) != nil ||
		page.AlgorithmVersion != intelligence.GlobalEssentialsAlgorithmVersion ||
		len(page.Items) != 1 || page.Items[0].AnimeID != "anime" {
		t.Fatalf("unexpected response: %d %s", response.Code, response.Body)
	}
}
