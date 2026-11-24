package feed

import (
	"sort"
	"time"
)

type ScheduleChange struct {
	ExternalID       string    `json:"externalId"`
	Number           int       `json:"number"`
	Track            string    `json:"track"`
	PreviousSchedule time.Time `json:"previousSchedule"`
	CurrentSchedule  time.Time `json:"currentSchedule"`
	Direction        string    `json:"direction"`
}

// ScheduleChanges previews moves between two stated schedules in snapshots
// of the same source. A move does not establish release availability.
func ScheduleChanges(previous *Snapshot, next Snapshot, now time.Time) ([]ScheduleChange, error) {
	if err := validateSnapshotPair(previous, next, now); err != nil {
		return nil, err
	}
	out := make([]ScheduleChange, 0)
	if previous == nil {
		return out, nil
	}
	type episodeTrack struct {
		number int
		track  string
	}
	byTitle := make(map[string]map[episodeTrack]time.Time, len(previous.Titles))
	for _, title := range previous.Titles {
		schedules := make(map[episodeTrack]time.Time)
		for _, episode := range title.Episodes {
			if episode.ScheduledAt == nil || episode.ReleasedAt != nil {
				continue
			}
			key := episodeTrack{episode.Number, episode.Track}
			schedules[key] = episode.ScheduledAt.UTC().Truncate(time.Microsecond)
		}
		byTitle[title.ExternalID] = schedules
	}
	for _, title := range next.Titles {
		schedules := byTitle[title.ExternalID]
		for _, episode := range title.Episodes {
			if episode.ScheduledAt == nil || episode.ReleasedAt != nil {
				continue
			}
			key := episodeTrack{episode.Number, episode.Track}
			before, found := schedules[key]
			after := episode.ScheduledAt.UTC().Truncate(time.Microsecond)
			if !found || before.Equal(after) {
				continue
			}
			direction := "LATER"
			if after.Before(before) {
				direction = "EARLIER"
			}
			out = append(out, ScheduleChange{
				ExternalID: title.ExternalID,
				Number:     episode.Number, Track: episode.Track,
				PreviousSchedule: before, CurrentSchedule: after, Direction: direction,
			})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].ExternalID != out[j].ExternalID {
			return out[i].ExternalID < out[j].ExternalID
		}
		if out[i].Number != out[j].Number {
			return out[i].Number < out[j].Number
		}
		return out[i].Track < out[j].Track
	})
	return out, nil
}
