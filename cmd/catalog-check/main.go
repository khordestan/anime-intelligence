// catalog-check validates an offline catalog feed and previews release events.
// It never writes the database or sends notifications.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/khordestan/anime-intelligence/internal/modules/catalog/feed"
)

func read(path string, now time.Time) (feed.Snapshot, error) {
	f, err := os.Open(path)
	if err != nil {
		return feed.Snapshot{}, feed.ErrInvalid
	}
	defer f.Close()
	return feed.Decode(f, now)
}
func run() error {
	input := flag.String("input", "", "Snapshot JSON file (required)")
	baseline := flag.String("previous", "", "Optional previous snapshot JSON file")
	referencePath := flag.String("reference", "", "independent reviewed snapshot for field accuracy scoring")
	coverage := flag.Bool("coverage", false, "report aggregate field and schedule presence for the snapshot")
	changes := flag.Bool("schedule-changes", false, "preview schedule moves between the supplied snapshots")
	flag.Parse()
	now := time.Now().UTC()
	next, err := read(*input, now)
	if err != nil {
		return err
	}
	var previous *feed.Snapshot
	if *baseline != "" {
		s, err := read(*baseline, now)
		if err != nil {
			return err
		}
		previous = &s
	}
	events, err := feed.Releases(previous, next, now)
	if err != nil {
		return err
	}
	var report *feed.SnapshotCoverage
	if *coverage {
		counts := next.Coverage()
		report = &counts
	}
	var accuracy *feed.FieldAccuracy
	if *referencePath != "" {
		reference, err := read(*referencePath, now)
		if err != nil {
			return err
		}
		scored, err := feed.CompareReviewedFields(next, reference, now)
		if err != nil {
			return err
		}
		accuracy = &scored
	}
	var scheduleChanges *[]feed.ScheduleChange
	if *changes {
		moves, err := feed.ScheduleChanges(previous, next, now)
		if err != nil {
			return err
		}
		scheduleChanges = &moves
	}
	return json.NewEncoder(os.Stdout).Encode(struct {
		Titles          int                    `json:"titles"`
		Events          []feed.Event           `json:"events"`
		Coverage        *feed.SnapshotCoverage `json:"coverage,omitempty"`
		ScheduleChanges *[]feed.ScheduleChange `json:"scheduleChanges,omitempty"`
		Accuracy        *feed.FieldAccuracy    `json:"accuracy,omitempty"`
	}{len(next.Titles), events, report, scheduleChanges, accuracy})
}
func main() {
	if run() != nil {
		fmt.Fprintln(os.Stderr, "catalog validation failed")
		os.Exit(1)
	}
}
