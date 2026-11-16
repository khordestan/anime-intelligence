package store

import (
	"context"
	"errors"
)

var ErrUnauthorized = errors.New("unauthorized")
var ErrEmailUnverified = errors.New("email verification required")

// Repository is the request-aware persistence boundary shared by both runtimes.
// PostgreSQL implementations must resolve ownership from a verified session.
type Repository interface {
	Catalog(context.Context, string, int, int) ([]Anime, error)
	GetAnime(context.Context, string) (Anime, bool, error)
	Library(context.Context) ([]Entry, error)
	Add(context.Context, string, string, int) (Entry, error)
	Update(context.Context, string, *string, *int) (Entry, bool, error)
	Remove(context.Context, string) (bool, error)
	Rate(context.Context, string, int) (Entry, bool, error)
}

type Local struct{ *Store }

func (s Local) Catalog(ctx context.Context, q string, l, o int) ([]Anime, error) {
 return s.FilterCatalog(ctx,CatalogFilter{Query:q},l,o)
}
func (s Local) GetAnime(_ context.Context, id string) (Anime, bool, error) {
	a, ok := s.Store.GetAnime(id)
	return a, ok, nil
}
func (s Local) Library(_ context.Context) ([]Entry, error) { return s.Store.Library(), nil }
func (s Local) Add(_ context.Context, a, status string, n int) (Entry, error) {
	return s.Store.Add(a, status, n)
}
func (s Local) Update(_ context.Context, id string, status *string, n *int) (Entry, bool, error) {
	return s.Store.Update(id, status, n)
}
func (s Local) Rate(_ context.Context, id string, n int) (Entry, bool, error) {
	return s.Store.Rate(id, n)
}

func (s Local) Remove(_ context.Context, id string) (bool, error) { return s.Store.Remove(id) }
