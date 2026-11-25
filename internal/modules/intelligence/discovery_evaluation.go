package intelligence

import (
	"cmp"
	"errors"
	"fmt"
	"slices"
	"strings"
)

type DiscoveryEvaluationSuite struct {
	Version int                       `json:"version"`
	Cases   []DiscoveryEvaluationCase `json:"cases"`
}

type DiscoveryEvaluationCase struct {
	ID            string               `json:"id"`
	Feature       string               `json:"feature,omitempty"`
	KnownFeatures []string             `json:"knownFeatures"`
	Candidates    []DiscoveryCandidate `json:"candidates"`
	Relevant      []string             `json:"relevant"`
}

type DiscoveryCandidate struct {
	ID       string   `json:"id"`
	Features []string `json:"features"`
	Excluded bool     `json:"excluded,omitempty"`
}

type DiscoveryEvaluationResult struct {
	ID         string         `json:"id"`
	Coverage   RankingMetrics `json:"coverage"`
	Unfamiliar RankingMetrics `json:"unfamiliar"`
}

type DiscoveryEvaluationReport struct {
	K          int                         `json:"k"`
	Cases      []DiscoveryEvaluationResult `json:"cases"`
	Coverage   RankingMetrics              `json:"coverageMean"`
	Unfamiliar RankingMetrics              `json:"unfamiliarMean"`
}

// EvaluateDiscovery compares the runtime ordering rules on independently judged
// fixtures. The caller supplies latest eligible positive evidence for each title.
func EvaluateDiscovery(suite DiscoveryEvaluationSuite, k int) (DiscoveryEvaluationReport, error) {
	if suite.Version != 1 || len(suite.Cases) < 1 || len(suite.Cases) > 100 || k < 1 || k > 100 {
		return DiscoveryEvaluationReport{}, errors.New("expected a v1 suite with 1–100 cases and k between 1 and 100")
	}
	result := DiscoveryEvaluationReport{K: k}
	seen := make(map[string]bool, len(suite.Cases))
	for index, item := range suite.Cases {
		if seen[item.ID] || !validDiscoveryFixtureKey(item.ID) {
			return DiscoveryEvaluationReport{}, fmt.Errorf("invalid case identifier at case %d", index+1)
		}
		seen[item.ID] = true
		evaluation, err := evaluateDiscoveryCase(item, k)
		if err != nil {
			return DiscoveryEvaluationReport{}, fmt.Errorf("case %d: %w", index+1, err)
		}
		result.Cases = append(result.Cases, evaluation)
		for _, metric := range []struct {
			sum  *RankingMetrics
			part RankingMetrics
		}{{&result.Coverage, evaluation.Coverage}, {&result.Unfamiliar, evaluation.Unfamiliar}} {
			n := float64(len(suite.Cases))
			metric.sum.NDCG += metric.part.NDCG / n
			metric.sum.FillRate += metric.part.FillRate / n
			metric.sum.Recall += metric.part.Recall / n
			metric.sum.ReciprocalRank += metric.part.ReciprocalRank / n
		}
	}
	return result, nil
}

type discoveryRank struct {
	id    string
	count int
}

func evaluateDiscoveryCase(item DiscoveryEvaluationCase, k int) (DiscoveryEvaluationResult, error) {
	if len(item.Candidates) > 2000 || len(item.KnownFeatures) > 256 ||
		(item.Feature != "" && !validDiscoveryFixtureKey(item.Feature)) {
		return DiscoveryEvaluationResult{}, errors.New("invalid candidates or feature filter")
	}
	known := make(map[string]bool, len(item.KnownFeatures))
	for _, feature := range item.KnownFeatures {
		if !validDiscoveryFixtureKey(feature) || known[feature] {
			return DiscoveryEvaluationResult{}, errors.New("invalid or repeated known feature")
		}
		known[feature] = true
	}
	seen := make(map[string]bool, len(item.Candidates))
	eligible := make(map[string]bool, len(item.Candidates))
	var coverage, unfamiliar []discoveryRank
	for _, candidate := range item.Candidates {
		if !validDiscoveryFixtureKey(candidate.ID) || seen[candidate.ID] || len(candidate.Features) > 256 {
			return DiscoveryEvaluationResult{}, errors.New("invalid or repeated candidate")
		}
		seen[candidate.ID] = true
		features := make(map[string]bool, len(candidate.Features))
		novel := 0
		for _, feature := range candidate.Features {
			if !validDiscoveryFixtureKey(feature) || features[feature] {
				return DiscoveryEvaluationResult{}, errors.New("invalid or repeated candidate feature")
			}
			features[feature] = true
			if !known[feature] {
				novel++
			}
		}
		if candidate.Excluded || len(features) == 0 || (item.Feature != "" && !features[item.Feature]) {
			continue
		}
		eligible[candidate.ID] = true
		coverage = append(coverage, discoveryRank{candidate.ID, len(features)})
		if novel > 0 {
			unfamiliar = append(unfamiliar, discoveryRank{candidate.ID, novel})
		}
	}
	relevant := make(map[string]struct{}, len(item.Relevant))
	for _, id := range item.Relevant {
		_, duplicate := relevant[id]
		if !eligible[id] || duplicate {
			return DiscoveryEvaluationResult{}, errors.New("relevant judgments must identify distinct eligible candidates")
		}
		relevant[id] = struct{}{}
	}
	if len(relevant) == 0 {
		return DiscoveryEvaluationResult{}, errors.New("at least one relevant judgment is required")
	}
	return DiscoveryEvaluationResult{ID: item.ID,
		Coverage:   rankingMetrics(discoveryRankedIDs(coverage, k), relevant, k),
		Unfamiliar: rankingMetrics(discoveryRankedIDs(unfamiliar, k), relevant, k)}, nil
}

func discoveryRankedIDs(ranking []discoveryRank, k int) []string {
	slices.SortFunc(ranking, func(a, b discoveryRank) int {
		if a.count != b.count {
			return cmp.Compare(b.count, a.count)
		}
		return cmp.Compare(a.id, b.id)
	})
	ranking = ranking[:min(len(ranking), k)]
	ids := make([]string, len(ranking))
	for i, item := range ranking {
		ids[i] = item.id
	}
	return ids
}

func validDiscoveryFixtureKey(value string) bool {
	return len(value) > 0 && len(value) <= 128 && strings.TrimSpace(value) == value
}
