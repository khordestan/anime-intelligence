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

func TestCreditQualityMatchesCompletePairs(t *testing.T) {
	policy, err := DecodeQualityPolicy(strings.NewReader(`{"version":1,"minTitles":2,"presence":{"staffCredits":1,"characterCredits":0.5},
	"accuracy":{"staffCredits":{"minJudgments":2,"minCorrectRate":0.5},"characterCredits":{"minJudgments":2,"minCorrectRate":0.5}}}`))
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	candidate := Snapshot{Version: 1, Provider: "synthetic", ObservedAt: now, Titles: []Title{
		{ExternalID: "a", Title: "Synthetic A", MediaType: "TV", Status: "UNKNOWN",
			StaffCredits:     []Credit{{"Person One", "Director"}, {"Person Two", "Writer"}},
			CharacterCredits: []Credit{{"Character One", "Lead"}}},
		{ExternalID: "b", Title: "Synthetic B", MediaType: "TV", Status: "UNKNOWN",
			StaffCredits: []Credit{{"Person One", "Writer"}, {"Person Two", "Director"}}},
	}}
	reference := candidate
	reference.Titles = append([]Title(nil), candidate.Titles...)
	reference.Titles[0].StaffCredits = []Credit{{"person two", "writer"}, {"PERSON ONE", "DIRECTOR"}}
	reference.Titles[1].StaffCredits = candidate.Titles[0].StaffCredits
	reference.Titles[1].CharacterCredits = []Credit{{"Character Two", "Supporting"}}
	accuracy, err := CompareReviewedFields(candidate, reference, now)
	if err != nil {
		t.Fatal(err)
	}
	if accuracy.Fields["staffCredits"] != (JudgedField{Correct: 1, Mismatch: 1}) ||
		accuracy.Fields["characterCredits"] != (JudgedField{Correct: 1, Missing: 1}) {
		t.Fatalf("unexpected credit judgments: %+v", accuracy.Fields)
	}
	result, err := EvaluateQuality(policy, candidate.Coverage(), &accuracy)
	if err != nil || !result.Passed || result.Presence["staffCredits"].Numerator != 2 ||
		result.Presence["characterCredits"].Numerator != 1 || result.Accuracy["staffCredits"].Denominator != 2 {
		t.Fatalf("unexpected credit quality: %+v, %v", result, err)
	}
}

func TestReviewedEpisodeAccuracyUsesExactTitleEpisodeAndTrack(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	release := now.Add(-time.Hour)
	sameInstant := release.In(time.FixedZone("Fixture offset", -4*60*60))
	later := release.Add(time.Minute)
	candidate := Snapshot{Version: 1, Provider: "synthetic", ObservedAt: now, Titles: []Title{
		{ExternalID: "known", Title: "Synthetic known", MediaType: "TV", Status: "UNKNOWN", Episodes: []Episode{
			{Number: 1, Track: "SUB", ScheduledAt: &sameInstant, ReleasedAt: &later},
			{Number: 1, Track: "DUB", ReleasedAt: &sameInstant},
			{Number: 2, Track: "SUB", ScheduledAt: &later},
			{Number: 3, Track: "SUB", ReleasedAt: &sameInstant},
		}},
	}}
	reference := Snapshot{Version: 1, Provider: "synthetic", ObservedAt: now, Titles: []Title{
		{ExternalID: "known", Title: "Synthetic known", MediaType: "TV", Status: "UNKNOWN", Episodes: []Episode{
			{Number: 1, Track: "SUB", ScheduledAt: &release, ReleasedAt: &release},
			{Number: 1, Track: "DUB", ReleasedAt: &release},
			{Number: 2, Track: "SUB", ReleasedAt: &release},
			{Number: 3, Track: "RAW", ReleasedAt: &release},
		}},
		{ExternalID: "missing", Title: "Synthetic missing", MediaType: "TV", Status: "UNKNOWN", Episodes: []Episode{
			{Number: 1, Track: "SUB", ReleasedAt: &release},
		}},
	}}
	accuracy, err := CompareReviewedFields(candidate, reference, now)
	if err != nil {
		t.Fatal(err)
	}
	for key, want := range map[string]JudgedField{
		"scheduledAt.SUB": {Correct: 1},
		"releasedAt.SUB":  {Mismatch: 1, Missing: 2},
		"releasedAt.DUB":  {Correct: 1},
		"releasedAt.RAW":  {Missing: 1},
	} {
		if accuracy.Fields[key] != want {
			t.Fatalf("%s judgments = %+v, want %+v", key, accuracy.Fields[key], want)
		}
	}
	policy, err := DecodeQualityPolicy(strings.NewReader(`{"version":1,"minTitles":1,"accuracy":{
		"releasedAt.DUB":{"minJudgments":1,"minCorrectRate":1},"releasedAt.SUB":{"minJudgments":3,"minCorrectRate":0.5}}}`))
	if err != nil {
		t.Fatal(err)
	}
	result, err := EvaluateQuality(policy, candidate.Coverage(), &accuracy)
	if err != nil || result.Passed || !result.Accuracy["releasedAt.DUB"].Passed || result.Accuracy["releasedAt.SUB"].Denominator != 3 {
		t.Fatalf("unexpected episode quality: %+v, %v", result, err)
	}
}
