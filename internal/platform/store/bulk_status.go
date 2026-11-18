package store

import (
 "context"
 "errors"
)

const MaxBulkStatusEntries = 25

var ErrBulkStatusInvalid = errors.New("invalid bulk tracker status change")

type BulkStatusTarget struct {
 ID string `json:"id"`
 ExpectedCycle int `json:"expectedCycle"`
}

type BulkStatusTracker interface {
 UpdateStatuses(context.Context, string, []BulkStatusTarget) ([]Entry, bool, error)
}
