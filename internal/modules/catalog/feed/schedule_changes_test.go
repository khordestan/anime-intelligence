package feed

import (
	"testing"
	"time"
)

func TestScheduleChangesDistinguishesMovesFromConfirmationsAndBaselines(t *testing.T) {
	now := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	original := now.Add(-time.Hour)
	later := now.Add(3 * time.Hour)
	earlier := now.Add(-2 * time.Hour)
	sameAtStoragePrecision := original.Add(50 * time.Nanosecond)
	released := now.Add(-time.Minute)
	previous := Snapshot{Version: 1, Provider: "synthetic", ObservedAt: now.Add(-3 * time.Hour), Titles: []Title{
		{ExternalID: "title", Title: "Example", MediaType: "TV", Status: "RELEASING", Episodes: []Episode{
			{Number: 1, Track: "SUB", ScheduledAt: &original},
			{Number: 1, Track: "DUB", ScheduledAt: &original},
			{Number: 2, Track: "SUB", ScheduledAt: &original},
			{Number: 3, Track: "SUB", ScheduledAt: &original},
			{Number: 4, Track: "SUB", ScheduledAt: &original},
		}},
	}}
	next := Snapshot{Version: 1, Provider: previous.Provider, ObservedAt: now, Titles: []Title{
		{ExternalID: "title", Title: "Example", MediaType: "TV", Status: "RELEASING", Episodes: []Episode{
			{Number: 1, Track: "SUB", ScheduledAt: &later},
			{Number: 1, Track: "DUB", ScheduledAt: &earlier},
			{Number: 2, Track: "SUB", ScheduledAt: &sameAtStoragePrecision},
			{Number: 3, Track: "SUB", ScheduledAt: &later, ReleasedAt: &released},
			{Number: 5, Track: "SUB", ScheduledAt: &later},
		}},
	}}
	changes, err := ScheduleChanges(&previous, next, now)
	if err != nil || len(changes) != 2 || changes[0] != (ScheduleChange{"title", 1, "DUB", original, earlier, "EARLIER"}) ||
		changes[1] != (ScheduleChange{"title", 1, "SUB", original, later, "LATER"}) {
		t.Fatalf("changes = %+v, err = %v", changes, err)
	}
	confirmed, err := Releases(&previous, next, now)
	if err != nil || len(confirmed) != 1 || confirmed[0].Number != 3 {
		t.Fatalf("confirmed = %+v, err = %v", confirmed, err)
	}
	if changes, err := ScheduleChanges(nil, next, now); err != nil || len(changes) != 0 {
		t.Fatalf("first snapshot = %+v, err = %v", changes, err)
	}
	next.Provider = "other"
	if _, err := ScheduleChanges(&previous, next, now); err == nil {
		t.Fatal("accepted snapshots from different sources")
	}
}
