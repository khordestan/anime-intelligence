package catalog

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/khordestan/anime-intelligence/internal/platform/store"
)

func catalogFilter(r *http.Request) (store.CatalogFilter, error) {
	query, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		return store.CatalogFilter{}, store.ErrCatalogFilter
	}
	for _, key := range []string{"q", "mediaType", "status", "releaseYear", "season", "genre", "theme", "demographic", "studio", "staff", "minEpisodes", "maxEpisodes", "sort", "minYear", "maxYear", "confirmedRelease", "metadataProvider", "confirmedTrack", "seasonNumber", "partNumber"} {
		if values, ok := query[key]; ok && (len(values) != 1 || (key != "q" && strings.TrimSpace(values[0]) == "")) {
			return store.CatalogFilter{}, store.ErrCatalogFilter
		}
	}
	f := store.CatalogFilter{ConfirmedTrack: query.Get("confirmedTrack"), MetadataProvider: query.Get("metadataProvider"), Sort: query.Get("sort"), Query: query.Get("q"), MediaType: query.Get("mediaType"), Status: query.Get("status"), Season: query.Get("season"), Genre: query.Get("genre"), Theme: query.Get("theme"), Demographic: query.Get("demographic"), Studio: query.Get("studio"), Staff: query.Get("staff")}
	if values, ok := query["confirmedRelease"]; ok {
		if values[0] != "true" && values[0] != "false" {
			return f, store.ErrCatalogFilter
		}
		f.ConfirmedRelease = values[0] == "true"
	}
	if year, ok := query["releaseYear"]; ok {
		n, err := strconv.Atoi(year[0])
		if err != nil || n == 0 {
			return f, store.ErrCatalogFilter
		}
		f.ReleaseYear = n
	}
	for _, bound := range []struct {
		key    string
		target **int
	}{{"minEpisodes", &f.MinEpisodes}, {"maxEpisodes", &f.MaxEpisodes}, {"minYear", &f.MinYear}, {"maxYear", &f.MaxYear}, {"seasonNumber", &f.SeasonNumber}, {"partNumber", &f.PartNumber}} {
		if values, ok := query[bound.key]; ok {
			n, err := strconv.Atoi(values[0])
			if err != nil {
				return f, store.ErrCatalogFilter
			}
			*bound.target = &n
		}
	}
	return f.Normalize()
}
