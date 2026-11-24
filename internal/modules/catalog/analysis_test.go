package catalog

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/khordestan/anime-intelligence/internal/platform/store"
)

type analysisCatalog struct {
	store.Repository
}

func (analysisCatalog) CatalogDetails(_ context.Context, id string) (store.CatalogDetails, bool, error) {
	return store.CatalogDetails{
		Anime:        store.Anime{ID: id, Title: "Synthetic title", EpisodeCountKnown: true, Synopsis: "Hidden story"},
		Provider:     "base",
		FieldSources: []store.CatalogFieldSource{{Field: "episodeCount", Mode: "SELECT", Provider: "reviewed"}},
		Themes:       []string{"Hidden theme"},
	}, true, nil
}

func TestAnalyzeReturnsOnlySelectedCatalogEvidence(t *testing.T) {
	handler := NewHandler(analysisCatalog{})
	const id = "10000000-0000-4000-8000-000000000001"
	request := httptest.NewRequest(http.MethodPost, "/api/v1/analysis/catalog",
		strings.NewReader(`{"question":"How many episodes?","animeIds":["`+id+`"]}`))
	response := httptest.NewRecorder()
	handler.Analyze(response, request)
	var payload struct {
		Engine       string
		TopicMatched bool
		Items        []catalogAnalysisTitle
	}
	if response.Code != 200 || json.Unmarshal(response.Body.Bytes(), &payload) != nil ||
		payload.Engine != "catalog-facts-v1" || !payload.TopicMatched || len(payload.Items) != 1 {
		t.Fatalf("unexpected response: %d %s", response.Code, response.Body)
	}
	facts := payload.Items[0].Evidence
	if len(facts) != 1 || facts[0].Field != "episodeCount" || len(facts[0].Values) != 1 ||
		facts[0].Values[0] != "0" || facts[0].Provider != "reviewed" {
		t.Fatalf("unexpected evidence: %+v", facts)
	}
	if strings.Contains(response.Body.String(), "Hidden") || strings.Contains(response.Body.String(), "How many") {
		t.Fatalf("returned unrelated data: %s", response.Body)
	}
}

func TestAnalyzeRejectsUnboundedAndAmbiguousRequests(t *testing.T) {
	handler := NewHandler(analysisCatalog{})
	const id = "10000000-0000-4000-8000-000000000001"
	for _, body := range []string{
		`{"question":"Year?","animeIds":["` + id + `","` + id + `"]}`,
		`{"question":"Year?","animeIds":["bad"]}`,
		`{"question":"Year?","animeIds":["` + id + `"],"sql":"select 1"}`,
		`{"question":"` + strings.Repeat("x", 4096) + `","animeIds":["` + id + `"]}`,
	} {
		response := httptest.NewRecorder()
		handler.Analyze(response, httptest.NewRequest(http.MethodPost, "/api/v1/analysis/catalog", strings.NewReader(body)))
		if response.Code != 400 {
			t.Fatalf("accepted invalid request: %d %s", response.Code, response.Body)
		}
	}
}
