package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/khordestan/anime-intelligence/internal/platform/postgres"
)

func TestAtomicTextfileMatchesStreamAndLeavesNoStagingFiles(t *testing.T) {
	directory := t.TempDir()
	destination := filepath.Join(directory, "identity.prom")
	if err := os.WriteFile(destination, []byte("old snapshot\n"), 0600); err != nil {
		t.Fatal(err)
	}
	snapshot := postgres.QueueDiagnostics{Version: 1, AsOf: time.Unix(2000, 0), RequestCapacity: 256}
	if err := writeSnapshotFile(destination, snapshot); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(destination)
	if err != nil {
		t.Fatal(err)
	}
	var expected bytes.Buffer
	if err = writeSnapshot(&expected, "prometheus", snapshot); err != nil || !bytes.Equal(got, expected.Bytes()) {
		t.Fatal("file differs from stream", err)
	}
	info, err := os.Stat(destination)
	if err != nil || info.Mode().Perm() != 0640 {
		t.Fatal("incorrect collector file permissions", err)
	}
	entries, err := os.ReadDir(directory)
	if err != nil || len(entries) != 1 {
		t.Fatal("staging file retained", err)
	}
}
func TestTextfileReplacesSymlinkWithoutChangingItsTarget(t *testing.T) {
	directory := t.TempDir()
	target := filepath.Join(directory, "unrelated.txt")
	destination := filepath.Join(directory, "identity.prom")
	if err := os.WriteFile(target, []byte("keep this\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, destination); err != nil {
		t.Fatal(err)
	}
	if err := writeSnapshotFile(destination, postgres.QueueDiagnostics{}); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(target)
	if err != nil || string(got) != "keep this\n" {
		t.Fatal("symlink target overwritten", err)
	}
	info, err := os.Lstat(destination)
	if err != nil || !info.Mode().IsRegular() {
		t.Fatal("destination is not a complete regular file", err)
	}
}
func TestTextfileFailurePreservesDestinationAndHidesPaths(t *testing.T) {
	directory := t.TempDir()
	destination := filepath.Join(directory, "identity.prom")
	if err := os.WriteFile(destination, []byte("old snapshot\n"), 0600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if code := run([]string{"-format=prometheus", "-output-file=" + destination}, "invalid://synthetic-secret", &stdout, &stderr); code != 1 || stdout.Len() != 0 || strings.Contains(stderr.String(), "synthetic-secret") || strings.Contains(stderr.String(), directory) {
		t.Fatal("failure output leaked or acknowledged")
	}
	got, err := os.ReadFile(destination)
	if err != nil || string(got) != "old snapshot\n" {
		t.Fatal("failed snapshot replaced prior export", err)
	}
	blocked := filepath.Join(directory, "blocked.prom")
	if err = os.Mkdir(blocked, 0700); err != nil {
		t.Fatal(err)
	}
	if err = writeSnapshotFile(blocked, postgres.QueueDiagnostics{}); err == nil {
		t.Fatal("rename to directory succeeded")
	}
	entries, err := os.ReadDir(directory)
	if err != nil || len(entries) != 2 {
		t.Fatal("failed rename left staging files", err)
	}
	for _, args := range [][]string{{"-output-file=" + destination}, {"-format=prometheus", "-output-file=relative.prom"}, {"-format=prometheus", "-output-file=" + directory + "/synthetic-secret.txt"}} {
		stdout.Reset()
		stderr.Reset()
		if code := run(args, "", &stdout, &stderr); code != 1 || strings.Contains(stderr.String(), "synthetic-secret") || strings.Contains(stderr.String(), directory) {
			t.Fatal("invalid output options accepted or exposed")
		}
	}
}
