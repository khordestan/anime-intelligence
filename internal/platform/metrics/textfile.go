package metrics

import (
	"io"
	"os"
	"path/filepath"
	"strings"
)

func ValidTextfilePath(path string) bool {
	return filepath.IsAbs(path) && strings.HasSuffix(path, ".prom")
}

// Stage in the same directory, then rename only after the complete snapshot is
// written, synced and closed. Never follow an existing destination symlink.
func WriteTextfile(path string, write func(io.Writer) error) error {
	temporary, err := os.CreateTemp(filepath.Dir(path), ".anime-metrics-*.tmp")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	defer temporary.Close()
	if err = write(temporary); err != nil {
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
