package catalogsync
import("strings";"testing";"time")
func TestCatalogPrometheusAggregatesOnly(t *testing.T){
 s:=Status{AsOf:time.Unix(2000,0),StaleAfterSeconds:86400,Titles:8,ObservedTitles:3,UnobservedTitles:5,StaleTitles:2,FutureTitles:1,EpisodeSchedules:4,ConfirmedEpisodes:1,Sources:[]SourceStatus{{Provider:"Synthetic hidden provider"}},Coverage:MetadataCoverage{Genres:2}}
 text,err:=Prometheus(s)
 if err!=nil||!strings.Contains(text,"anime_catalog_metadata_genres 2\n")||!strings.Contains(text,"anime_catalog_unobserved_titles 5\n")||strings.Contains(text,"Synthetic")||strings.Contains(text,"{"){t.Fatal("aggregate export",err)}
 if strings.Count(text,"# TYPE ")!=19{t.Fatal("metric contract changed")}
 for _,mutate:=range []func(*Status){func(s *Status){s.Titles=-1},func(s *Status){s.AsOf=time.Time{}},func(s *Status){s.ConfirmedEpisodes=5},func(s *Status){s.FutureTitles=3},func(s *Status){s.UnobservedTitles=0}}{
 invalid:=s;mutate(&invalid);if text,err:=Prometheus(invalid);err!=ErrStatus||text!=""{t.Fatal("invalid snapshot exported")}
 }
}
