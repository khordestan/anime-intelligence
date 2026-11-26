package store

import (
	"context"
	"time"
)

type CatalogCredit struct {
	Name string `json:"name"`
	Role string `json:"role"`
}

type CharacterCredits struct {
	Items      []CatalogCredit `json:"items"`
	Provider   string          `json:"provider,omitempty"`
	ObservedAt *time.Time      `json:"observedAt,omitempty"`
}

type CharacterCreditCatalog interface {
	CharacterCredits(context.Context, string) (CharacterCredits, bool, error)
}

func (s Local) CharacterCredits(_ context.Context, id string) (CharacterCredits, bool, error) {
	_, found := s.Store.GetAnime(id)
	return CharacterCredits{Items: []CatalogCredit{}}, found, nil
}
