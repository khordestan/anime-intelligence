package catalog

import (
 "net/http"

 "github.com/khordestan/anime-intelligence/internal/platform/store"
)

func(h *Handler)Continuity(w http.ResponseWriter,r *http.Request){
 id:=r.PathValue("id")
 if !validUUID(id){writeError(w,400,"invalid_id","Anime ID must be a UUID");return}
 data,ok:=h.data.(store.ContinuityCatalog)
 if !ok{writeError(w,503,"catalog_unavailable","Linked titles are unavailable");return}
 graph,found,err:=data.CatalogContinuity(r.Context(),id)
 if err!=nil{writeError(w,500,"catalog_read_failed","Could not read linked titles");return}
 if !found{writeError(w,404,"anime_not_found","Anime was not found");return}
 writeJSON(w,200,graph)
}
