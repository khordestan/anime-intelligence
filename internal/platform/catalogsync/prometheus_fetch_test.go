package catalogsync

import (
	"strings"
	"testing"
	"time"
)

func TestPrometheusFetchOutcomesUseOnlyFixedLabelsAndPolicyRecordedSources(t *testing.T) {
	failed, success, unknown := "FETCH_FAILED", "SUCCESS", "synthetic-unrecognized"
	status := Status{AsOf: time.Unix(2000, 0), StaleAfterSeconds: 86400, Sources: []SourceStatus{
		{Enabled: true, PolicyRecorded: true, Provider: "synthetic-provider", Outcome: &failed},
		{Enabled: true, PolicyRecorded: true, Outcome: &success},
		{Enabled: true, PolicyRecorded: true, Outcome: &unknown},
		{Enabled: true, PolicyRecorded: true},
		{Enabled: true, Outcome: &failed},
		{PolicyRecorded: true, Outcome: &failed},
	}}
	text, err := Prometheus(status)
	if err != nil {
		t.Fatal(err)
	}
	for _, sample := range []string{
		`{outcome="fetch_failed"} 1`,
		`{outcome="success"} 1`,
		`{outcome="unknown"} 1`,
		`{outcome="never_attempted"} 1`,
		`{outcome="import_failed"} 0`,
	} {
		if !strings.Contains(text, "anime_catalog_sources_by_last_fetch_outcome"+sample+"\n") {
			t.Errorf("missing sample %s", sample)
		}
	}
	if strings.Contains(text, "synthetic-") {
		t.Fatal("provider identifier or unrecognized outcome escaped the allowlist")
	}
}
