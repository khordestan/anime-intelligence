package catalog

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"
)

type SelectorEvaluationSuite struct {
	Version int                `json:"version"`
	Cases   []SelectorJudgment `json:"cases"`
}

type SelectorJudgment struct {
	ID             string    `json:"id"`
	Question       string    `json:"question"`
	ExpectedFields []string  `json:"expectedFields"`
	ModelFields    *[]string `json:"modelFields,omitempty"`
}

type SelectorCaseResult struct {
	ID            string `json:"id"`
	Deterministic bool   `json:"deterministicExact"`
	Model         *bool  `json:"modelExact,omitempty"`
	InvalidModel  bool   `json:"invalidModel,omitempty"`
}

type SelectorSummary struct {
	Cases         int     `json:"cases"`
	Exact         int     `json:"exact"`
	Abstained     int     `json:"abstained"`
	Invalid       int     `json:"invalid"`
	MeanPrecision float64 `json:"meanPrecision"`
	MeanRecall    float64 `json:"meanRecall"`
}

type SelectorEvaluationReport struct {
	Deterministic SelectorSummary      `json:"deterministic"`
	Model         *SelectorSummary     `json:"model,omitempty"`
	Cases         []SelectorCaseResult `json:"cases"`
}

// EvaluateTopicSelector compares deterministic topics and optional recorded model
// output with reviewed field sets. It never contacts a model provider.
func EvaluateTopicSelector(suite SelectorEvaluationSuite) (SelectorEvaluationReport, error) {
	if suite.Version != 1 || len(suite.Cases) == 0 || len(suite.Cases) > 1000 {
		return SelectorEvaluationReport{}, errors.New("expected a v1 selector suite with 1–1000 cases")
	}
	allowed := allAnalysisFields()
	seen := map[string]bool{}
	var report SelectorEvaluationReport
	for i, item := range suite.Cases {
		if strings.TrimSpace(item.ID) == "" || len(item.ID) > 128 || seen[item.ID] ||
			strings.TrimSpace(item.Question) == "" || len(item.Question) > 400 || !utf8.ValidString(item.Question) ||
			item.ExpectedFields == nil {
			return SelectorEvaluationReport{}, fmt.Errorf("invalid judgment at case %d", i+1)
		}
		seen[item.ID] = true
		expected, matched, valid := validateSelectedFields(allowed, item.ExpectedFields)
		if !valid {
			return SelectorEvaluationReport{}, fmt.Errorf("unknown or duplicate expected field at case %d", i+1)
		}
		if !matched {
			expected = nil
		}
		deterministic, matched := selectAnalysisFields(item.Question)
		if !matched {
			deterministic = nil
		}
		result := SelectorCaseResult{ID: item.ID, Deterministic: report.Deterministic.add(expected, deterministic, true)}
		if item.ModelFields != nil {
			if report.Model == nil {
				report.Model = &SelectorSummary{}
			}
			got, matched, valid := validateSelectedFields(allowed, *item.ModelFields)
			if !matched {
				got = nil
			}
			exact := report.Model.add(expected, got, valid)
			result.Model = &exact
			result.InvalidModel = !valid
		}
		report.Cases = append(report.Cases, result)
	}
	report.Deterministic.finish()
	if report.Model != nil {
		report.Model.finish()
	}
	return report, nil
}

func (s *SelectorSummary) add(expected, selected []string, valid bool) bool {
	s.Cases++
	if !valid {
		s.Invalid++
		return false
	}
	if len(selected) == 0 {
		s.Abstained++
	}
	exact := slices.Equal(expected, selected)
	if exact {
		s.Exact++
		s.MeanPrecision++
		s.MeanRecall++
		return true
	}
	overlap := 0
	for _, field := range selected {
		if slices.Contains(expected, field) {
			overlap++
		}
	}
	if len(selected) > 0 {
		s.MeanPrecision += float64(overlap) / float64(len(selected))
	}
	if len(expected) > 0 {
		s.MeanRecall += float64(overlap) / float64(len(expected))
	}
	return false
}

func (s *SelectorSummary) finish() {
	s.MeanPrecision /= float64(s.Cases)
	s.MeanRecall /= float64(s.Cases)
}
