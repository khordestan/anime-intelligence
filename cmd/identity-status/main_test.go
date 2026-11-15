package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"github.com/khordestan/anime-intelligence/internal/platform/postgres"
	"io"
	"strings"
	"testing"
)

func TestDiagnosticsFailWithoutLeakingConnectionDetails(t *testing.T) {
	for _, dsn := range []string{"", "postgres://synthetic-user:synthetic-secret@%invalid/synthetic-private-db"} {
		var stdout, stderr bytes.Buffer
		if code := run(nil, dsn, &stdout, &stderr); code != 1 {
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

func TestOutputFlagsNeverEchoUntrustedArguments(t *testing.T) {
	for _, args := range [][]string{{"-format=synthetic-secret"}, {"-unknown=synthetic-secret"}, {"synthetic-secret"}, {"-format"}} {
		var stdout, stderr bytes.Buffer
		if code := run(args, "", &stdout, &stderr); code != 1 || stdout.Len() != 0 || strings.Contains(stderr.String(), "synthetic-secret") {
			t.Fatal("argument failure exposed input or succeeded")
		}
	}
	var stdout, stderr bytes.Buffer
	if code := run([]string{"-h"}, "", &stdout, &stderr); code != 0 || !strings.Contains(stdout.String(), "prometheus") || stderr.Len() != 0 {
		t.Fatal("help requires database")
	}
	for _, format := range []string{"json", "prometheus"} {
		stdout.Reset()
		stderr.Reset()
		if code := run([]string{"-format", format}, "", &stdout, &stderr); code != 1 || stdout.Len() != 0 || !strings.Contains(stderr.String(), "DATABASE_URL") {
			t.Fatal("valid format did not reach configuration check")
		}
	}
}
func TestSnapshotFormatsAndWriteFailures(t *testing.T) {
	var helpErrors bytes.Buffer
	if code := run([]string{"-h"}, "", failingWriter{}, &helpErrors); code != 1 {
		t.Fatal("help output failure ignored")
	}
	var jsonOutput, metricOutput bytes.Buffer
	snapshot := postgres.QueueDiagnostics{Version: 1, RequestCapacity: 256}
	if err := writeSnapshot(&jsonOutput, "json", snapshot); err != nil || !json.Valid(jsonOutput.Bytes()) {
		t.Fatal("default JSON changed", err)
	}
	if err := writeSnapshot(&metricOutput, "prometheus", snapshot); err != nil || !strings.Contains(metricOutput.String(), "anime_identity_requests_capacity 256") {
		t.Fatal("metric output failed", err)
	}
	if err := writeSnapshot(io.Discard, "unsupported", snapshot); err == nil {
		t.Fatal("unsupported format accepted")
	}
	for _, format := range []string{"json", "prometheus"} {
		if err := writeSnapshot(failingWriter{}, format, snapshot); err == nil {
			t.Fatal("write failure ignored")
		}
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("synthetic output failure") }
