package main

import (
	"io"

	"github.com/khordestan/anime-intelligence/internal/platform/metrics"
	"github.com/khordestan/anime-intelligence/internal/platform/postgres"
)

func writeSnapshotFile(path string, snapshot postgres.QueueDiagnostics) error {
	return metrics.WriteTextfile(path, func(w io.Writer) error {
		return metrics.WriteIdentity(w, snapshot)
	})
}
