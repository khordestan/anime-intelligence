// Package metrics renders bounded operational snapshots without private labels.
package metrics

import (
	"bytes"
	"fmt"
	"io"
	"time"

	"github.com/khordestan/anime-intelligence/internal/platform/postgres"
)

// WriteIdentity emits Prometheus text exposition with only fixed queue/outcome
// labels. Retained row counts are gauges, never cumulative counters.
func WriteIdentity(w io.Writer, snapshot postgres.QueueDiagnostics) error {
	var output bytes.Buffer
	gauge := func(name, help string) { fmt.Fprintf(&output, "# HELP %s %s\n# TYPE %s gauge\n", name, help, name) }
	gauge("anime_identity_snapshot_timestamp_seconds", "Database snapshot Unix timestamp in seconds.")
	fmt.Fprintf(&output, "anime_identity_snapshot_timestamp_seconds %d\n", snapshot.AsOf.Unix())
	gauge("anime_identity_requests_capacity", "Shared active request admission capacity.")
	fmt.Fprintf(&output, "anime_identity_requests_capacity %d\n", snapshot.RequestCapacity)
	queues := []struct {
		name   string
		counts postgres.QueueCounts
	}{{"requests", snapshot.Requests}, {"deliveries", snapshot.Deliveries}}
	for _, metric := range []struct {
		name, help string
		value      func(postgres.QueueCounts) int64
	}{
		{"active", "Uncompleted unexpired records.", func(q postgres.QueueCounts) int64 { return q.Active }},
		{"ready", "Due active records eligible for another claim.", func(q postgres.QueueCounts) int64 { return q.Ready }},
		{"leased", "Active records with an unexpired lease.", func(q postgres.QueueCounts) int64 { return q.Leased }},
		{"delayed", "Active records waiting for a scheduled retry.", func(q postgres.QueueCounts) int64 { return q.Delayed }},
		{"attempted_active", "Active records with at least one persisted claim, including first attempts.", func(q postgres.QueueCounts) int64 { return q.AttemptedActive }},
		{"awaiting_cleanup", "Expired or attempt-limited records awaiting cleanup.", func(q postgres.QueueCounts) int64 { return q.AwaitingCleanup }},
		{"completed", "Retained terminal records, including discarded work.", func(q postgres.QueueCounts) int64 { return q.Completed }},
		{"payload_rows", "Records retaining encrypted payloads.", func(q postgres.QueueCounts) int64 { return q.PayloadRows }},
	} {
		name := "anime_identity_queue_" + metric.name
		gauge(name, metric.help)
		for _, queue := range queues {
			fmt.Fprintf(&output, "%s{queue=%q} %d\n", name, queue.name, metric.value(queue.counts))
		}
	}
	gauge("anime_identity_queue_outcome_rows", "Retained completed rows by bounded outcome; decreases with cleanup and cascades.")
	for _, queue := range queues {
		q := queue.counts.Outcomes
		for _, outcome := range []struct {
			name  string
			value int64
		}{{"issued", q.Issued}, {"no_action", q.NoAction}, {"accepted", q.Accepted}, {"invalidated", q.Invalidated}, {"expired", q.Expired}, {"exhausted", q.Exhausted}, {"legacy_unknown", q.LegacyUnknown}} {
			fmt.Fprintf(&output, "anime_identity_queue_outcome_rows{queue=%q,outcome=%q} %d\n", queue.name, outcome.name, outcome.value)
		}
	}
	for _, metric := range []struct {
		name, help string
		value      func(postgres.QueueCounts) float64
	}{
		{"oldest_active_age_seconds", "Age of oldest active record at snapshot time; zero when empty.", func(q postgres.QueueCounts) float64 { return age(snapshot.AsOf, q.OldestActiveAt) }},
		{"oldest_payload_age_seconds", "Age of oldest retained payload at snapshot time; zero when empty.", func(q postgres.QueueCounts) float64 { return age(snapshot.AsOf, q.OldestPayloadAt) }},
		{"latest_payload_expiry_timestamp_seconds", "Latest retained payload expiry Unix timestamp; zero when empty.", func(q postgres.QueueCounts) float64 {
			if q.LatestPayloadExpiry == nil {
				return 0
			}
			return float64(q.LatestPayloadExpiry.Unix())
		}},
	} {
		name := "anime_identity_queue_" + metric.name
		gauge(name, metric.help)
		for _, queue := range queues {
			fmt.Fprintf(&output, "%s{queue=%q} %g\n", name, queue.name, metric.value(queue.counts))
		}
	}
	n, err := w.Write(output.Bytes())
	if err == nil && n != output.Len() {
		return io.ErrShortWrite
	}
	return err
}
func age(asOf time.Time, created *time.Time) float64 {
	if created == nil || created.After(asOf) {
		return 0
	}
	return asOf.Sub(*created).Seconds()
}
