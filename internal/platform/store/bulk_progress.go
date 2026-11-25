package store

import (
	"context"
	"errors"
)

var ErrBulkProgressConflict = errors.New("bulk progress state changed")

type BulkProgressTarget struct {
	BulkStatusTarget
	ExpectedEpisodesWatched *int `json:"expectedEpisodesWatched"`
}

type BulkProgressTracker interface {
	AdvanceProgress(context.Context, []BulkProgressTarget) ([]Entry, bool, error)
}
