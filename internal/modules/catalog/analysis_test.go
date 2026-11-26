package catalog

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/khordestan/anime-intelligence/internal/platform/store"
)

type analysisCatalog struct {
	store.Repository
}

func (analysisCatalog) CatalogDetails(_ context.Context, id string) (store.CatalogDetails, bool, error) {
	return store.CatalogDetails{
		Anime:        store.Anime{ID: id, Title: "Synthetic title", NativeTitle: "Synthetic native", Season: "SPRING", EpisodeCountKnown: true, Synopsis: "Hidden story"},
		Provider:     "base",
		FieldSources: []store.CatalogFieldSource{{Field: "episodeCount", Mode: "SELECT", Provider: "reviewed"}, {Field: "season", Mode: "SELECT", Provider: "reviewed-season"}},
		Demographics: []string{"Synthetic demographic"},
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

func TestAnalyzeAssistedRejectsUnknownKeys(t *testing.T) {
	handler := NewHandler(analysisCatalog{})
	for _, tc := range []struct {
		fields []string
		status int
	}{
		{[]string{"episodeCount"}, 200},
		{[]string{"continuity"}, 200},
		{[]string{"nativeTitle", "season", "demographics"}, 200},
		{[]string{"episodeCount", "episodeCount"}, 502},
		{[]string{"synopsis"}, 502},
	} {
		response := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/api/v1/analysis/catalog/assisted",
			strings.NewReader(`{"question":"How long?","animeIds":["10000000-0000-4000-8000-000000000001"]}`))
		handler.AnalyzeWithSelector(response, request, func(_ context.Context, question string, allowed []string) ([]string, string, error) {
			if question != "How long?" || len(allowed) != 12 {
				t.Fatalf("unexpected provider input: %q %v", question, allowed)
			}
			return tc.fields, "synthetic-1", nil
		})
		if response.Code != tc.status || strings.Contains(response.Body.String(), "Hidden") {
			t.Fatalf("unexpected response: %d %s", response.Code, response.Body)
		}
	}
}

func TestAnalysisComparisonsRequireKnownValues(t *testing.T) {
	titles := []catalogAnalysisTitle{
		{AnimeID: "a", Evidence: []catalogAnalysisFact{
			{Field: "releaseYear", Values: []string{}},
			{Field: "episodeCount", Values: []string{"0"}},
		}},
		{AnimeID: "b", Evidence: []catalogAnalysisFact{
			{Field: "releaseYear", Values: []string{"2024"}},
			{Field: "episodeCount", Values: []string{"12"}},
		}},
	}
	got := compareAnalysisFacts(titles)
	if len(got) != 1 || got[0].Field != "episodeCount" || got[0].Difference != -12 ||
		len(got[0].AnimeIDs) != 2 || got[0].AnimeIDs[0] != "a" || got[0].AnimeIDs[1] != "b" {
		t.Fatalf("comparisons: %+v", got)
	}
}

func TestAnalysisSharedLabelsRequireEvidenceForBothTitles(t *testing.T) {
	items := []catalogAnalysisTitle{
		{Evidence: []catalogAnalysisFact{
			{Field: "genres", Values: []string{"Adventure", "Adventure", "Drama"}, Truncated: true},
			{Field: "studios", Values: []string{"Synthetic studio"}},
			{Field: "status", Values: []string{"FINISHED"}},
		}},
		{Evidence: []catalogAnalysisFact{
			{Field: "genres", Values: []string{"adventure", "Comedy"}},
			{Field: "studios", Values: []string{}},
			{Field: "status", Values: []string{"FINISHED"}},
		}},
	}
	got := sharedAnalysisEvidence(items)
	if len(got) != 1 || got[0].Field != "genres" || len(got[0].Values) != 1 || got[0].Values[0] != "Adventure" {
		t.Fatalf("shared labels: %+v", got)
	}
	if got := sharedAnalysisEvidence(items[:1]); len(got) != 0 {
		t.Fatalf("a single title cannot share labels: %+v", got)
	}
}

func TestAnalyzeAdditionalTopicsPreserveFieldProvenance(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/api/v1/analysis/catalog",
		strings.NewReader(`{"question":"What native title, release quarter and demographic are recorded?","animeIds":["10000000-0000-4000-8000-000000000001"]}`))
	response := httptest.NewRecorder()
	NewHandler(analysisCatalog{}).Analyze(response, request)
	var payload struct {
		TopicMatched bool
		Items        []catalogAnalysisTitle
	}
	if response.Code != 200 || json.Unmarshal(response.Body.Bytes(), &payload) != nil || !payload.TopicMatched || len(payload.Items) != 1 {
		t.Fatalf("response: %d %s", response.Code, response.Body)
	}
	facts := map[string]catalogAnalysisFact{}
	for _, fact := range payload.Items[0].Evidence {
		facts[fact.Field] = fact
	}
	for key, expected := range map[string][2]string{
		"nativeTitle":  {"Synthetic native", "base"},
		"season":       {"SPRING", "reviewed-season"},
		"demographics": {"Synthetic demographic", "base"},
	} {
		got := facts[key]
		if len(got.Values) != 1 || got.Values[0] != expected[0] || got.Provider != expected[1] {
			t.Errorf("%s: unexpected evidence %+v", key, got)
		}
	}
	if strings.Contains(response.Body.String(), "Hidden") {
		t.Fatal("additional topics exposed hidden evidence")
	}
}

func TestAnalysisSeparatesConfirmedReleasesFromUpcomingSchedules(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	past := now.Add(-time.Hour)
	future := now.Add(time.Hour)
	details := store.CatalogDetails{Episodes: []store.CatalogEpisode{
		{Number: 1, Track: "SUB", ReleasedAt: &past, Provider: "synthetic-release"},
		{Number: 2, Track: "SUB", ScheduledAt: &past, Provider: "synthetic-expired"},
		{Number: 2, Track: "DUB", ScheduledAt: &future, Provider: "synthetic-schedule"},
		{Number: 3, Track: "SUB", ReleasedAt: &future, Provider: "synthetic-invalid"},
	}}
	released := projectAnalysisFact(details, "confirmedReleases", now)
	if len(released.EpisodeFacts) != 1 || released.EpisodeFacts[0].Provider != "synthetic-release" || !released.EpisodeFacts[0].Time.Equal(past) {
		t.Fatalf("release evidence: %+v", released)
	}
	scheduled := projectAnalysisFact(details, "upcomingSchedule", now)
	if len(scheduled.EpisodeFacts) != 1 || scheduled.EpisodeFacts[0].Track != "DUB" || !scheduled.EpisodeFacts[0].Time.Equal(future) {
		t.Fatalf("schedule evidence: %+v", scheduled)
	}
	for i := 1; i <= 22; i++ {
		at := now.Add(time.Duration(i) * time.Minute)
		details.Episodes = append(details.Episodes, store.CatalogEpisode{Number: i + 3, Track: "SUB", ScheduledAt: &at})
	}
	scheduled = projectAnalysisFact(details, "upcomingSchedule", now)
	if !scheduled.Truncated || len(scheduled.EpisodeFacts) != 20 || scheduled.EpisodeFacts[0].Number != 4 || scheduled.EpisodeFacts[19].Number != 23 {
		t.Fatalf("schedule evidence was not bounded in timestamp order: %+v", scheduled)
	}
}

type continuityAnalysisCatalog struct{ analysisCatalog }

func (continuityAnalysisCatalog) CatalogContinuity(_ context.Context, root string) (store.Continuity, bool, error) {
	return store.Continuity{RootID: root, Truncated: true,
		Nodes: []store.ContinuityNode{{ID: "prior", Title: "Earlier title"}, {ID: "later", Title: "Later title"}, {ID: "indirect", Title: "Indirect title"}},
		Edges: []store.ContinuityEdge{
			{FromID: "prior", ToID: root, Providers: []string{"synthetic-review"}},
			{FromID: root, ToID: "later"},
			{FromID: "later", ToID: "indirect"},
		},
	}, true, nil
}

func TestAnalyzeContinuityReturnsDirectAttributedEdges(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/api/v1/analysis/catalog",
		strings.NewReader(`{"question":"Which prequels and sequels are recorded?","animeIds":["10000000-0000-4000-8000-000000000001"]}`))
	response := httptest.NewRecorder()
	NewHandler(continuityAnalysisCatalog{}).Analyze(response, request)
	var payload struct {
		TopicMatched bool
		Items        []catalogAnalysisTitle
	}
	if response.Code != 200 || json.Unmarshal(response.Body.Bytes(), &payload) != nil || !payload.TopicMatched || len(payload.Items) != 1 {
		t.Fatalf("response: %d %s", response.Code, response.Body)
	}
	facts := payload.Items[0].Evidence
	if len(facts) != 1 || facts[0].Field != "continuity" || !facts[0].Truncated || len(facts[0].ContinuityFacts) != 2 {
		t.Fatalf("facts: %+v", facts)
	}
	prior, later := facts[0].ContinuityFacts[0], facts[0].ContinuityFacts[1]
	if prior.AnimeID != "prior" || prior.Title != "Earlier title" || prior.Relation != "PREQUEL" ||
		len(prior.Providers) != 1 || prior.Providers[0] != "synthetic-review" ||
		later.AnimeID != "later" || later.Relation != "SEQUEL" || len(later.Providers) != 0 {
		t.Fatalf("relations: %+v", facts[0].ContinuityFacts)
	}
}
