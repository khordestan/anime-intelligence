package catalog
import("net/http";"net/url";"strconv";"github.com/khordestan/anime-intelligence/internal/platform/store")
func(h *Handler)Facets(w http.ResponseWriter,r *http.Request){
 q,err:=url.ParseQuery(r.URL.RawQuery);if err!=nil{writeError(w,400,"invalid_catalog_filter","Check facet parameters");return}
 for key,values:=range q{if (key!="field"&&key!="prefix"&&key!="limit")||len(values)!=1{writeError(w,400,"invalid_catalog_filter","Check facet parameters");return}}
 limit:=50
 if values,ok:=q["limit"];ok{limit,err=strconv.Atoi(values[0]);if err!=nil{writeError(w,400,"invalid_catalog_filter","Check facet parameters");return}}
 field,prefix:=q.Get("field"),q.Get("prefix")
 if store.ValidateFacet(field,prefix,limit)!=nil{writeError(w,400,"invalid_catalog_filter","Check facet parameters");return}
 data,ok:=h.data.(store.FacetedCatalog);if !ok{writeError(w,503,"catalog_unavailable","Catalog facets unavailable");return}
 page,err:=data.CatalogFacets(r.Context(),field,prefix,limit);if err!=nil{writeError(w,500,"catalog_read_failed","Could not read catalog facets");return}
 writeJSON(w,200,map[string]any{"field":field,"items":page.Items,"hasMore":page.HasMore,"limit":limit})
}
