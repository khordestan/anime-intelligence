package intelligence

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
)

type EvaluationSuite struct {
	CandidateLimit int              `json:"candidateLimit,omitempty"`
	Version        int              `json:"version"`
	Cases          []EvaluationCase `json:"cases"`
}

type EvaluationCase struct {
	ID          string                    `json:"id"`
	Rated       []RatedAnime              `json:"rated"`
	Preferences map[string]Preference     `json:"preferences"`
	Candidates  []Candidate               `json:"candidates"`
	Excluded    []string                  `json:"excluded"`
	Relevant    []string                  `json:"relevant"`
	Feedback    []EvaluationTitleFeedback `json:"feedback,omitempty"`
}

type EvaluationTitleFeedback struct {
	AnimeID   string         `json:"animeId"`
	Direction int            `json:"direction"`
	Features  []FeatureValue `json:"features"`
}

type RankingMetrics struct {
	NDCG           float64 `json:"ndcg"`
	FillRate       float64 `json:"fillRate"`
	Recall         float64 `json:"recall"`
	ReciprocalRank float64 `json:"reciprocalRank"`
}

type RetrievalMetrics struct {
	CandidateCount int     `json:"candidateCount"`
	Recall         float64 `json:"recall"`
}

type CaseEvaluation struct {
	Retrieval     *RetrievalMetrics `json:"retrieval,omitempty"`
	ID            string            `json:"id"`
	PersonalMatch RankingMetrics    `json:"personalMatch"`
	TitleBaseline RankingMetrics    `json:"titleBaseline"`
	WithFeedback  *FeedbackRanking  `json:"withFeedback,omitempty"`
}

type FeedbackRanking struct {
	Ranking   RankingMetrics    `json:"ranking"`
	Retrieval *RetrievalMetrics `json:"retrieval,omitempty"`
}

type FeedbackComparison struct {
	CaseCount       int            `json:"caseCount"`
	WithoutFeedback RankingMetrics `json:"withoutFeedbackMean"`
	WithFeedback    RankingMetrics `json:"withFeedbackMean"`
}

func (comparison *FeedbackComparison) add(result CaseEvaluation) {
	comparison.CaseCount++
	comparison.WithoutFeedback.add(result.PersonalMatch)
	comparison.WithFeedback.add(result.WithFeedback.Ranking)
}

func (comparison FeedbackComparison) mean() FeedbackComparison {
	n := float64(comparison.CaseCount)
	comparison.WithoutFeedback.divide(n)
	comparison.WithFeedback.divide(n)
	return comparison
}

type EvaluationReport struct {
	CandidateLimit      int                           `json:"candidateLimit,omitempty"`
	MeanRetrievalRecall *float64                      `json:"meanRetrievalRecall,omitempty"`
	AlgorithmVersion    string                        `json:"algorithmVersion"`
	K                   int                           `json:"k"`
	Cases               []CaseEvaluation              `json:"cases"`
	PersonalMatch       RankingMetrics                `json:"personalMatchMean"`
	TitleBaseline       RankingMetrics                `json:"titleBaselineMean"`
	FeedbackComparison  *FeedbackComparison           `json:"feedbackComparison,omitempty"`
	FeedbackByDirection map[string]FeedbackComparison `json:"feedbackByDirection,omitempty"`
}

// EvaluatePersonalMatch evaluates explicit positive judgments at k. Both
// rankings exclude already-rated titles. An optional stage caps Personal Match
// candidates before ranking to measure the recall cost of retrieval.
func EvaluatePersonalMatch(suite EvaluationSuite, k int) (EvaluationReport, error) {
	if suite.Version != 1 || len(suite.Cases) == 0 || len(suite.Cases) > 100 || k < 1 || k > 100 {
		return EvaluationReport{}, errors.New("expected a v1 evaluation suite with 1–100 cases and k between 1 and 100")
	}
	if suite.CandidateLimit < 0 || suite.CandidateLimit > 500 {
		return EvaluationReport{}, errors.New("candidateLimit must be between 0 and 500")
	}
	maxCandidates := 500
	if suite.CandidateLimit > 0 {
		maxCandidates = 2000
	}
	report := EvaluationReport{AlgorithmVersion: PersonalMatchAlgorithmVersion, K: k, CandidateLimit: suite.CandidateLimit}
	var retrievalRecall float64
	var feedback FeedbackComparison
	feedbackByDirection := make(map[string]FeedbackComparison, 3)
	caseIDs := make(map[string]struct{}, len(suite.Cases))
	for index, item := range suite.Cases {
		_, duplicate := caseIDs[item.ID]
		if duplicate || strings.TrimSpace(item.ID) == "" || len(item.Candidates) > maxCandidates || len(item.Rated) > 1000 {
			return EvaluationReport{}, fmt.Errorf("invalid case ID or fixture size at case %d", index+1)
		}
		caseIDs[item.ID] = struct{}{}
		result, err := evaluateCase(item, k, suite.CandidateLimit)
		if err != nil {
			return EvaluationReport{}, fmt.Errorf("case %d: %w", index+1, err)
		}
		report.Cases = append(report.Cases, result)
		if result.Retrieval != nil {
			retrievalRecall += result.Retrieval.Recall
		}
		report.PersonalMatch.add(result.PersonalMatch)
		report.TitleBaseline.add(result.TitleBaseline)
		if result.WithFeedback != nil {
			feedback.add(result)
			direction := feedbackDirection(item.Feedback)
			comparison := feedbackByDirection[direction]
			comparison.add(result)
			feedbackByDirection[direction] = comparison
		}
	}
	n := float64(len(report.Cases))
	if suite.CandidateLimit > 0 {
		mean := retrievalRecall / n
		report.MeanRetrievalRecall = &mean
	}
	report.PersonalMatch.divide(n)
	report.TitleBaseline.divide(n)
	if feedback.CaseCount > 0 {
		mean := feedback.mean()
		report.FeedbackComparison = &mean
		for direction, comparison := range feedbackByDirection {
			feedbackByDirection[direction] = comparison.mean()
		}
		report.FeedbackByDirection = feedbackByDirection
	}
	return report, nil
}

// The fixture has at least one validated reaction. Only fixed group names are reported.
func feedbackDirection(feedback []EvaluationTitleFeedback) string {
	direction := feedback[0].Direction
	for _, item := range feedback[1:] {
		if item.Direction != direction {
			return "mixed"
		}
	}
	if direction > 0 {
		return "positiveOnly"
	}
	return "negativeOnly"
}

func evaluateCase(item EvaluationCase, k, candidateLimit int) (CaseEvaluation, error) {
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
	personal := evaluateProfile(profile, eligible, relevant, k, candidateLimit)
	var withFeedback *FeedbackRanking
	if len(item.Feedback) > 0 {
		reactions, err := evaluationReactions(item.Feedback, seen)
		if err != nil {
			return CaseEvaluation{}, err
		}
		profile.ApplyTitleFeedback(reactions)
		paired := evaluateProfile(profile, eligible, relevant, k, candidateLimit)
		withFeedback = &paired
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
		ID:            item.ID,
		Retrieval:     personal.Retrieval,
		PersonalMatch: personal.Ranking,
		WithFeedback:  withFeedback,
		TitleBaseline: rankingMetrics(baselineIDs, relevant, k),
	}, nil
}

func evaluationReactions(feedback []EvaluationTitleFeedback, candidates map[string]struct{}) ([]TitleFeedback, error) {
	if len(feedback) > 100 {
		return nil, errors.New("at most 100 training title reactions are allowed")
	}
	reactions := make([]TitleFeedback, 0, len(feedback))
	seen := make(map[string]bool, len(feedback))
	for _, item := range feedback {
		_, candidate := candidates[item.AnimeID]
		if item.AnimeID == "" || strings.TrimSpace(item.AnimeID) != item.AnimeID || seen[item.AnimeID] || candidate ||
			(item.Direction != -1 && item.Direction != 1) || len(item.Features) > 256 {
			return nil, errors.New("training reactions require unique titles outside the candidate set, bounded features and a direction of -1 or 1")
		}
		for _, feature := range item.Features {
			if feature.Key == "" || !validUnit(feature.Value) || !validUnit(feature.Confidence) {
				return nil, errors.New("invalid training reaction feature")
			}
		}
		seen[item.AnimeID] = true
		reactions = append(reactions, TitleFeedback{Direction: item.Direction, Features: item.Features})
	}
	return reactions, nil
}

func evaluateProfile(profile TasteProfile, eligible []Candidate, relevant map[string]struct{}, k, candidateLimit int) FeedbackRanking {
	rankingCandidates := eligible
	var retrieval *RetrievalMetrics
	if candidateLimit > 0 {
		rankingCandidates = retrieveEvaluationCandidates(profile, eligible, candidateLimit)
		var hits int
		for _, candidate := range rankingCandidates {
			if _, ok := relevant[candidate.AnimeID]; ok {
				hits++
			}
		}
		retrieval = &RetrievalMetrics{CandidateCount: len(rankingCandidates), Recall: float64(hits) / float64(len(relevant))}
	}
	ranked := RankPersonalMatches(profile, rankingCandidates, nil, Options{Limit: k})
	personalIDs := make([]string, 0, len(ranked))
	for _, match := range ranked {
		personalIDs = append(personalIDs, match.AnimeID)
	}
	return FeedbackRanking{Ranking: rankingMetrics(personalIDs, relevant, k), Retrieval: retrieval}
}

func (m *RankingMetrics) add(other RankingMetrics) {
	m.NDCG += other.NDCG
	m.FillRate += other.FillRate
	m.Recall += other.Recall
	m.ReciprocalRank += other.ReciprocalRank
}

func (m *RankingMetrics) divide(by float64) {
	m.NDCG /= by
	m.FillRate /= by
	m.Recall /= by
	m.ReciprocalRank /= by
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

// Fixtures supply current eligible evidence. This mirrors the PostgreSQL
// overlap order without reproducing the database's evidence eligibility checks.
func retrieveEvaluationCandidates(profile TasteProfile, eligible []Candidate, limit int) []Candidate {
	keys := make(map[string]bool)
	for _, key := range PersonalMatchFeatureKeys(profile) {
		keys[key] = true
	}
	type supportedCandidate struct {
		candidate Candidate
		overlap   int
	}
	supported := make([]supportedCandidate, 0, len(eligible))
	for _, candidate := range eligible {
		seen := make(map[string]bool)
		for _, feature := range candidate.Features {
			if keys[feature.Key] && validUnit(feature.Value) && validUnit(feature.Confidence) && feature.Confidence > 0 {
				seen[feature.Key] = true
			}
		}
		if len(seen) > 0 {
			supported = append(supported, supportedCandidate{candidate: candidate, overlap: len(seen)})
		}
	}
	sort.Slice(supported, func(i, j int) bool {
		if supported[i].overlap == supported[j].overlap {
			return supported[i].candidate.AnimeID < supported[j].candidate.AnimeID
		}
		return supported[i].overlap > supported[j].overlap
	})
	count := min(limit, len(supported))
	result := make([]Candidate, 0, count)
	for _, item := range supported[:count] {
		result = append(result, item.candidate)
	}
	return result
}
