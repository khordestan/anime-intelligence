package store

import (
	"context"
	"errors"
)

var ErrSeasonRatingUnavailable = errors.New("catalog season rating is unavailable")

type SeasonRatingTracker interface {
	SetSeasonRating(ctx context.Context, id string, expectedCycle int, score *int) (Entry, bool, error)
}
