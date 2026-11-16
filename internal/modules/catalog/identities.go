package catalog

import (
 "net/http"

 "github.com/khordestan/anime-intelligence/internal/platform/store"
)
func validIdentity(s string)bool{
 if len(s)==0||len(s)>128{return false}
 for _,r:=range s{if !((r>='a'&&r<='z')||(r>='A'&&r<='Z')||(r>='0'&&r<='9')||r=='-'||r=='_'||r=='.'){return false}}
 return true
}
func(h *Handler) Lookup(w http.ResponseWriter,r *http.Request){
 q:=r.URL.Query()
 if len(q["provider"])!=1||len(q["externalId"])!=1||!validIdentity(q.Get("provider"))||!validIdentity(q.Get("externalId")){
  writeError(w,400,"invalid_external_identity","Supply one valid provider and externalId");return
 }
 catalog,ok:=h.data.(store.IdentityCatalog)
 if !ok{writeError(w,503,"catalog_unavailable","Identity lookup is unavailable");return}
 anime,found,err:=catalog.GetAnimeByExternalID(r.Context(),q.Get("provider"),q.Get("externalId"))
 if err!=nil{writeError(w,500,"catalog_read_failed","Could not read catalog");return}
 if !found{writeError(w,404,"anime_not_found","Anime was not found");return}
 writeJSON(w,200,anime)
}
