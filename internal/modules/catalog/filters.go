package catalog

import (
 "net/http"
 "net/url"
 "strconv"
 "strings"

 "github.com/khordestan/anime-intelligence/internal/platform/store"
)
func catalogFilter(r *http.Request)(store.CatalogFilter,error){
 query,err:=url.ParseQuery(r.URL.RawQuery);if err!=nil{return store.CatalogFilter{},store.ErrCatalogFilter}
 for _,key:=range []string{"q","mediaType","status","releaseYear","season","genre","theme","demographic","studio"}{
  if values,ok:=query[key];ok&&(len(values)!=1||(key!="q"&&strings.TrimSpace(values[0])=="")){return store.CatalogFilter{},store.ErrCatalogFilter}
 }
 f:=store.CatalogFilter{Query:query.Get("q"),MediaType:query.Get("mediaType"),Status:query.Get("status"),Season:query.Get("season"),Genre:query.Get("genre"),Theme:query.Get("theme"),Demographic:query.Get("demographic"),Studio:query.Get("studio")}
 if year,ok:=query["releaseYear"];ok{n,err:=strconv.Atoi(year[0]);if err!=nil||n==0{return f,store.ErrCatalogFilter};f.ReleaseYear=n}
 return f.Normalize()
}
