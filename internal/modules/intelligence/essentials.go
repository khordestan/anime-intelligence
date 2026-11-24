package intelligence

import (
	"errors"
	"fmt"
	"net/url"
	"slices"
	"sort"
	"strings"
)

const GlobalEssentialsAlgorithmVersion = "global-essentials-v1"

type EssentialsEvidenceKind string

const (
	JuriedAwardWin         EssentialsEvidenceKind = "JURIED_AWARD_WIN"
	InstitutionalSelection EssentialsEvidenceKind = "INSTITUTIONAL_SELECTION"
)

type EssentialsEvidence struct {
	Kind        EssentialsEvidenceKind `json:"kind"`
	PublisherID string                 `json:"publisherId"`
	SourceURL   string                 `json:"sourceUrl"`
}

type EssentialsCandidate struct {
	AnimeID  string               `json:"animeId"`
	Title    string               `json:"title"`
	Evidence []EssentialsEvidence `json:"evidence"`
}

type EssentialsMatch struct {
	AnimeID        string               `json:"animeId"`
	Title          string               `json:"title"`
	EvidenceKinds  int                  `json:"evidenceKinds"`
	PublisherCount int                  `json:"publisherCount"`
	Evidence       []EssentialsEvidence `json:"evidence"`
}

// RankGlobalEssentials accepts reviewed, title-level evidence. The caller
// supplies stable IDs for independent publishers and applies source policy.
func RankGlobalEssentials(candidates []EssentialsCandidate, limit int) ([]EssentialsMatch, error) {
	if limit < 1 || limit > 100 || len(candidates) > 2000 {
		return nil, errors.New("expected at most 2000 candidates and a limit between 1 and 100")
	}
	seen := make(map[string]bool, len(candidates))
	matches := make([]EssentialsMatch, 0)
	for index, candidate := range candidates {
		id := candidate.AnimeID
		if id == "" || strings.TrimSpace(id) != id || seen[id] ||
			strings.TrimSpace(candidate.Title) == "" || len(candidate.Evidence) > 20 {
			return nil, fmt.Errorf("invalid ID, title, or evidence count at candidate %d", index+1)
		}
		seen[id] = true
		if len(candidate.Evidence) == 0 {
			continue
		}
		kinds := make(map[EssentialsEvidenceKind]bool)
		publishers := make(map[string]bool)
		unique := make(map[EssentialsEvidence]bool)
		evidence := make([]EssentialsEvidence, 0, len(candidate.Evidence))
		for _, item := range candidate.Evidence {
			source, err := url.Parse(item.SourceURL)
			if err != nil || source.Scheme != "https" || source.Hostname() == "" || source.User != nil ||
				item.PublisherID == "" || strings.TrimSpace(item.PublisherID) != item.PublisherID ||
				(item.Kind != JuriedAwardWin && item.Kind != InstitutionalSelection) {
				return nil, fmt.Errorf("invalid evidence at candidate %d", index+1)
			}
			kinds[item.Kind] = true
			publishers[item.PublisherID] = true
			if !unique[item] {
				evidence = append(evidence, item)
				unique[item] = true
			}
		}
		sort.Slice(evidence, func(i, j int) bool {
			if evidence[i].Kind != evidence[j].Kind {
				return evidence[i].Kind < evidence[j].Kind
			}
			if evidence[i].PublisherID != evidence[j].PublisherID {
				return evidence[i].PublisherID < evidence[j].PublisherID
			}
			return evidence[i].SourceURL < evidence[j].SourceURL
		})
		matches = append(matches, EssentialsMatch{
			AnimeID: id, Title: candidate.Title, EvidenceKinds: len(kinds),
			PublisherCount: len(publishers), Evidence: evidence,
		})
	}
	sort.Slice(matches, func(i, j int) bool {
		if matches[i].EvidenceKinds != matches[j].EvidenceKinds {
			return matches[i].EvidenceKinds > matches[j].EvidenceKinds
		}
		if matches[i].PublisherCount != matches[j].PublisherCount {
			return matches[i].PublisherCount > matches[j].PublisherCount
		}
		return matches[i].AnimeID < matches[j].AnimeID
	})
	return matches[:min(limit, len(matches))], nil
}

type EssentialsEvaluation struct {
	Version    int                   `json:"version"`
	Candidates []EssentialsCandidate `json:"candidates"`
	Relevant   []string              `json:"relevant"`
}

type EssentialsCoverage struct {
	Candidates         int     `json:"candidates"`
	EligibleCandidates int     `json:"eligibleCandidates"`
	Relevant           int     `json:"relevant"`
	EligibleRelevant   int     `json:"eligibleRelevant"`
	RecallCeilingAtK   float64 `json:"recallCeilingAtK"`
}

type EssentialsReport struct {
	AlgorithmVersion string             `json:"algorithmVersion"`
	K                int                `json:"k"`
	Coverage         EssentialsCoverage `json:"coverage"`
	Ranking          []EssentialsMatch  `json:"ranking"`
	GlobalEssentials RankingMetrics     `json:"globalEssentials"`
	TitleBaseline    RankingMetrics     `json:"titleBaseline"`
}

func EvaluateGlobalEssentials(fixture EssentialsEvaluation, k int) (EssentialsReport, error) {
	if fixture.Version != 1 || len(fixture.Relevant) == 0 || len(fixture.Relevant) > len(fixture.Candidates) {
		return EssentialsReport{}, errors.New("expected a v1 fixture with held-out relevant IDs")
	}
	ranking, err := RankGlobalEssentials(fixture.Candidates, k)
	if err != nil {
		return EssentialsReport{}, err
	}
	coverage := EssentialsCoverage{Candidates: len(fixture.Candidates), Relevant: len(fixture.Relevant)}
	eligible := make(map[string]bool, len(fixture.Candidates))
	for _, item := range fixture.Candidates {
		supported := len(item.Evidence) > 0
		eligible[item.AnimeID] = supported
		if supported {
			coverage.EligibleCandidates++
		}
	}
	relevant := make(map[string]struct{}, len(fixture.Relevant))
	for _, id := range fixture.Relevant {
		supported, known := eligible[id]
		if _, duplicate := relevant[id]; duplicate || !known {
			return EssentialsReport{}, errors.New("relevant IDs must be unique candidates")
		}
		relevant[id] = struct{}{}
		if supported {
			coverage.EligibleRelevant++
		}
	}
	coverage.RecallCeilingAtK = float64(min(k, coverage.EligibleRelevant)) / float64(coverage.Relevant)
	ids := make([]string, 0, len(ranking))
	for _, item := range ranking {
		ids = append(ids, item.AnimeID)
	}
	baseline := slices.Clone(fixture.Candidates)
	sort.Slice(baseline, func(i, j int) bool {
		if baseline[i].Title == baseline[j].Title {
			return baseline[i].AnimeID < baseline[j].AnimeID
		}
		return baseline[i].Title < baseline[j].Title
	})
	baselineIDs := make([]string, 0, min(k, len(baseline)))
	for _, item := range baseline[:min(k, len(baseline))] {
		baselineIDs = append(baselineIDs, item.AnimeID)
	}
	return EssentialsReport{
		AlgorithmVersion: GlobalEssentialsAlgorithmVersion, K: k, Ranking: ranking, Coverage: coverage,
		GlobalEssentials: rankingMetrics(ids, relevant, k), TitleBaseline: rankingMetrics(baselineIDs, relevant, k),
	}, nil
}
