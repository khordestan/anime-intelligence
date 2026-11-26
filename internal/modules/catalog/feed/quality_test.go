package feed

import (
	"strings"
	"testing"
	"time"
)

func TestQualityGateSeparatesPresenceJudgmentsAndMissingValues(t *testing.T) {
	policy, err := DecodeQualityPolicy(strings.NewReader(`{"version":1,"minTitles":2,"presence":{"seasonNumber":0.5},
	"accuracy":{"partNumber":{"minJudgments":2,"minCorrectRate":0.5},"demographics":{"minJudgments":1,"minCorrectRate":1}}}`))
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	candidate := Snapshot{Version: 1, Provider: "synthetic", ObservedAt: now, Titles: []Title{
		{ExternalID: "a", Title: "Synthetic A", MediaType: "TV", Status: "UNKNOWN", SeasonNumber: 2, PartNumber: 1},
		{ExternalID: "b", Title: "Synthetic B", MediaType: "TV", Status: "UNKNOWN", Demographics: []string{"Synthetic demographic"}},
	}}
	reference := candidate
	reference.Titles = append([]Title(nil), candidate.Titles...)
	reference.Titles[1].SeasonNumber = 2
	reference.Titles[1].PartNumber = 2
	accuracy, err := CompareReviewedFields(candidate, reference, now)
	if err != nil {
		t.Fatal(err)
	}
	out, err := EvaluateQuality(policy, candidate.Coverage(), &accuracy)
	if err != nil || !out.Passed || !out.EnoughCandidates || out.Presence["seasonNumber"].Numerator != 1 ||
		out.Accuracy["partNumber"].Numerator != 1 || out.Accuracy["partNumber"].Denominator != 2 ||
		out.Accuracy["demographics"].Denominator != 1 {
		t.Fatalf("quality: %+v, err=%v", out, err)
	}

	delete(accuracy.Fields, "demographics")
	out, err = EvaluateQuality(policy, candidate.Coverage(), &accuracy)
	if err != nil || out.Passed || out.Accuracy["demographics"].Passed || out.Accuracy["demographics"].Rate != nil {
		t.Fatalf("no judgments must fail: %+v, err=%v", out, err)
	}
	if _, err = EvaluateQuality(policy, candidate.Coverage(), nil); err == nil {
		t.Fatal("accepted an accuracy policy without judgments")
	}
	policy.Presence["seasonNumber"] = nil
	if _, err = EvaluateQuality(policy, candidate.Coverage(), &accuracy); err == nil {
		t.Fatal("accepted a null threshold")
	}
}
