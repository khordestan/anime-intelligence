package intelligence

import "testing"

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
	if err != nil || len(report.Cases) != 1 || report.PersonalMatch != (RankingMetrics{Recall: 1, ReciprocalRank: 1}) ||
		report.TitleBaseline != (RankingMetrics{}) {
		t.Fatalf("report=%+v err=%v", report, err)
	}
	report, err = EvaluatePersonalMatch(suite, 2)
	if err != nil || report.TitleBaseline != (RankingMetrics{Recall: 1, ReciprocalRank: .5}) {
		t.Fatalf("top-two report=%+v err=%v", report, err)
	}
	suite.Cases[0].Relevant = []string{"training"}
	if _, err := EvaluatePersonalMatch(suite, 2); err == nil {
		t.Fatal("training title accepted as a held-out judgment")
	}
}
