package store

import (
	"context"
	"time"
)

// CatalogDetails contains public catalog facts only, never account progress.
type CatalogDetails struct {
	Anime Anime `json:"anime"`
	Provider string `json:"provider,omitempty"`
	ObservedAt *time.Time `json:"observedAt,omitempty"`
	Synonyms []string `json:"synonyms"`
	Genres []string `json:"genres"`
	Tags []string `json:"tags"`
	Studios []string `json:"studios"`
	Relations []CatalogRelation `json:"relations"`
	Episodes []CatalogEpisode `json:"episodes"`
}
type CatalogRelation struct {
	AnimeID string `json:"animeId"`
	Type string `json:"type"`
}
type CatalogEpisode struct {
	Number int `json:"number"`
	Track string `json:"track"`
	ScheduledAt *time.Time `json:"scheduledAt,omitempty"`
	ReleasedAt *time.Time `json:"releasedAt,omitempty"`
	VerifiedAt *time.Time `json:"verifiedAt,omitempty"`
	Revision int `json:"revision"`
}
type DetailedCatalog interface {
	CatalogDetails(context.Context,string) (CatalogDetails,bool,error)
}
func EmptyCatalogDetails(a Anime) CatalogDetails {
	return CatalogDetails{Anime:a,Synonyms:[]string{},Genres:[]string{},Tags:[]string{},Studios:[]string{},Relations:[]CatalogRelation{},Episodes:[]CatalogEpisode{}}
}
func (s Local) CatalogDetails(_ context.Context,id string) (CatalogDetails,bool,error) {
	a,ok:=s.Store.GetAnime(id)
	return EmptyCatalogDetails(a),ok,nil
}
