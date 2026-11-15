package main

import (
	"os"
	"path/filepath"

	"github.com/khordestan/anime-intelligence/internal/platform/metrics"
	"github.com/khordestan/anime-intelligence/internal/platform/postgres"
)

// Stage in the same directory, then rename only after the complete snapshot is
// written, synced and closed. Never follow an existing destination symlink.
func writeSnapshotFile(path string, snapshot postgres.QueueDiagnostics) error {
	temporary, err := os.CreateTemp(filepath.Dir(path), ".anime-identity-*.tmp")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	defer temporary.Close()
	if err = metrics.WriteIdentity(temporary, snapshot); err != nil {
		return err
	}
	if err = temporary.Chmod(0640); err != nil {
		return err
	}
	if err = temporary.Sync(); err != nil {
		return err
	}
	if err = temporary.Close(); err != nil {
		return err
	}
	return os.Rename(temporaryPath, path)
}
