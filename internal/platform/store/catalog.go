package store

import (
	"context"
	"time"
)

// CatalogDetails contains public catalog facts only, never account progress.
type CatalogDetails struct {
 FieldSources []CatalogFieldSource `json:"fieldSources"`
	ExternalIDs []CatalogExternalID `json:"externalIds"`
	Anime Anime `json:"anime"`
	Provider string `json:"provider,omitempty"`
	ObservedAt *time.Time `json:"observedAt,omitempty"`
	Synonyms []string `json:"synonyms"`
	Genres []string `json:"genres"`
	Tags []string `json:"tags"`
	Themes []string `json:"themes"`
	Demographics []string `json:"demographics"`
	Studios []string `json:"studios"`
	Relations []CatalogRelation `json:"relations"`
	Episodes []CatalogEpisode `json:"episodes"`
}
type CatalogFieldSource struct {
 Field string `json:"field"`
 Mode string `json:"mode"`
 Revision int `json:"revision"`
 Provider string `json:"provider"`
 ObservedAt *time.Time `json:"observedAt,omitempty"`
}
type CatalogExternalID struct {Provider string `json:"provider"`;ExternalID string `json:"externalId"`}
type CatalogRelation struct {
	Title string `json:"title"`
	MediaType string `json:"mediaType"`
	Status string `json:"status"`
	AnimeID string `json:"animeId"`
	Type string `json:"type"`
}
type CatalogEpisode struct {
 Provider string `json:"provider,omitempty"`
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
	return CatalogDetails{Anime:a,FieldSources:[]CatalogFieldSource{},ExternalIDs:[]CatalogExternalID{},Synonyms:[]string{},Genres:[]string{},Tags:[]string{},Themes:[]string{},Demographics:[]string{},Studios:[]string{},Relations:[]CatalogRelation{},Episodes:[]CatalogEpisode{}}
}
func (s Local) CatalogDetails(_ context.Context,id string) (CatalogDetails,bool,error) {
	a,ok:=s.Store.GetAnime(id)
	return EmptyCatalogDetails(a),ok,nil
}

type IdentityCatalog interface {
 GetAnimeByExternalID(context.Context,string,string)(Anime,bool,error)
}
func(s Local) GetAnimeByExternalID(_ context.Context,provider,external string)(Anime,bool,error){
 return Anime{},false,nil
}
