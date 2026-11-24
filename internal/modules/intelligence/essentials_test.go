package intelligence

import (
	"reflect"
	"slices"
	"testing"
)

func TestGlobalEssentialsEvaluation(t *testing.T) {
	award := EssentialsEvidence{Kind: JuriedAwardWin, PublisherID: "award", SourceURL: "https://award.example/results"}
	selection := EssentialsEvidence{Kind: InstitutionalSelection, PublisherID: "archive", SourceURL: "https://archive.example/registry"}
	secondAward := EssentialsEvidence{Kind: JuriedAwardWin, PublisherID: "other-award", SourceURL: "https://other-award.example/results"}
	fixture := EssentialsEvaluation{
		Version: 1,
		Candidates: []EssentialsCandidate{
			{AnimeID: "watch", Title: "A Title"},
			{AnimeID: "single", Title: "B Title", Evidence: []EssentialsEvidence{award, award}},
			{AnimeID: "parallel", Title: "C Title", Evidence: []EssentialsEvidence{secondAward, award}},
			{AnimeID: "broad", Title: "Z Title", Evidence: []EssentialsEvidence{award, selection}},
		},
		Relevant: []string{"broad", "watch"},
	}
	report, err := EvaluateGlobalEssentials(fixture, 3)
	if err != nil {
		t.Fatal(err)
	}
	if got := report.Ranking; len(got) != 3 || got[0].AnimeID != "broad" ||
		got[1].AnimeID != "parallel" || got[2].AnimeID != "single" ||
		got[2].PublisherCount != 1 || len(got[2].Evidence) != 1 {
		t.Fatalf("unexpected ranking: %+v", got)
	}
	if report.GlobalEssentials.Recall != 0.5 || report.GlobalEssentials.ReciprocalRank != 1 ||
		report.TitleBaseline.Recall != 0.5 || report.GlobalEssentials.FillRate != 1 {
		t.Fatalf("unexpected metrics: %+v", report)
	}
	wantCoverage := EssentialsCoverage{
		Candidates: 4, EligibleCandidates: 3, Relevant: 2, EligibleRelevant: 1, RecallCeilingAtK: 0.5,
	}
	if report.Coverage != wantCoverage {
		t.Fatalf("coverage = %+v, want %+v", report.Coverage, wantCoverage)
	}
	slices.Reverse(fixture.Candidates)
	for _, candidate := range fixture.Candidates {
		slices.Reverse(candidate.Evidence)
	}
	reversed, err := EvaluateGlobalEssentials(fixture, 3)
	if err != nil || !reflect.DeepEqual(report, reversed) {
		t.Fatalf("input order changed results: %+v, %v", reversed, err)
	}
}

func TestGlobalEssentialsRejectsInvalidEvidence(t *testing.T) {
	for _, evidence := range []EssentialsEvidence{
		{Kind: JuriedAwardWin, PublisherID: "org", SourceURL: "http://award.example/results"},
		{Kind: InstitutionalSelection, PublisherID: "org", SourceURL: "https://user@archive.example/selection"},
		{Kind: "AUDIENCE_VOTE", PublisherID: "org", SourceURL: "https://award.example/results"},
	} {
		candidates := []EssentialsCandidate{{AnimeID: "test", Title: "Test", Evidence: []EssentialsEvidence{evidence}}}
		if _, err := RankGlobalEssentials(candidates, 1); err == nil {
			t.Fatalf("accepted invalid evidence: %+v", evidence)
		}
	}
}
