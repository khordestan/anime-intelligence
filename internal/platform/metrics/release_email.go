package metrics

import (
	"bytes"
	"fmt"
	"io"

	"github.com/khordestan/anime-intelligence/internal/platform/postgres"
)

func WriteReleaseEmail(w io.Writer, snapshot postgres.ReleaseEmailDiagnostics) error {
	var output bytes.Buffer
	for _, metric := range []struct {
		name, description string
		value             int64
	}{
		{"snapshot_timestamp_seconds", "Database snapshot Unix timestamp.", snapshot.AsOf.Unix()},
		{"subscribers", "Enrolled accounts with a scheduled scan.", snapshot.Subscribers},
		{"due", "Accounts whose next scan or lease expiry is due.", snapshot.Due},
		{"leased", "Accounts with an active delivery lease.", snapshot.Leased},
		{"delayed", "Accounts awaiting a scheduled scan or retry.", snapshot.Delayed},
		{"attempted", "Accounts with a persisted delivery attempt, including first attempts.", snapshot.Attempted},
		{"exhausted_due", "Due accounts whose pending event has reached its attempt limit.", snapshot.ExhaustedDue},
	} {
		name := "anime_release_email_" + metric.name
		fmt.Fprintf(&output, "# HELP %s %s\n# TYPE %s gauge\n%s %d\n",
			name, metric.description, name, name, metric.value)
	}
	if snapshot.OldestDueAt != nil {
		fmt.Fprintf(&output, "# HELP anime_release_email_oldest_due_timestamp_seconds Oldest due scan Unix timestamp.\n# TYPE anime_release_email_oldest_due_timestamp_seconds gauge\nanime_release_email_oldest_due_timestamp_seconds %d\n", snapshot.OldestDueAt.Unix())
	}
	_, err := output.WriteTo(w)
	return err
}
