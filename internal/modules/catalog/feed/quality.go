package feed

import (
	"encoding/json"
	"io"
	"math"
	"slices"
)

type QualityPolicy struct {
	Version   int                          `json:"version"`
	MinTitles int                          `json:"minTitles"`
	Presence  map[string]*float64          `json:"presence"`
	Accuracy  map[string]AccuracyThreshold `json:"accuracy"`
}

type AccuracyThreshold struct {
	MinJudgments   int      `json:"minJudgments"`
	MinCorrectRate *float64 `json:"minCorrectRate"`
}

type QualityResult struct {
	Numerator   int      `json:"numerator"`
	Denominator int      `json:"denominator"`
	Rate        *float64 `json:"rate"`
	Passed      bool     `json:"passed"`
}

type QualityReport struct {
	Passed           bool                     `json:"passed"`
	EnoughCandidates bool                     `json:"enoughCandidates"`
	Presence         map[string]QualityResult `json:"presence,omitempty"`
	Accuracy         map[string]QualityResult `json:"accuracy,omitempty"`
}

func DecodeQualityPolicy(reader io.Reader) (QualityPolicy, error) {
	const maxBytes = 64 << 10
	limited := &io.LimitedReader{R: reader, N: maxBytes + 1}
	decoder := json.NewDecoder(limited)
	decoder.DisallowUnknownFields()
	var policy QualityPolicy
	if decoder.Decode(&policy) != nil || decoder.Decode(new(any)) != io.EOF || limited.N <= 0 {
		return QualityPolicy{}, ErrInvalid
	}
	return policy, nil
}

func EvaluateQuality(policy QualityPolicy, coverage SnapshotCoverage, reference *FieldAccuracy) (QualityReport, error) {
	if policy.Version != 1 || policy.MinTitles < 1 || policy.MinTitles > 10000 ||
		len(policy.Presence)+len(policy.Accuracy) == 0 || (len(policy.Accuracy) > 0 && reference == nil) {
		return QualityReport{}, ErrInvalid
	}
	out := QualityReport{EnoughCandidates: coverage.Titles >= policy.MinTitles}
	out.Passed = out.EnoughCandidates
	if len(policy.Presence) > 0 {
		out.Presence = make(map[string]QualityResult, len(policy.Presence))
	}
	for field, threshold := range policy.Presence {
		present, known := coverage.Fields[field]
		if !known || threshold == nil || !validQualityRate(*threshold) {
			return QualityReport{}, ErrInvalid
		}
		result := qualityRate(present, coverage.Titles, policy.MinTitles, *threshold)
		out.Presence[field] = result
		out.Passed = out.Passed && result.Passed
	}
	if len(policy.Accuracy) > 0 {
		out.Accuracy = make(map[string]QualityResult, len(policy.Accuracy))
	}
	for field, threshold := range policy.Accuracy {
		if !validJudgedQualityField(field) || threshold.MinJudgments < 1 || threshold.MinJudgments > 10000 ||
			threshold.MinCorrectRate == nil || !validQualityRate(*threshold.MinCorrectRate) {
			return QualityReport{}, ErrInvalid
		}
		scored := reference.Fields[field]
		result := qualityRate(scored.Correct, scored.Correct+scored.Missing+scored.Mismatch, threshold.MinJudgments, *threshold.MinCorrectRate)
		out.Accuracy[field] = result
		out.Passed = out.Passed && result.Passed
	}
	return out, nil
}

var judgedQualityFields = [...]string{
	"title", "nativeTitle", "mediaType", "status", "season", "seasonNumber", "partNumber",
	"releaseYear", "episodeCount", "genres", "studios", "themes", "demographics", "staffCredits", "characterCredits",
}

func validJudgedQualityField(field string) bool {
	if slices.Contains(judgedQualityFields[:], field) {
		return true
	}
	for _, track := range []string{"RAW", "SUB", "DUB", "UNKNOWN"} {
		if field == "scheduledAt."+track || field == "releasedAt."+track {
			return true
		}
	}
	return false
}

func qualityRate(numerator, denominator, minSamples int, threshold float64) QualityResult {
	result := QualityResult{Numerator: numerator, Denominator: denominator}
	if denominator > 0 {
		rate := float64(numerator) / float64(denominator)
		result.Rate = &rate
		result.Passed = denominator >= minSamples && rate >= threshold
	}
	return result
}

func validQualityRate(rate float64) bool {
	return !math.IsNaN(rate) && rate >= 0 && rate <= 1
}
