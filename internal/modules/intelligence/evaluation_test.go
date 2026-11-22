package intelligence

import (
	"math"
	"testing"
)

func TestEvaluatePersonalMatchUsesHeldOutJudgments(t *testing.T) {
	suite := EvaluationSuite{Version: 1, Cases: []EvaluationCase{{
		ID: "synthetic",
		Rated: []RatedAnime{{AnimeID: "training", Score: 5, Features: []FeatureValue{{Key: "feature", Value: 1, Confidence: 1}}}},
		Candidates: []Candidate{
			{AnimeID: "training", Title: "Already rated", Features: []FeatureValue{{Key: "feature", Value: 1, Confidence: 1}}},
			{AnimeID: "other", Title: "Alphabetically first", Features: []FeatureValue{{Key: "feature", Value: 0.1, Confidence: 1}}},
			{AnimeID: "relevant", Title: "Relevant candidate", Features: []FeatureValue{{Key: "feature", Value: 1, Confidence: 1}}},
		},
		Relevant: []string{"relevant"},
	}}}
	report, err := EvaluatePersonalMatch(suite, 1)
	if err != nil || len(report.Cases) != 1 || report.PersonalMatch != (RankingMetrics{Recall: 1, ReciprocalRank: 1, NDCG: 1, FillRate: 1}) ||
		report.TitleBaseline != (RankingMetrics{FillRate: 1}) {
		t.Fatalf("report=%+v err=%v", report, err)
	}
	report, err = EvaluatePersonalMatch(suite, 2)
	if err != nil || report.TitleBaseline.Recall != 1 || report.TitleBaseline.ReciprocalRank != .5 ||
		math.Abs(report.TitleBaseline.NDCG-1/math.Log2(3)) > 1e-12 || report.TitleBaseline.FillRate != 1 {
		t.Fatalf("top-two report=%+v err=%v", report, err)
	}
	suite.Cases[0].Rated = nil
	report, err = EvaluatePersonalMatch(suite, 2)
	if err != nil || report.PersonalMatch != (RankingMetrics{}) || report.TitleBaseline.FillRate != 1 {
		t.Fatalf("cold-start report=%+v err=%v", report, err)
	}
	suite.Cases[0].Rated = []RatedAnime{{AnimeID: "training", Score: 5}}
	suite.Cases[0].Relevant = []string{"training"}
	if _, err := EvaluatePersonalMatch(suite, 2); err == nil {
		t.Fatal("training title accepted as a held-out judgment")
	}
}
