package catalogsync

import (
	"strings"
	"testing"
	"time"
)

func TestCatalogPrometheusAggregatesOnly(t *testing.T) {
	s := Status{AsOf: time.Unix(2000, 0), StaleAfterSeconds: 86400, Titles: 8, ObservedTitles: 3, UnobservedTitles: 5, StaleTitles: 2, FutureTitles: 1, EpisodeSchedules: 4, ConfirmedEpisodes: 1, Sources: []SourceStatus{{Provider: "Synthetic hidden provider"}}, Coverage: MetadataCoverage{Genres: 2}}
	text, err := Prometheus(s)
	if err != nil || !strings.Contains(text, "anime_catalog_metadata_genres 2\n") || !strings.Contains(text, "anime_catalog_unobserved_titles 5\n") || strings.Contains(text, "Synthetic") {
		t.Fatal("aggregate export", err)
	}
	if strings.Count(text, "# TYPE ") != 29 {
		t.Fatal("metric contract changed")
	}
	for _, line := range strings.Split(text, "\n") {
		if strings.Contains(line, "{") && !strings.HasPrefix(line, "anime_catalog_sources_by_last_fetch_outcome{outcome=") {
			t.Fatalf("unexpected metric labels: %s", line)
		}
	}
	for _, mutate := range []func(*Status){func(s *Status) { s.Titles = -1 }, func(s *Status) { s.AsOf = time.Time{} }, func(s *Status) { s.ConfirmedEpisodes = 5 }, func(s *Status) { s.FutureTitles = 3 }, func(s *Status) { s.UnobservedTitles = 0 }, func(s *Status) { s.Coverage.Genres = 4 }} {
		invalid := s
		mutate(&invalid)
		if text, err := Prometheus(invalid); err != ErrStatus || text != "" {
			t.Fatal("invalid snapshot exported")
		}
	}
}

func TestCatalogCoverageDeficitsIncludeUnobservedTitles(t *testing.T) {
	s := Status{AsOf: time.Unix(2000, 0), StaleAfterSeconds: 86400, Titles: 8, ObservedTitles: 3, UnobservedTitles: 5, Coverage: MetadataCoverage{Genres: 2, EpisodeTotals: 3}}
	text, err := Prometheus(s)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"anime_catalog_missing_metadata_genres 6\n", "anime_catalog_missing_metadata_episode_totals 5\n", "anime_catalog_missing_metadata_studios 8\n"} {
		if !strings.Contains(text, want) {
			t.Fatal("wrong coverage deficit", want)
		}
	}
	empty := Status{AsOf: s.AsOf, StaleAfterSeconds: 86400}
	if text, err = Prometheus(empty); err != nil || !strings.Contains(text, "anime_catalog_missing_metadata_genres 0\n") {
		t.Fatal("empty catalog deficit", err)
	}
}
