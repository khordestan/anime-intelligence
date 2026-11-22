package intelligence

import (
	"math"
	"sort"
	"strings"
)

const PersonalMatchAlgorithmVersion = "personal-match-v1"

type FeatureValue struct {
	Key        string
	Value      float64
	Confidence float64
}

type RatedAnime struct {
	AnimeID string
	Score   int
	Features []FeatureValue
}

type Preference struct {
	Weight     float64
	Confidence float64
}

type TasteProfile struct {
	Explicit map[string]Preference
	Inferred map[string]Preference
	InferredRatingCounts map[string]int
}

type Candidate struct {
	AnimeID string
	Title   string
	Features []FeatureValue
}

type MatchEvidence struct {
	FeatureKey string  `json:"featureKey"`
	Direction  string  `json:"direction"`
	Strength   float64 `json:"strength"`
	Impact     float64 `json:"impact"`
}

type Recommendation struct {
	AnimeID          string          `json:"animeId"`
	Title            string          `json:"title"`
	Score            float64         `json:"score"`
	AlgorithmVersion string          `json:"algorithmVersion"`
	Evidence         []MatchEvidence `json:"evidence"`
}

type Options struct {
	Limit int
	// Explicit preferences dominate inferred preferences when both exist.
	ExplicitWeight float64
	InferredWeight float64
}

func BuildTasteProfile(rated []RatedAnime, explicit map[string]Preference) TasteProfile {
	profile := TasteProfile{
		Explicit: make(map[string]Preference, len(explicit)),
		Inferred: make(map[string]Preference),
		InferredRatingCounts: make(map[string]int),
	}
	for key, preference := range explicit {
		key = strings.TrimSpace(key)
		if key == "" || !validUnit(preference.Confidence) || !validSigned(preference.Weight) {
			continue
		}
		profile.Explicit[key] = preference
	}

	type accumulator struct {
		sum       float64
		magnitude float64
		ratingCount int
		lastRating int
	}
	acc := map[string]accumulator{}
	for index, item := range rated {
		if item.Score < 1 || item.Score > 5 {
			continue
		}
		signal := float64(item.Score-3) / 2
		if signal == 0 {
			continue
		}
		for _, feature := range item.Features {
			key := strings.TrimSpace(feature.Key)
			if key == "" || !validUnit(feature.Value) || !validUnit(feature.Confidence) || feature.Confidence == 0 {
				continue
			}
			current := acc[key]
			current.sum += signal * feature.Value * feature.Confidence
			current.magnitude += math.Abs(signal) * feature.Confidence
			if current.lastRating != index+1 {
				current.ratingCount++
				current.lastRating = index+1
			}
			acc[key] = current
		}
	}
	for key, current := range acc {
		if current.magnitude == 0 {
			continue
		}
		weight := clamp(current.sum/current.magnitude, -1, 1)
		if math.Abs(weight) < 0.05 {
			continue
		}
		profile.InferredRatingCounts[key] = current.ratingCount
		profile.Inferred[key] = Preference{Weight: weight, Confidence: clamp(current.magnitude/float64(maxInt(1, len(rated))), 0, 1)}
	}
	return profile
}

func RankPersonalMatches(profile TasteProfile, candidates []Candidate, excluded map[string]struct{}, options Options) []Recommendation {
	if options.Limit <= 0 {
		options.Limit = 20
	}
	if options.Limit > 100 {
		options.Limit = 100
	}
	options = preferenceWeights(profile, options)

	seen := make(map[string]struct{}, len(candidates))
	result := make([]Recommendation, 0, len(candidates))
	for _, candidate := range candidates {
		id := strings.TrimSpace(candidate.AnimeID)
		if id == "" || strings.TrimSpace(candidate.Title) == "" {
			continue
		}
		if _, ok := excluded[id]; ok {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}

		numerator, denominator := 0.0, 0.0
		evidence := make([]MatchEvidence, 0)
		for _, feature := range candidate.Features {
			key := strings.TrimSpace(feature.Key)
			if key == "" || !validUnit(feature.Value) || !validUnit(feature.Confidence) || feature.Confidence == 0 {
				continue
			}
			preference, ok := combinedPreference(profile, key, options)
			if !ok || preference.Weight == 0 || preference.Confidence == 0 {
				continue
			}
			reliability := feature.Confidence * preference.Confidence
			impact := preference.Weight * feature.Value * reliability
			numerator += impact
			denominator += math.Abs(preference.Weight) * reliability
			direction := "neutral"
			if impact > 0 {
				direction = "match"
			} else if impact < 0 {
				direction = "mismatch"
			}
			evidence = append(evidence, MatchEvidence{FeatureKey: key, Direction: direction, Strength: feature.Value, Impact: impact})
		}
		if denominator == 0 {
			continue
		}
		score := clamp(50+50*(numerator/denominator), 0, 100)
		sort.Slice(evidence, func(i, j int) bool {
			if math.Abs(evidence[i].Impact) == math.Abs(evidence[j].Impact) { return evidence[i].FeatureKey < evidence[j].FeatureKey }
			return math.Abs(evidence[i].Impact) > math.Abs(evidence[j].Impact)
		})
		if len(evidence) > 4 {
			evidence = evidence[:4]
		}
		result = append(result, Recommendation{
			AnimeID: id, Title: candidate.Title, Score: math.Round(score*100) / 100,
			AlgorithmVersion: PersonalMatchAlgorithmVersion, Evidence: evidence,
		})
	}
	sort.SliceStable(result, func(i, j int) bool {
		if result[i].Score == result[j].Score {
			if result[i].Title == result[j].Title { return result[i].AnimeID < result[j].AnimeID }
			return result[i].Title < result[j].Title
		}
		return result[i].Score > result[j].Score
	})
	if len(result) > options.Limit {
		result = result[:options.Limit]
	}
	return result
}

// PersonalMatchFeatureKeys returns features that can contribute to ranking with the default weights.
func PersonalMatchFeatureKeys(profile TasteProfile) []string {
	options := preferenceWeights(profile, Options{})
	keys := make([]string, 0, len(profile.Explicit)+len(profile.Inferred))
	for key := range profile.Explicit {
		preference, _ := combinedPreference(profile, key, options)
		if preference.Weight != 0 && preference.Confidence > 0 {
			keys = append(keys, key)
		}
	}
	for key, preference := range profile.Inferred {
		if _, declared := profile.Explicit[key]; !declared && preference.Weight != 0 && preference.Confidence > 0 {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	return keys
}

func preferenceWeights(profile TasteProfile, options Options) Options {
	if options.ExplicitWeight <= 0 || options.InferredWeight < 0 {
		options.ExplicitWeight, options.InferredWeight = 0.7, 0.3
	}
	if len(profile.Explicit) == 0 {
		options.ExplicitWeight, options.InferredWeight = 0, 1
	}
	if len(profile.Inferred) == 0 {
		options.ExplicitWeight, options.InferredWeight = 1, 0
	}
	return options
}

func combinedPreference(profile TasteProfile, key string, options Options) (Preference, bool) {
	explicit, hasExplicit := profile.Explicit[key]
	inferred, hasInferred := profile.Inferred[key]
	if !hasExplicit && !hasInferred {
		return Preference{}, false
	}
	if hasExplicit && !hasInferred {
		return explicit, true
	}
	if hasInferred && !hasExplicit {
		return inferred, true
	}
	weight := explicit.Weight*options.ExplicitWeight + inferred.Weight*options.InferredWeight
	confidence := explicit.Confidence*options.ExplicitWeight + inferred.Confidence*options.InferredWeight
	return Preference{Weight: clamp(weight, -1, 1), Confidence: clamp(confidence, 0, 1)}, true
}

func validUnit(value float64) bool { return !math.IsNaN(value) && !math.IsInf(value, 0) && value >= 0 && value <= 1 }
func validSigned(value float64) bool { return !math.IsNaN(value) && !math.IsInf(value, 0) && value >= -1 && value <= 1 }
func clamp(value, min, max float64) float64 {
	if value < min { return min }
	if value > max { return max }
	return value
}
func maxInt(a, b int) int { if a > b { return a }; return b }
