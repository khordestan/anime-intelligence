package store

import "context"

const ScheduleHistoryPageSize = 20

type ScheduleHistory struct {
	Items              []CatalogEpisode `json:"items"`
	NextBeforeRevision int              `json:"nextBeforeRevision,omitempty"`
}

type ScheduleHistoryCatalog interface {
	CatalogScheduleHistory(ctx context.Context, id string, number int, track string, before int) (ScheduleHistory, error)
}

func (s Local) CatalogScheduleHistory(context.Context, string, int, string, int) (ScheduleHistory, error) {
	return ScheduleHistory{Items: []CatalogEpisode{}}, nil
}
