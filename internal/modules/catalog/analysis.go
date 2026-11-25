package catalog

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/khordestan/anime-intelligence/internal/platform/store"
)

type catalogAnalysisFact struct {
	Field      string     `json:"field"`
	Values     []string   `json:"values"`
	Provider   string     `json:"provider,omitempty"`
	ObservedAt *time.Time `json:"observedAt,omitempty"`
	Truncated  bool       `json:"truncated,omitempty"`
}

type catalogAnalysisTitle struct {
	AnimeID  string                `json:"animeId"`
	Title    string                `json:"title"`
	Evidence []catalogAnalysisFact `json:"evidence"`
}

type catalogComparison struct {
	Field      string   `json:"field"`
	Difference int      `json:"difference"`
	AnimeIDs   []string `json:"animeIds"`
}

type catalogSharedEvidence struct {
	Field  string   `json:"field"`
	Values []string `json:"values"`
}

var analysisFields = []struct {
	key   string
	words []string
}{
	{"mediaType", []string{"format", "movie", "film", "series", "tv", "type"}},
	{"releaseYear", []string{"year", "date", "when", "released", "premiered"}},
	{"episodeCount", []string{"episode", "episodes", "length", "count"}},
	{"status", []string{"status", "finished", "ongoing", "cancelled", "announced", "airing"}},
	{"genres", []string{"genre", "genres", "category", "categories"}},
	{"studios", []string{"studio", "studios", "animated", "production"}},
}

type CatalogFieldSelector func(context.Context, string, []string) ([]string, string, error)

var ErrAnalysisCapacity = errors.New("analysis capacity reached")

func (h *Handler) Analyze(w http.ResponseWriter, r *http.Request) {
	h.AnalyzeWithSelector(w, r, nil)
}

func (h *Handler) AnalyzeWithSelector(w http.ResponseWriter, r *http.Request, selector CatalogFieldSelector) {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
	decoder.DisallowUnknownFields()
	var request struct {
		Question string   `json:"question"`
		AnimeIDs []string `json:"animeIds"`
	}
	if decoder.Decode(&request) != nil || decoder.Decode(new(any)) != io.EOF ||
		strings.TrimSpace(request.Question) == "" || len(request.Question) > 400 ||
		len(request.AnimeIDs) < 1 || len(request.AnimeIDs) > 2 {
		writeError(w, http.StatusBadRequest, "invalid_catalog_question", "Enter a question and select one or two anime")
		return
	}
	for i, id := range request.AnimeIDs {
		if !validUUID(id) {
			writeError(w, http.StatusBadRequest, "invalid_id", "Anime ID must be a UUID")
			return
		}
		request.AnimeIDs[i] = strings.ToLower(id)
	}
	if len(request.AnimeIDs) == 2 && request.AnimeIDs[0] == request.AnimeIDs[1] {
		writeError(w, http.StatusBadRequest, "duplicate_anime", "Select different anime to compare")
		return
	}
	data, ok := h.data.(store.DetailedCatalog)
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "catalog_unavailable", "Catalog facts are unavailable")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	detailsList := make([]store.CatalogDetails, 0, len(request.AnimeIDs))
	for _, id := range request.AnimeIDs {
		details, found, err := data.CatalogDetails(ctx, id)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "catalog_read_failed", "Could not read catalog facts")
			return
		}
		if !found {
			writeError(w, http.StatusNotFound, "anime_not_found", "Anime was not found")
			return
		}
		detailsList = append(detailsList, details)
	}
	cancel()

	engine := "catalog-facts-v1"
	fields, matched := selectAnalysisFields(request.Question)
	var model string
	if selector != nil {
		allowed := allAnalysisFields()
		selected, selectedModel, err := selector(r.Context(), request.Question, slices.Clone(allowed))
		if errors.Is(err, ErrAnalysisCapacity) {
			w.Header().Set("Retry-After", "60")
			writeError(w, http.StatusTooManyRequests, "analysis_capacity", "Model-assisted analysis is busy")
			return
		}
		fields, matched, ok = validateSelectedFields(allowed, selected)
		if err != nil || !ok || strings.TrimSpace(selectedModel) == "" || len(selectedModel) > 128 {
			writeError(w, http.StatusBadGateway, "analysis_provider_failed", "Model-assisted analysis is unavailable")
			return
		}
		engine, model = "model-selected-catalog-facts-v1", selectedModel
	}
	items := make([]catalogAnalysisTitle, 0, len(detailsList))
	for _, details := range detailsList {
		item := catalogAnalysisTitle{AnimeID: details.Anime.ID, Title: details.Anime.Title}
		for _, field := range fields {
			item.Evidence = append(item.Evidence, projectAnalysisFact(details, field))
		}
		items = append(items, item)
	}

	writeJSON(w, http.StatusOK, struct {
		Engine         string                  `json:"engine"`
		Model          string                  `json:"model,omitempty"`
		TopicMatched   bool                    `json:"topicMatched"`
		Items          []catalogAnalysisTitle  `json:"items"`
		Comparisons    []catalogComparison     `json:"comparisons,omitempty"`
		SharedEvidence []catalogSharedEvidence `json:"sharedEvidence,omitempty"`
	}{Engine: engine, Model: model, TopicMatched: matched, Items: items, Comparisons: compareAnalysisFacts(items), SharedEvidence: sharedAnalysisEvidence(items)})
}

// compareAnalysisFacts reports the first title's value minus the second's.
// Only fields with two known numeric values can support these differences.
func compareAnalysisFacts(items []catalogAnalysisTitle) []catalogComparison {
	if len(items) != 2 {
		return nil
	}
	var comparisons []catalogComparison
	for _, first := range items[0].Evidence {
		if first.Field != "episodeCount" && first.Field != "releaseYear" {
			continue
		}
		index := slices.IndexFunc(items[1].Evidence, func(fact catalogAnalysisFact) bool { return fact.Field == first.Field })
		if index < 0 {
			continue
		}
		second := items[1].Evidence[index]
		if len(first.Values) != 1 || len(second.Values) != 1 {
			continue
		}
		left, leftErr := strconv.Atoi(first.Values[0])
		right, rightErr := strconv.Atoi(second.Values[0])
		if leftErr == nil && rightErr == nil {
			comparisons = append(comparisons, catalogComparison{
				Field: first.Field, Difference: left - right, AnimeIDs: []string{items[0].AnimeID, items[1].AnimeID},
			})
		}
	}
	return comparisons
}

// Compare only the bounded evidence returned for both titles. With incomplete
// lists, a shared value is still grounded in both sources.
func sharedAnalysisEvidence(items []catalogAnalysisTitle) []catalogSharedEvidence {
	if len(items) != 2 {
		return nil
	}
	var shared []catalogSharedEvidence
	for _, first := range items[0].Evidence {
		if first.Field != "genres" && first.Field != "studios" {
			continue
		}
		index := slices.IndexFunc(items[1].Evidence, func(fact catalogAnalysisFact) bool {
			return fact.Field == first.Field
		})
		if index < 0 {
			continue
		}
		var values []string
		for _, value := range first.Values {
			sameLabel := func(candidate string) bool { return strings.EqualFold(value, candidate) }
			if slices.ContainsFunc(items[1].Evidence[index].Values, sameLabel) && !slices.ContainsFunc(values, sameLabel) {
				values = append(values, value)
			}
		}
		if len(values) > 0 {
			shared = append(shared, catalogSharedEvidence{Field: first.Field, Values: values})
		}
	}
	return shared
}

func selectAnalysisFields(question string) ([]string, bool) {
	words := strings.FieldsFunc(strings.ToLower(question), func(r rune) bool { return !unicode.IsLetter(r) })
	tokens := make(map[string]bool, len(words))
	for _, word := range words {
		tokens[word] = true
	}
	var selected []string
	for _, field := range analysisFields {
		for _, word := range field.words {
			if tokens[word] {
				selected = append(selected, field.key)
				break
			}
		}
	}
	matched := len(selected) > 0
	if !matched {
		selected = allAnalysisFields()
	}
	return selected, matched
}

func allAnalysisFields() []string {
	selected := make([]string, 0, len(analysisFields))
	for _, field := range analysisFields {
		selected = append(selected, field.key)
	}
	return selected
}

func validateSelectedFields(allowed, selected []string) ([]string, bool, bool) {
	if len(selected) == 0 {
		return allowed, false, true
	}
	chosen := make(map[string]bool, len(allowed))
	for _, key := range selected {
		if chosen[key] || !slices.Contains(allowed, key) {
			return nil, false, false
		}
		chosen[key] = true
	}
	fields := make([]string, 0, len(selected))
	for _, field := range allowed {
		if chosen[field] {
			fields = append(fields, field)
		}
	}
	return fields, true, true
}

func projectAnalysisFact(details store.CatalogDetails, field string) catalogAnalysisFact {
	fact := catalogAnalysisFact{Field: field, Values: []string{}}
	switch field {
	case "mediaType":
		if details.Anime.MediaType != "" && details.Anime.MediaType != "UNKNOWN" {
			fact.Values = []string{details.Anime.MediaType}
		}
	case "releaseYear":
		if details.Anime.ReleaseYear != 0 {
			fact.Values = []string{strconv.Itoa(details.Anime.ReleaseYear)}
		}
	case "episodeCount":
		if details.Anime.EpisodeCountKnown {
			fact.Values = []string{strconv.Itoa(details.Anime.EpisodeCount)}
		}
	case "status":
		if details.Anime.Status != "" && details.Anime.Status != "UNKNOWN" {
			fact.Values = []string{details.Anime.Status}
		}
	case "genres":
		fact.Values = append(fact.Values, details.Genres...)
	case "studios":
		fact.Values = append(fact.Values, details.Studios...)
	}
	if len(fact.Values) == 0 {
		return fact
	}
	const maxValues = 20
	if len(fact.Values) > maxValues {
		fact.Values = fact.Values[:maxValues]
		fact.Truncated = true
	}
	fact.Provider, fact.ObservedAt = details.Provider, details.ObservedAt
	for _, source := range details.FieldSources {
		if source.Field == field {
			fact.Provider, fact.ObservedAt = source.Provider, source.ObservedAt
			break
		}
	}
	return fact
}
