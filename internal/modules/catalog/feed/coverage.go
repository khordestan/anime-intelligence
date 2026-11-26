package feed

import "strings"

type ScheduleCoverage struct {
	ScheduledRows int `json:"scheduledRows"`
	ReleasedRows  int `json:"releasedRows"`
}

// SnapshotCoverage describes fields present in one validated snapshot. Counts
// cover only its supplied titles and schedule rows.
type SnapshotCoverage struct {
	Titles int                         `json:"titles"`
	Fields map[string]int              `json:"titlesWithField"`
	Tracks map[string]ScheduleCoverage `json:"rowsByTrack"`
}

func (s Snapshot) Coverage() SnapshotCoverage {
	out := SnapshotCoverage{
		Titles: len(s.Titles),
		Fields: map[string]int{
			"nativeTitle": 0, "synopsis": 0, "mediaType": 0, "status": 0,
			"releaseYear": 0, "season": 0, "seasonNumber": 0, "partNumber": 0, "episodeCount": 0, "synonyms": 0,
			"genres": 0, "tags": 0, "themes": 0, "demographics": 0,
			"studios": 0, "relations": 0, "staffCredits": 0, "characterCredits": 0,
		},
		Tracks: map[string]ScheduleCoverage{"RAW": {}, "SUB": {}, "DUB": {}, "UNKNOWN": {}},
	}
	for _, title := range s.Titles {
		for key, present := range map[string]bool{
			"nativeTitle":      strings.TrimSpace(title.NativeTitle) != "",
			"synopsis":         strings.TrimSpace(title.Synopsis) != "",
			"mediaType":        title.MediaType != "UNKNOWN",
			"status":           title.Status != "UNKNOWN",
			"releaseYear":      title.ReleaseYear != 0,
			"season":           title.Season != "",
			"seasonNumber":     title.SeasonNumber > 0,
			"partNumber":       title.PartNumber > 0,
			"episodeCount":     title.EpisodeCount != nil,
			"synonyms":         len(title.Synonyms) > 0,
			"genres":           len(title.Genres) > 0,
			"tags":             len(title.Tags) > 0,
			"themes":           len(title.Themes) > 0,
			"demographics":     len(title.Demographics) > 0,
			"studios":          len(title.Studios) > 0,
			"relations":        len(title.Relations) > 0,
			"staffCredits":     len(title.StaffCredits) > 0,
			"characterCredits": len(title.CharacterCredits) > 0,
		} {
			if present {
				out.Fields[key]++
			}
		}
		for _, episode := range title.Episodes {
			counts := out.Tracks[episode.Track]
			if episode.ScheduledAt != nil {
				counts.ScheduledRows++
			}
			if episode.ReleasedAt != nil {
				counts.ReleasedRows++
			}
			out.Tracks[episode.Track] = counts
		}
	}
	return out
}
