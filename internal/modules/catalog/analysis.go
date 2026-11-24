package catalog

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
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

func (h *Handler) Analyze(w http.ResponseWriter, r *http.Request) {
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
	fields, matched := selectAnalysisFields(request.Question)
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	items := make([]catalogAnalysisTitle, 0, len(request.AnimeIDs))
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
		item := catalogAnalysisTitle{AnimeID: details.Anime.ID, Title: details.Anime.Title}
		for _, field := range fields {
			item.Evidence = append(item.Evidence, projectAnalysisFact(details, field))
		}
		items = append(items, item)
	}
	writeJSON(w, http.StatusOK, struct {
		Engine       string                 `json:"engine"`
		TopicMatched bool                   `json:"topicMatched"`
		Items        []catalogAnalysisTitle `json:"items"`
	}{Engine: "catalog-facts-v1", TopicMatched: matched, Items: items})
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
		for _, field := range analysisFields {
			selected = append(selected, field.key)
		}
	}
	return selected, matched
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
