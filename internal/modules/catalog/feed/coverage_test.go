package feed

import (
	"testing"
	"time"
)

func TestSnapshotCoverageKeepsUnknownTotalsAndSchedulePredictionsDistinct(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	zero := 0
	snapshot := Snapshot{Version: 1, Provider: "test", ObservedAt: now, Titles: []Title{
		{ExternalID: "a", Title: "A", MediaType: "UNKNOWN", Status: "UNKNOWN", EpisodeCount: &zero},
		{ExternalID: "b", Title: "B", MediaType: "TV", Status: "FINISHED", NativeTitle: " ", ReleaseYear: 2020,
			Episodes: []Episode{
				{Number: 1, Track: "SUB", ScheduledAt: &now},
				{Number: 1, Track: "RAW", ScheduledAt: &now, ReleasedAt: &now},
				{Number: 1, Track: "DUB", ReleasedAt: &now},
			},
		},
	}}
	if err := snapshot.Validate(now); err != nil {
		t.Fatal(err)
	}
	coverage := snapshot.Coverage()
	if coverage.Titles != 2 || coverage.Fields["episodeCount"] != 1 ||
		coverage.Fields["nativeTitle"] != 0 || coverage.Fields["mediaType"] != 1 ||
		coverage.Fields["status"] != 1 || coverage.Fields["releaseYear"] != 1 {
		t.Fatalf("unexpected field counts: %+v", coverage)
	}
	if len(coverage.Tracks) != 4 || coverage.Tracks["SUB"] != (ScheduleCoverage{ScheduledRows: 1}) ||
		coverage.Tracks["RAW"] != (ScheduleCoverage{ScheduledRows: 1, ReleasedRows: 1}) ||
		coverage.Tracks["DUB"] != (ScheduleCoverage{ReleasedRows: 1}) ||
		coverage.Tracks["UNKNOWN"] != (ScheduleCoverage{}) {
		t.Fatalf("unexpected schedule coverage: %+v", coverage.Tracks)
	}
}
