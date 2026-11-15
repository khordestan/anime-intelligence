package catalog

import "net/http"

import "github.com/khordestan/anime-intelligence/internal/platform/store"

func (h *Handler) Details(w http.ResponseWriter,r *http.Request) {
	id:=r.PathValue("id")
	if !validUUID(id) {writeError(w,400,"invalid_id","Anime ID must be a UUID");return}
	data,ok:=h.data.(store.DetailedCatalog)
	if !ok {writeError(w,503,"catalog_unavailable","Catalog details are unavailable");return}
	details,found,err:=data.CatalogDetails(r.Context(),id)
	if err!=nil {writeError(w,500,"catalog_read_failed","Could not read catalog");return}
	if !found {writeError(w,404,"anime_not_found","Anime was not found");return}
	writeJSON(w,200,details)
}
