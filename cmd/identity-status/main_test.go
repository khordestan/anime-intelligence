package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestDiagnosticsFailWithoutLeakingConnectionDetails(t *testing.T) {
	for _, dsn := range []string{"", "postgres://synthetic-user:synthetic-secret@%invalid/synthetic-private-db"} {
		var stdout, stderr bytes.Buffer
		if code := run(dsn, &stdout, &stderr); code != 1 {
			t.Fatal("invalid configuration accepted", code)
		}
		if stdout.Len() != 0 || stderr.Len() == 0 {
			t.Fatal("failure output contract")
		}
		for _, private := range []string{"synthetic-user", "synthetic-secret", "synthetic-private-db"} {
			if strings.Contains(stdout.String()+stderr.String(), private) {
				t.Fatal("connection detail leaked")
			}
		}
	}
}
