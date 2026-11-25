package feed

import (
	"testing"
	"time"
)

func TestReviewedFieldAccuracySeparatesMissingFromConflictingFields(t *testing.T) {
	now := time.Date(2026, 11, 25, 12, 0, 0, 0, time.UTC)
	zero := 0
	reference := Snapshot{Version: 1, Provider: "synthetic", ObservedAt: now, Titles: []Title{
		{ExternalID: "one", Title: "Synthetic anime", MediaType: "TV", Status: "ANNOUNCED", ReleaseYear: 2026,
			EpisodeCount: &zero, Genres: []string{"Drama", "Mystery"}},
		{ExternalID: "two", Title: "Synthetic unmatched anime", MediaType: "TV", Status: "UNKNOWN", EpisodeCount: &zero},
	}}
	candidate := Snapshot{Version: 1, Provider: "synthetic", ObservedAt: now.Add(-time.Hour), Titles: []Title{
		{ExternalID: "one", Title: "Synthetic anime", MediaType: "TV", Status: "UNKNOWN", ReleaseYear: 2024,
			EpisodeCount: &zero, Genres: []string{" mystery", "drama"}},
		{ExternalID: "elsewhere", Title: "Synthetic unmatched anime", MediaType: "TV", Status: "UNKNOWN"},
	}}
	report, err := CompareReviewedFields(candidate, reference, now)
	if err != nil || report.ReviewedTitles != 2 || report.MatchedTitles != 1 {
		t.Fatalf("report: %+v %v", report, err)
	}
	want := map[string]JudgedField{
		"title": {Correct: 1, Missing: 1}, "mediaType": {Correct: 1, Missing: 1},
		"status": {Missing: 1}, "releaseYear": {Mismatch: 1},
		"episodeCount": {Correct: 1, Missing: 1}, "genres": {Correct: 1},
	}
	for key, expected := range want {
		if got := report.Fields[key]; got != expected {
			t.Errorf("%s: got %+v, want %+v", key, got, expected)
		}
	}
	if _, found := report.Fields["nativeTitle"]; found {
		t.Error("unknown reference field contributed a judgment")
	}
	reference.Provider = "other-provider"
	if _, err := CompareReviewedFields(candidate, reference, now); err == nil {
		t.Error("accepted incompatible provider identities")
	}
}
