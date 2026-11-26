package store

import (
	"context"
	"errors"
	"time"
)

var ErrBulkRemovalChanged = errors.New("selected library entry changed")

type BulkRemovalTarget struct {
	BulkStatusTarget
	ExpectedUpdatedAt time.Time `json:"expectedUpdatedAt"`
}

type BulkRemovalTracker interface {
	RemoveEntries(context.Context, []BulkRemovalTarget) (bool, error)
}
