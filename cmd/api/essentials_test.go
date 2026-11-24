package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/khordestan/anime-intelligence/internal/platform/store"
)

type fixedEssentialsCatalog struct{ title string }

func (c fixedEssentialsCatalog) GetAnime(_ context.Context, id string) (store.Anime, bool, error) {
	return store.Anime{ID: id, Title: c.title}, true, nil
}

func TestLoadEssentialsChecksCatalogAndManifest(t *testing.T) {
	const manifest = `{"version":1,"candidates":[{"animeId":"test","title":"Canonical","evidence":[{"kind":"JURIED_AWARD_WIN","publisherId":"award","sourceUrl":"https://award.example/results"}]}]}`
	path := filepath.Join(t.TempDir(), "essentials.json")
	for _, tc := range []struct {
		name, data, title string
		wantError         bool
	}{
		{"valid", manifest, "Canonical", false},
		{"title mismatch", manifest, "Other", true},
		{"trailing value", manifest + " null", "Canonical", true},
		{"missing evidence", `{"version":1,"candidates":[{"animeId":"test","title":"Canonical"}]}`, "Canonical", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := os.WriteFile(path, []byte(tc.data), 0600); err != nil {
				t.Fatal(err)
			}
			items, err := loadEssentials(context.Background(), path, fixedEssentialsCatalog{tc.title})
			if tc.wantError {
				if err == nil {
					t.Fatal("accepted invalid manifest")
				}
				return
			}
			if err != nil || len(items) != 1 || items[0].Title != tc.title {
				t.Fatalf("items = %+v, err = %v", items, err)
			}
		})
	}
}
