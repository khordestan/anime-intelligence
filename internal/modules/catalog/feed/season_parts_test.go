package feed

import (
	"testing"
	"time"
)

func TestSeasonPartLabelsRequireAnExplicitSeason(t *testing.T) {
	now := time.Now()
	for _, test := range []struct {
		season, part int32
		valid        bool
	}{{0, 0, true}, {3, 0, true}, {3, 2, true}, {0, 2, false}, {-1, 0, false}, {2, -1, false}} {
		snapshot := Snapshot{Version: 1, Provider: "synthetic", ObservedAt: now, Titles: []Title{{
			ExternalID: "work", Title: "Synthetic work", MediaType: "TV", Status: "ANNOUNCED",
			Season: "WINTER", SeasonNumber: test.season, PartNumber: test.part,
		}}}
		if got := snapshot.Validate(now) == nil; got != test.valid {
			t.Errorf("season %d, part %d: validity %t, want %t", test.season, test.part, got, test.valid)
		}
	}
}
