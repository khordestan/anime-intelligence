package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTextfileFailurePreservesPreviousSnapshot(t *testing.T) {
	destination := filepath.Join(t.TempDir(), "release.prom")
	const previous = "previous metrics\n"
	if err := os.WriteFile(destination, []byte(previous), 0640); err != nil {
		t.Fatal(err)
	}
	const dsn = "invalid://synthetic-secret"
	var stdout, stderr bytes.Buffer
	if got := run([]string{"-format=prometheus", "-output-file=" + destination}, dsn, &stdout, &stderr); got != 1 {
		t.Fatalf("exit status %d", got)
	}
	if stdout.Len() != 0 || strings.Contains(stderr.String(), destination) || strings.Contains(stderr.String(), dsn) {
		t.Fatal("diagnostics exposed connection or destination details")
	}
	data, err := os.ReadFile(destination)
	if err != nil || string(data) != previous {
		t.Fatalf("previous snapshot changed: %q %v", data, err)
	}
}

func TestTextfileRequiresPrometheusAndAnAbsolutePath(t *testing.T) {
	for _, args := range [][]string{
		{"-output-file=/tmp/release.prom"},
		{"-format=prometheus", "-output-file=relative.prom"},
		{"-format=prometheus", "-output-file=/tmp/release.txt"},
	} {
		var stdout, stderr bytes.Buffer
		if got := run(args, "", &stdout, &stderr); got != 1 || stdout.Len() != 0 || strings.TrimSpace(stderr.String()) != usage {
			t.Fatalf("input validation failed for %q", args)
		}
	}
}
