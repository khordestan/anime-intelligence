package catalog

import (
	"math"
	"testing"
)

func TestEvaluateTopicSelectorAbstentionAndRejectedModelOutput(t *testing.T) {
	valid := []string{"genres", "episodeCount"}
	invalid := []string{"synopsis"}
	abstain := []string{}
	suite := SelectorEvaluationSuite{Version: 1, Cases: []SelectorJudgment{
		{ID: "supported", Question: "What genres and how many episodes?", ExpectedFields: []string{"episodeCount", "genres"}, ModelFields: &valid},
		{ID: "unsupported", Question: "What did the ending mean?", ExpectedFields: []string{}, ModelFields: &abstain},
		{ID: "rejected", Question: "What is the demographic?", ExpectedFields: []string{"demographics"}, ModelFields: &invalid},
		{ID: "deterministic", Question: "Which studio?", ExpectedFields: []string{"studios", "genres"}},
	}}
	result, err := EvaluateTopicSelector(suite)
	if err != nil || result.Deterministic.Exact != 3 || result.Deterministic.Abstained != 1 ||
		result.Deterministic.MeanPrecision != 1 || math.Abs(result.Deterministic.MeanRecall-0.875) > 1e-9 {
		t.Fatalf("deterministic evaluation: %+v, %v", result, err)
	}
	if result.Model == nil || result.Model.Cases != 3 || result.Model.Exact != 2 ||
		result.Model.Invalid != 1 || result.Model.Abstained != 1 || !result.Cases[2].InvalidModel || result.Cases[3].Model != nil {
		t.Fatalf("replay evaluation: %+v", result)
	}
}
