package store

import (
	"context"
	"time"
)

// CuratedCatalogFeature describes one current non-spoiler observation.
type CuratedCatalogFeature struct {
	Key        string    `json:"key"`
	Name       string    `json:"name"`
	Confidence float64   `json:"confidence"`
	ObservedAt time.Time `json:"observedAt"`
}

type CuratedFeatureCatalog interface {
	CuratedCatalogFeatures(ctx context.Context, animeID string) (features []CuratedCatalogFeature, truncated bool, err error)
}
