package metrics

import (
	"bytes"
	"errors"
	"io"
	"math"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/khordestan/anime-intelligence/internal/platform/postgres"
)

func TestIdentityTextExpositionHasBoundedGaugeSeries(t *testing.T) {
	now := time.Unix(2000, 0).UTC()
	oldest := time.Unix(1970, 0).UTC()
	payload := time.Unix(1900, 0).UTC()
	expiry := time.Unix(2100, 0).UTC()
	snapshot := postgres.QueueDiagnostics{AsOf: now, RequestCapacity: 256, Requests: postgres.QueueCounts{Active: 3, Ready: 2, Completed: 12, OldestActiveAt: &oldest, OldestPayloadAt: &payload, LatestPayloadExpiry: &expiry, Outcomes: postgres.QueueOutcomes{Issued: 5, NoAction: 7}}, Deliveries: postgres.QueueCounts{Outcomes: postgres.QueueOutcomes{Accepted: 4, Exhausted: 1}}}
	var output bytes.Buffer
	if err := WriteIdentity(&output, snapshot); err != nil {
		t.Fatal(err)
	}
	text := output.String()
	for _, expected := range []string{
		"anime_identity_snapshot_timestamp_seconds 2000\n",
		"anime_identity_requests_capacity 256\n",
		"anime_identity_queue_active{queue=\"requests\"} 3\n",
		"anime_identity_queue_outcome_rows{queue=\"requests\",outcome=\"issued\"} 5\n",
		"anime_identity_queue_outcome_rows{queue=\"deliveries\",outcome=\"accepted\"} 4\n",
		"anime_identity_queue_oldest_active_age_seconds{queue=\"requests\"} 30\n",
		"anime_identity_queue_oldest_payload_age_seconds{queue=\"requests\"} 100\n",
		"anime_identity_queue_latest_payload_expiry_timestamp_seconds{queue=\"requests\"} 2100\n",
	} {
		if !strings.Contains(text, expected) {
			t.Fatal("missing metric", expected)
		}
	}
	help, types, samples := map[string]bool{}, map[string]bool{}, map[string]bool{}
	namePattern := regexp.MustCompile(`^([a-zA-Z_:][a-zA-Z0-9_:]*)(\{[^}]+\})?$`)
	for _, line := range strings.Split(strings.TrimSpace(text), "\n") {
		fields := strings.Fields(line)
		if strings.HasPrefix(line, "# HELP ") {
			if help[fields[2]] {
				t.Fatal("duplicate HELP")
			}
			help[fields[2]] = true
			continue
		}
		if strings.HasPrefix(line, "# TYPE ") {
			if len(fields) != 4 || fields[3] != "gauge" || types[fields[2]] {
				t.Fatal("metric not a unique gauge", line)
			}
			types[fields[2]] = true
			continue
		}
		if len(fields) != 2 {
			t.Fatal("invalid sample", line)
		}
		match := namePattern.FindStringSubmatch(fields[0])
		if match == nil || !types[match[1]] || !help[match[1]] {
			t.Fatal("missing metadata or invalid name", line)
		}
		value, err := strconv.ParseFloat(fields[1], 64)
		if err != nil || math.IsNaN(value) || math.IsInf(value, 0) {
			t.Fatal("invalid sample value", line)
		}
		if samples[fields[0]] {
			t.Fatal("duplicate sample", line)
		}
		samples[fields[0]] = true
		if strings.Contains(fields[0], "{") && !strings.Contains(fields[0], `queue="requests"`) && !strings.Contains(fields[0], `queue="deliveries"`) {
			t.Fatal("unbounded queue label")
		}
	}
	if len(help) != 14 || len(types) != 14 || len(samples) != 38 {
		t.Fatal("unexpected metric cardinality", len(help), len(types), len(samples))
	}
	if strings.Contains(text, "_total") || !strings.HasSuffix(text, "\n") {
		t.Fatal("retained counts exposed as counters or missing final newline")
	}
}
func TestIdentityMetricEmptyAndFutureAgesAreZero(t *testing.T) {
	now := time.Unix(2000, 0)
	future := now.Add(time.Minute)
	var output bytes.Buffer
	if err := WriteIdentity(&output, postgres.QueueDiagnostics{AsOf: now, Requests: postgres.QueueCounts{OldestActiveAt: &future}}); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		"anime_identity_queue_oldest_active_age_seconds{queue=\"requests\"} 0\n",
		"anime_identity_queue_oldest_payload_age_seconds{queue=\"requests\"} 0\n",
		"anime_identity_queue_latest_payload_expiry_timestamp_seconds{queue=\"deliveries\"} 0\n",
	} {
		if !strings.Contains(output.String(), expected) {
			t.Fatal("empty/future time handling", expected)
		}
	}
}

type brokenWriter struct{ err error }

func (w brokenWriter) Write(p []byte) (int, error) { return 1, w.err }
func TestIdentityMetricWriteFailuresAreReturned(t *testing.T) {
	failure := errors.New("write failed")
	if err := WriteIdentity(brokenWriter{failure}, postgres.QueueDiagnostics{}); !errors.Is(err, failure) {
		t.Fatal(err)
	}
	if err := WriteIdentity(brokenWriter{}, postgres.QueueDiagnostics{}); !errors.Is(err, io.ErrShortWrite) {
		t.Fatal(err)
	}
}
