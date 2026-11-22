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

func TestEvaluatePersonalMatchMeasuresRetrievalLossSeparately(t *testing.T) {
	first := FeatureValue{Key: "first", Value: 1, Confidence: 1}
	second := FeatureValue{Key: "second", Value: .1, Confidence: 1}
	suite := EvaluationSuite{Version: 1, CandidateLimit: 1, Cases: []EvaluationCase{{
		ID: "retrieval",
		Preferences: map[string]Preference{
			"first": {Weight: 1, Confidence: 1},
			"second": {Weight: 1, Confidence: 1},
			"neutral": {Weight: 0, Confidence: 1},
		},
		Candidates: []Candidate{
			{AnimeID: "1", Title: "Alphabetically first", Features: []FeatureValue{first, second}},
			{AnimeID: "2", Title: "Held-out relevant", Features: []FeatureValue{first}},
			{AnimeID: "0", Title: "Neutral evidence only", Features: []FeatureValue{{Key: "neutral", Value: 1, Confidence: 1}}},
		},
		Relevant: []string{"2"},
	}}}
	report, err := EvaluatePersonalMatch(suite, 2)
	if err != nil || len(report.Cases) != 1 || report.MeanRetrievalRecall == nil || *report.MeanRetrievalRecall != 0 ||
		report.Cases[0].Retrieval == nil || report.Cases[0].Retrieval.CandidateCount != 1 || report.PersonalMatch.FillRate != .5 {
		t.Fatalf("capped report=%+v err=%v", report, err)
	}
	suite.CandidateLimit = 2
	report, err = EvaluatePersonalMatch(suite, 2)
	if err != nil || report.MeanRetrievalRecall == nil || *report.MeanRetrievalRecall != 1 ||
		report.Cases[0].Retrieval.CandidateCount != 2 || report.PersonalMatch.ReciprocalRank != 1 {
		t.Fatalf("expanded report=%+v err=%v", report, err)
	}
	suite.CandidateLimit = 0
	report, err = EvaluatePersonalMatch(suite, 2)
	if err != nil || report.MeanRetrievalRecall != nil || report.Cases[0].Retrieval != nil {
		t.Fatalf("default report=%+v err=%v", report, err)
	}
}
