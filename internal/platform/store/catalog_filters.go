package store

import (
 "context"
 "errors"
 "strings"
 "sort"
 "unicode/utf8"
)

var ErrCatalogFilter = errors.New("invalid catalog filter")

type CatalogFilter struct {
 MinEpisodes *int
 MaxEpisodes *int
 Sort string
 Query string
 MediaType string
 Status string
 ReleaseYear int
 Season string
 Genre string
 Theme string
 Demographic string
 Studio string
}
type FilteredCatalog interface {
 FilterCatalog(context.Context,CatalogFilter,int,int)([]Anime,error)
}
func(f CatalogFilter)Normalize()(CatalogFilter,error){
 f.Query=strings.TrimSpace(f.Query)
 f.Genre=strings.TrimSpace(f.Genre);f.Theme=strings.TrimSpace(f.Theme)
 f.Demographic=strings.TrimSpace(f.Demographic);f.Studio=strings.TrimSpace(f.Studio)
 if !utf8.ValidString(f.Query)||strings.ContainsRune(f.Query,0)||len(f.Query)>2048{return f,ErrCatalogFilter}
 for _,v:=range []string{f.Genre,f.Theme,f.Demographic,f.Studio}{if !utf8.ValidString(v)||strings.ContainsRune(v,0)||len(v)>256{return f,ErrCatalogFilter}}
 allowed:=func(s string,values ...string)bool{if s==""{return true};for _,v:=range values{if s==v{return true}};return false}
 if !allowed(f.Sort,"TITLE","NEWEST","OLDEST"){return f,ErrCatalogFilter}
 if !allowed(f.MediaType,"TV","MOVIE","OVA","ONA","SPECIAL","MUSIC","UNKNOWN")||!allowed(f.Status,"UNKNOWN","ANNOUNCED","RELEASING","FINISHED","CANCELLED","HIATUS")||!allowed(f.Season,"WINTER","SPRING","SUMMER","FALL")||(f.ReleaseYear!=0&&(f.ReleaseYear<1900||f.ReleaseYear>9999)){return f,ErrCatalogFilter}
 for _,bound:=range []*int{f.MinEpisodes,f.MaxEpisodes}{if bound!=nil&&(*bound<0||*bound>100000){return f,ErrCatalogFilter}}
 if f.MinEpisodes!=nil&&f.MaxEpisodes!=nil&&*f.MinEpisodes>*f.MaxEpisodes{return f,ErrCatalogFilter}
 return f,nil
}
func(f CatalogFilter)HasMetadataFilters()bool{
 return f.Sort!=""||f.MinEpisodes!=nil||f.MaxEpisodes!=nil||f.MediaType!=""||f.Status!=""||f.ReleaseYear!=0||f.Season!=""||f.Genre!=""||f.Theme!=""||f.Demographic!=""||f.Studio!=""
}
// Local has no imported taxonomy. An active source-label filter has no matches.
func(s Local)FilterCatalog(ctx context.Context,f CatalogFilter,limit,offset int)([]Anime,error){
 f,err:=f.Normalize();if err!=nil||limit<1||limit>100||offset<0||offset>100000{return nil,ErrCatalogFilter}
 if err:=ctx.Err();err!=nil{return nil,err}
 out:=[]Anime{}
 if f.Genre!=""||f.Theme!=""||f.Demographic!=""||f.Studio!=""{return out,nil}
 for _,a:=range s.Store.Catalog(f.Query,100000,0){
  if (f.MediaType!=""&&f.MediaType!=a.MediaType)||(f.Status!=""&&f.Status!=a.Status)||(f.ReleaseYear!=0&&f.ReleaseYear!=a.ReleaseYear)||(f.Season!=""&&f.Season!=a.Season){continue}
  if (f.MinEpisodes!=nil||f.MaxEpisodes!=nil)&&!a.EpisodeCountKnown{continue}
  if f.MinEpisodes!=nil&&a.EpisodeCount<*f.MinEpisodes{continue};if f.MaxEpisodes!=nil&&a.EpisodeCount>*f.MaxEpisodes{continue}
  out=append(out,a)
 }
 // Match the PostgreSQL deterministic title/ID ordering before pagination.
 sort.Slice(out,func(i,j int)bool{if f.Sort=="NEWEST"||f.Sort=="OLDEST"{a,b:=out[i].ReleaseYear,out[j].ReleaseYear;if a!=b{if a==0{return false};if b==0{return true};if f.Sort=="NEWEST"{return a>b};return a<b}};if out[i].Title!=out[j].Title{return out[i].Title<out[j].Title};return out[i].ID<out[j].ID})
 if offset>=len(out){return []Anime{},nil}
 out=out[offset:];if len(out)>limit{out=out[:limit]}
 return out,nil
}
