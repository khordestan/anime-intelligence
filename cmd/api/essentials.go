package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"

	"github.com/khordestan/anime-intelligence/internal/modules/intelligence"
	"github.com/khordestan/anime-intelligence/internal/platform/store"
)

type essentialsCatalog interface {
	GetAnime(context.Context, string) (store.Anime, bool, error)
}

func loadEssentials(ctx context.Context, path string, catalog essentialsCatalog) ([]intelligence.EssentialsMatch, error) {
	if path == "" {
		return nil, nil
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	const maxBytes = 1 << 20
	reader := &io.LimitedReader{R: file, N: maxBytes + 1}
	decoder := json.NewDecoder(reader)
	decoder.DisallowUnknownFields()
	var manifest struct {
		Version    int                                `json:"version"`
		Candidates []intelligence.EssentialsCandidate `json:"candidates"`
	}
	if decoder.Decode(&manifest) != nil || decoder.Decode(new(any)) != io.EOF || reader.N == 0 ||
		manifest.Version != 1 || len(manifest.Candidates) == 0 || len(manifest.Candidates) > 100 {
		return nil, errors.New("expected a bounded v1 Global Essentials manifest")
	}
	for _, candidate := range manifest.Candidates {
		if len(candidate.Evidence) == 0 {
			return nil, errors.New("Global Essentials manifest candidate requires evidence")
		}
	}
	ranking, err := intelligence.RankGlobalEssentials(manifest.Candidates, 100)
	if err != nil {
		return nil, err
	}
	for i := range ranking {
		anime, found, err := catalog.GetAnime(ctx, ranking[i].AnimeID)
		if err != nil || !found || anime.ID != ranking[i].AnimeID || anime.Title != ranking[i].Title {
			return nil, errors.New("Global Essentials manifest does not match the canonical catalog")
		}
	}
	return ranking, nil
}
