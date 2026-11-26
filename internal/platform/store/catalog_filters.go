package store

import (
	"context"
	"errors"
	"math"
	"sort"
	"strings"
	"unicode/utf8"
)

var ErrCatalogFilter = errors.New("invalid catalog filter")

type CatalogFilter struct {
	ConfirmedRelease bool   `json:",omitempty"`
	ConfirmedTrack   string `json:",omitempty"`
	MetadataProvider string `json:",omitempty"`
	SeasonNumber     *int   `json:",omitempty"`
	PartNumber       *int   `json:",omitempty"`
	MinEpisodes      *int
	MaxEpisodes      *int
	MinYear          *int
	MaxYear          *int
	Sort             string
	Query            string
	MediaType        string
	Status           string
	ReleaseYear      int
	Season           string
	Genre            string
	Theme            string
	Demographic      string
	Studio           string
	Staff            string `json:",omitempty"`
}
type FilteredCatalog interface {
	FilterCatalog(context.Context, CatalogFilter, int, int) ([]Anime, error)
}

// CatalogPosition is the last ordered key from a public catalog page.
type CatalogPosition struct {
	YearKey int64
	Title   string
	ID      string
}

type CursorCatalog interface {
	CatalogAfter(context.Context, CatalogFilter, int, *CatalogPosition) ([]Anime, error)
}

func (f CatalogFilter) Position(a Anime) CatalogPosition {
	key := int64(0)
	switch f.Sort {
	case "NEWEST", "OLDEST":
		key = int64(a.ReleaseYear)
		if f.Sort == "NEWEST" {
			key = -key
		}
		if a.ReleaseYear == 0 {
			key = math.MaxInt64
		}
	}
	return CatalogPosition{YearKey: key, Title: a.Title, ID: a.ID}
}

func (p CatalogPosition) before(other CatalogPosition) bool {
	if p.YearKey != other.YearKey {
		return p.YearKey < other.YearKey
	}
	if p.Title != other.Title {
		return p.Title < other.Title
	}
	return p.ID < other.ID
}

func (f CatalogFilter) Normalize() (CatalogFilter, error) {
	f.Query = strings.TrimSpace(f.Query)
	f.MetadataProvider = strings.TrimSpace(f.MetadataProvider)
	if len(f.MetadataProvider) > 128 {
		return f, ErrCatalogFilter
	}
	for _, c := range f.MetadataProvider {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '_' || c == '-' || c == '.') {
			return f, ErrCatalogFilter
		}
	}
	f.Genre = strings.TrimSpace(f.Genre)
	f.Theme = strings.TrimSpace(f.Theme)
	f.Demographic = strings.TrimSpace(f.Demographic)
	f.Studio = strings.TrimSpace(f.Studio)
	f.Staff = strings.TrimSpace(f.Staff)
	if !utf8.ValidString(f.Query) || strings.ContainsRune(f.Query, 0) || len(f.Query) > 2048 {
		return f, ErrCatalogFilter
	}
	for _, v := range []string{f.Genre, f.Theme, f.Demographic, f.Studio, f.Staff} {
		if !utf8.ValidString(v) || strings.ContainsRune(v, 0) || len(v) > 256 {
			return f, ErrCatalogFilter
		}
	}
	allowed := func(s string, values ...string) bool {
		if s == "" {
			return true
		}
		for _, v := range values {
			if s == v {
				return true
			}
		}
		return false
	}
	if !allowed(f.ConfirmedTrack, "RAW", "SUB", "DUB", "UNKNOWN") || !allowed(f.Sort, "TITLE", "NEWEST", "OLDEST") {
		return f, ErrCatalogFilter
	}
	if !allowed(f.MediaType, "TV", "MOVIE", "OVA", "ONA", "SPECIAL", "MUSIC", "UNKNOWN") || !allowed(f.Status, "UNKNOWN", "ANNOUNCED", "RELEASING", "FINISHED", "CANCELLED", "HIATUS") || !allowed(f.Season, "WINTER", "SPRING", "SUMMER", "FALL") || (f.ReleaseYear != 0 && (f.ReleaseYear < 1900 || f.ReleaseYear > 9999)) {
		return f, ErrCatalogFilter
	}
	for _, number := range []*int{f.SeasonNumber, f.PartNumber} {
		if number != nil && (*number < 1 || *number > math.MaxInt32) {
			return f, ErrCatalogFilter
		}
	}
	for _, bound := range []*int{f.MinEpisodes, f.MaxEpisodes} {
		if bound != nil && (*bound < 0 || *bound > 100000) {
			return f, ErrCatalogFilter
		}
	}
	if f.MinEpisodes != nil && f.MaxEpisodes != nil && *f.MinEpisodes > *f.MaxEpisodes {
		return f, ErrCatalogFilter
	}
	for _, bound := range []*int{f.MinYear, f.MaxYear} {
		if bound != nil && (*bound < 1900 || *bound > 9999) {
			return f, ErrCatalogFilter
		}
	}
	if f.MinYear != nil && f.MaxYear != nil && *f.MinYear > *f.MaxYear {
		return f, ErrCatalogFilter
	}
	return f, nil
}
func (f CatalogFilter) HasMetadataFilters() bool {
	return f.ConfirmedRelease || f.ConfirmedTrack != "" || f.MetadataProvider != "" || f.SeasonNumber != nil || f.PartNumber != nil || f.MinYear != nil || f.MaxYear != nil || f.Sort != "" || f.MinEpisodes != nil || f.MaxEpisodes != nil || f.MediaType != "" || f.Status != "" || f.ReleaseYear != 0 || f.Season != "" || f.Genre != "" || f.Theme != "" || f.Demographic != "" || f.Studio != "" || f.Staff != ""
}

// Local has no imported taxonomy. An active source-label filter has no matches.
func (s Local) FilterCatalog(ctx context.Context, f CatalogFilter, limit, offset int) ([]Anime, error) {
	if limit < 1 || limit > 100 || offset < 0 || offset > 100000 {
		return nil, ErrCatalogFilter
	}
	return s.filterCatalog(ctx, f, limit, offset, nil)
}

func (s Local) CatalogAfter(ctx context.Context, f CatalogFilter, limit int, after *CatalogPosition) ([]Anime, error) {
	if limit < 1 || limit > 101 {
		return nil, ErrCatalogFilter
	}
	return s.filterCatalog(ctx, f, limit, 0, after)
}

func (s Local) filterCatalog(ctx context.Context, f CatalogFilter, limit, offset int, after *CatalogPosition) ([]Anime, error) {
	f, err := f.Normalize()
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	out := []Anime{}
	if f.ConfirmedRelease || f.ConfirmedTrack != "" || f.MetadataProvider != "" || f.SeasonNumber != nil || f.PartNumber != nil || f.Genre != "" || f.Theme != "" || f.Demographic != "" || f.Studio != "" || f.Staff != "" {
		return out, nil
	}
	for _, a := range s.Store.Catalog(f.Query, 100000, 0) {
		if (f.MediaType != "" && f.MediaType != a.MediaType) || (f.Status != "" && f.Status != a.Status) || (f.ReleaseYear != 0 && f.ReleaseYear != a.ReleaseYear) || (f.Season != "" && f.Season != a.Season) {
			continue
		}
		if (f.MinEpisodes != nil || f.MaxEpisodes != nil) && !a.EpisodeCountKnown {
			continue
		}
		if f.MinEpisodes != nil && a.EpisodeCount < *f.MinEpisodes {
			continue
		}
		if f.MaxEpisodes != nil && a.EpisodeCount > *f.MaxEpisodes {
			continue
		}
		if (f.MinYear != nil || f.MaxYear != nil) && a.ReleaseYear == 0 {
			continue
		}
		if f.MinYear != nil && a.ReleaseYear < *f.MinYear {
			continue
		}
		if f.MaxYear != nil && a.ReleaseYear > *f.MaxYear {
			continue
		}
		if after == nil || after.before(f.Position(a)) {
			out = append(out, a)
		}
	}
	// Match the PostgreSQL deterministic title/ID ordering before pagination.
	sort.Slice(out, func(i, j int) bool {
		return f.Position(out[i]).before(f.Position(out[j]))
	})
	if offset >= len(out) {
		return []Anime{}, nil
	}
	out = out[offset:]
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}
