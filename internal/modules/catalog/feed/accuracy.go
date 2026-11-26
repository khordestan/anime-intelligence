package feed

import (
	"slices"
	"strconv"
	"strings"
	"time"
)

type JudgedField struct {
	Correct  int `json:"correct"`
	Missing  int `json:"missing"`
	Mismatch int `json:"mismatch"`
}

type FieldAccuracy struct {
	ReviewedTitles      int                    `json:"reviewedTitles"`
	MatchedTitles       int                    `json:"matchedTitles"`
	CandidateObservedAt time.Time              `json:"candidateObservedAt"`
	ReferenceObservedAt time.Time              `json:"referenceObservedAt"`
	Fields              map[string]JudgedField `json:"fields"`
}

// CompareReviewedFields scores known reference values at exact provider IDs.
// Unknown reference values leave the field unjudged. A missing candidate title
// contributes missing values for every field judged for that title.
func CompareReviewedFields(candidate, reference Snapshot, now time.Time) (FieldAccuracy, error) {
	if candidate.Provider != reference.Provider || len(reference.Titles) == 0 ||
		candidate.Validate(now) != nil || reference.Validate(now) != nil {
		return FieldAccuracy{}, ErrInvalid
	}
	result := FieldAccuracy{
		ReviewedTitles:      len(reference.Titles),
		CandidateObservedAt: candidate.ObservedAt,
		ReferenceObservedAt: reference.ObservedAt,
		Fields:              map[string]JudgedField{},
	}
	byID := make(map[string]Title, len(candidate.Titles))
	for _, title := range candidate.Titles {
		byID[title.ExternalID] = title
	}
	for _, title := range reference.Titles {
		actual, found := byID[title.ExternalID]
		if found {
			result.MatchedTitles++
		}
		actualFields := judgmentFields(actual)
		for key, expected := range judgmentFields(title) {
			if len(expected) == 0 {
				continue
			}
			count := result.Fields[key]
			switch values := actualFields[key]; {
			case len(values) == 0:
				count.Missing++
			case slices.Equal(values, expected):
				count.Correct++
			default:
				count.Mismatch++
			}
			result.Fields[key] = count
		}
	}
	return result, nil
}

func judgmentFields(title Title) map[string][]string {
	values := map[string][]string{
		"genres":           normalizedJudgmentLabels(title.Genres),
		"demographics":     normalizedJudgmentLabels(title.Demographics),
		"studios":          normalizedJudgmentLabels(title.Studios),
		"themes":           normalizedJudgmentLabels(title.Themes),
		"staffCredits":     normalizedJudgmentCredits(title.StaffCredits),
		"characterCredits": normalizedJudgmentCredits(title.CharacterCredits),
	}
	for key, value := range map[string]string{
		"title": title.Title, "nativeTitle": title.NativeTitle,
		"mediaType": title.MediaType, "status": title.Status, "season": title.Season,
	} {
		if value = strings.TrimSpace(value); value != "" && (value != "UNKNOWN" || (key != "mediaType" && key != "status")) {
			values[key] = []string{value}
		}
	}
	if title.SeasonNumber > 0 {
		values["seasonNumber"] = []string{strconv.Itoa(int(title.SeasonNumber))}
	}
	if title.PartNumber > 0 {
		values["partNumber"] = []string{strconv.Itoa(int(title.PartNumber))}
	}
	if title.ReleaseYear != 0 {
		values["releaseYear"] = []string{strconv.Itoa(title.ReleaseYear)}
	}
	if title.EpisodeCount != nil {
		values["episodeCount"] = []string{strconv.Itoa(*title.EpisodeCount)}
	}
	return values
}

func normalizedJudgmentLabels(labels []string) []string {
	values := make([]string, len(labels))
	for i, label := range labels {
		values[i] = strings.ToLower(strings.TrimSpace(label))
	}
	slices.Sort(values)
	return values
}

// Validation excludes NUL from both labels, so the separator preserves the pair.
func normalizedJudgmentCredits(credits []Credit) []string {
	values := make([]string, len(credits))
	for i, credit := range credits {
		values[i] = strings.ToLower(credit.Name) + "\x00" + strings.ToLower(credit.Role)
	}
	slices.Sort(values)
	return values
}
