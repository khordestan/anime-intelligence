package intelligence

import "testing"

func TestEvaluateDiscoveryComparesKnownCoverageWithUnfamiliarEvidence(t *testing.T) {
	suite := DiscoveryEvaluationSuite{Version: 1, Cases: []DiscoveryEvaluationCase{{
		ID: "synthetic", Feature: "action", KnownFeatures: []string{"action", "adventure"},
		Candidates: []DiscoveryCandidate{
			{ID: "a", Features: []string{"action", "adventure"}},
			{ID: "b", Features: []string{"action", "mystery"}},
			{ID: "hidden", Features: []string{"action", "adventure", "mystery"}, Excluded: true},
			{ID: "outside-filter", Features: []string{"mystery", "short"}},
		},
		Relevant: []string{"b"},
	}}}
	report, err := EvaluateDiscovery(suite, 1)
	if err != nil || len(report.Cases) != 1 ||
		report.Coverage != (RankingMetrics{FillRate: 1}) ||
		report.Unfamiliar != (RankingMetrics{FillRate: 1, Recall: 1, ReciprocalRank: 1, NDCG: 1}) {
		t.Fatalf("ranking comparison: %+v %v", report, err)
	}
	suite.Cases[0].Relevant = []string{"hidden"}
	if _, err := EvaluateDiscovery(suite, 1); err == nil {
		t.Fatal("accepted judgment for an excluded title")
	}
}
