package store

import (
 "context"
 "time"
)

const ContinuityLimit = 50

// Continuity contains public explicit prequel/sequel links. Neither distance
// from the requested title nor edge direction recommends a viewing order.
type Continuity struct {
 RootID string `json:"rootId"`
 Nodes []ContinuityNode `json:"nodes"`
 Edges []ContinuityEdge `json:"edges"`
 Limit int `json:"limit"`
 Truncated bool `json:"truncated"`
}
type ContinuityNode struct {
 ID string `json:"id"`
 Title string `json:"title"`
 MediaType string `json:"mediaType"`
 Status string `json:"status"`
 EpisodeCount int `json:"episodeCount,omitempty"`
 EpisodeCountKnown bool `json:"episodeCountKnown"`
 ReleaseYear int `json:"releaseYear,omitempty"`
 Season string `json:"season,omitempty"`
 Distance int `json:"distance"`
 Provider string `json:"provider,omitempty"`
 ObservedAt *time.Time `json:"observedAt,omitempty"`
}
type ContinuityEdge struct {
 FromID string `json:"fromId"`
 ToID string `json:"toId"`
 Providers []string `json:"providers"`
}
type ContinuityCatalog interface {
 CatalogContinuity(context.Context,string)(Continuity,bool,error)
}
func EmptyContinuity(id string)Continuity{
 return Continuity{RootID:id,Nodes:[]ContinuityNode{},Edges:[]ContinuityEdge{},Limit:ContinuityLimit}
}
func(s Local)CatalogContinuity(_ context.Context,id string)(Continuity,bool,error){
 a,ok:=s.Store.GetAnime(id)
 out:=EmptyContinuity(id)
 if ok{out.RootID=a.ID;out.Nodes=append(out.Nodes,ContinuityNode{ID:a.ID,Title:a.Title,MediaType:a.MediaType,Status:a.Status,EpisodeCount:a.EpisodeCount,EpisodeCountKnown:a.EpisodeCountKnown,ReleaseYear:a.ReleaseYear,Season:a.Season})}
 return out,ok,nil
}
