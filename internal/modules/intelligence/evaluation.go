package intelligence

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
)

type EvaluationSuite struct {
	Version int `json:"version"`
	Cases []EvaluationCase `json:"cases"`
}

type EvaluationCase struct {
	ID string `json:"id"`
	Rated []RatedAnime `json:"rated"`
	Preferences map[string]Preference `json:"preferences"`
	Candidates []Candidate `json:"candidates"`
	Excluded []string `json:"excluded"`
	Relevant []string `json:"relevant"`
}

type RankingMetrics struct {
	NDCG float64 `json:"ndcg"`
	FillRate float64 `json:"fillRate"`
	Recall float64 `json:"recall"`
	ReciprocalRank float64 `json:"reciprocalRank"`
}

type CaseEvaluation struct {
	ID string `json:"id"`
	PersonalMatch RankingMetrics `json:"personalMatch"`
	TitleBaseline RankingMetrics `json:"titleBaseline"`
}

type EvaluationReport struct {
	AlgorithmVersion string `json:"algorithmVersion"`
	K int `json:"k"`
	Cases []CaseEvaluation `json:"cases"`
	PersonalMatch RankingMetrics `json:"personalMatchMean"`
	TitleBaseline RankingMetrics `json:"titleBaselineMean"`
}

// EvaluatePersonalMatch evaluates explicit positive judgments at k. Both
// rankings receive the same candidate set and exclude already-rated titles.
func EvaluatePersonalMatch(suite EvaluationSuite, k int) (EvaluationReport, error) {
	if suite.Version != 1 || len(suite.Cases) == 0 || len(suite.Cases) > 100 || k < 1 || k > 100 {
		return EvaluationReport{}, errors.New("expected a v1 evaluation suite with 1–100 cases and k between 1 and 100")
	}
	report := EvaluationReport{AlgorithmVersion: PersonalMatchAlgorithmVersion, K: k}
	caseIDs := make(map[string]struct{}, len(suite.Cases))
	for index, item := range suite.Cases {
		_, duplicate := caseIDs[item.ID]
		if duplicate || strings.TrimSpace(item.ID) == "" || len(item.Candidates) > 500 || len(item.Rated) > 1000 {
			return EvaluationReport{}, fmt.Errorf("invalid case ID or fixture size at case %d", index+1)
		}
		caseIDs[item.ID] = struct{}{}
		result, err := evaluateCase(item, k)
		if err != nil {
			return EvaluationReport{}, fmt.Errorf("case %d: %w", index+1, err)
		}
		report.Cases = append(report.Cases, result)
		report.PersonalMatch.NDCG += result.PersonalMatch.NDCG
		report.PersonalMatch.FillRate += result.PersonalMatch.FillRate
		report.TitleBaseline.NDCG += result.TitleBaseline.NDCG
		report.TitleBaseline.FillRate += result.TitleBaseline.FillRate
		report.PersonalMatch.Recall += result.PersonalMatch.Recall
		report.PersonalMatch.ReciprocalRank += result.PersonalMatch.ReciprocalRank
		report.TitleBaseline.Recall += result.TitleBaseline.Recall
		report.TitleBaseline.ReciprocalRank += result.TitleBaseline.ReciprocalRank
	}
	n := float64(len(report.Cases))
	report.PersonalMatch.NDCG /= n
	report.PersonalMatch.FillRate /= n
	report.TitleBaseline.NDCG /= n
	report.TitleBaseline.FillRate /= n
	report.PersonalMatch.Recall /= n
	report.PersonalMatch.ReciprocalRank /= n
	report.TitleBaseline.Recall /= n
	report.TitleBaseline.ReciprocalRank /= n
	return report, nil
}

func evaluateCase(item EvaluationCase, k int) (CaseEvaluation, error) {
	excluded := make(map[string]struct{}, len(item.Excluded)+len(item.Rated))
	for _, id := range item.Excluded {
		excluded[id] = struct{}{}
	}
	for _, rated := range item.Rated {
		if rated.Score < 1 || rated.Score > 5 || strings.TrimSpace(rated.AnimeID) == "" {
			return CaseEvaluation{}, errors.New("invalid rated title")
		}
		excluded[rated.AnimeID] = struct{}{}
	}
	seen := make(map[string]struct{}, len(item.Candidates))
	eligible := make([]Candidate, 0, len(item.Candidates))
	for _, candidate := range item.Candidates {
		_, duplicate := seen[candidate.AnimeID]
		if duplicate || strings.TrimSpace(candidate.AnimeID) != candidate.AnimeID ||
			candidate.AnimeID == "" || strings.TrimSpace(candidate.Title) == "" {
			return CaseEvaluation{}, errors.New("candidate IDs must be nonempty and unique and titles must be nonempty")
		}
		seen[candidate.AnimeID] = struct{}{}
		if _, skip := excluded[candidate.AnimeID]; !skip {
			eligible = append(eligible, candidate)
		}
	}
	relevant := make(map[string]struct{}, len(item.Relevant))
	for _, id := range item.Relevant {
		_, known := seen[id]
		_, skip := excluded[id]
		_, duplicate := relevant[id]
		if !known || skip || duplicate {
			return CaseEvaluation{}, errors.New("relevant IDs must be unique eligible candidates")
		}
		relevant[id] = struct{}{}
	}
	if len(relevant) == 0 {
		return CaseEvaluation{}, errors.New("at least one relevant candidate is required")
	}
	profile := BuildTasteProfile(item.Rated, item.Preferences)
	ranked := RankPersonalMatches(profile, eligible, nil, Options{Limit: k})
	personalIDs := make([]string, 0, len(ranked))
	for _, match := range ranked {
		personalIDs = append(personalIDs, match.AnimeID)
	}
	sort.Slice(eligible, func(i, j int) bool {
		if eligible[i].Title == eligible[j].Title {
			return eligible[i].AnimeID < eligible[j].AnimeID
		}
		return eligible[i].Title < eligible[j].Title
	})
	baselineIDs := make([]string, 0, min(k, len(eligible)))
	for _, candidate := range eligible[:min(k, len(eligible))] {
		baselineIDs = append(baselineIDs, candidate.AnimeID)
	}
	return CaseEvaluation{
		ID: item.ID,
		PersonalMatch: rankingMetrics(personalIDs, relevant, k),
		TitleBaseline: rankingMetrics(baselineIDs, relevant, k),
	}, nil
}

func rankingMetrics(ids []string, relevant map[string]struct{}, k int) RankingMetrics {
	result := RankingMetrics{FillRate: float64(len(ids)) / float64(k)}
	var gain, idealGain float64
	for i := range min(k, len(relevant)) {
		idealGain += 1 / math.Log2(float64(i+2))
	}
	var hits int
	for i, id := range ids {
		if _, ok := relevant[id]; !ok {
			continue
		}
		hits++
		gain += 1 / math.Log2(float64(i+2))
		if hits == 1 {
			result.ReciprocalRank = 1 / float64(i+1)
		}
	}
	result.Recall = float64(hits) / float64(len(relevant))
	result.NDCG = gain / idealGain
	return result
}
