package postgres_test
import("context";"io";"log/slog";"net/http/httptest";"testing";"github.com/khordestan/anime-intelligence/internal/httpapi")
func TestCatalogFacetDiscoveryBoundariesAndCategories(t *testing.T){
 db,owner,_,_:=testDB(t);ctx:=context.Background()
 if _,err:=owner.Exec(ctx,`INSERT INTO data_sources(provider_key,display_name) VALUES('facet-fixture','Synthetic facets');
 INSERT INTO catalog_metadata(anime_id,source_id,observed_at,payload) SELECT '10000000-0000-4000-8000-000000000001',id,now(),'{"genres":["Action","action","Adventure","100% Literal"],"themes":["Strategy"],"tags":["Not a theme"]}'::jsonb FROM data_sources WHERE provider_key='facet-fixture'`);err!=nil{t.Fatal(err)}
 repo:=db.Repository("")
 page,err:=repo.CatalogFacets(ctx,"genres","A",1)
 if err!=nil||len(page.Items)!=1||page.Items[0]!="Action"||!page.HasMore{t.Fatal(page,err)}
 page,err=repo.CatalogFacets(ctx,"genres","A",2)
 if err!=nil||len(page.Items)!=2||page.HasMore{t.Fatal(page,err)}
 page,err=repo.CatalogFacets(ctx,"themes","",100)
 if err!=nil||len(page.Items)!=1||page.Items[0]!="Strategy"{t.Fatal("category inference",page,err)}
 page,err=repo.CatalogFacets(ctx,"genres","100%",50)
 if err!=nil||len(page.Items)!=1{t.Fatal("literal prefix",page,err)}
 page,err=repo.CatalogFacets(ctx,"studios","",50)
 if err!=nil||page.Items==nil||len(page.Items)!=0{t.Fatal("missing source labels",page,err)}
 h:=httpapi.NewPostgresRouter(slog.New(slog.NewTextHandler(io.Discard,nil)),db)
 for query,want:=range map[string]int{"?field=genres":200,"?field=tags":400,"?field=genres&limit=101":400,"?field=genres&field=themes":400,"?field=genres&prefix=%00":400}{
 w:=httptest.NewRecorder();h.ServeHTTP(w,httptest.NewRequest("GET","/api/v1/anime/facets"+query,nil));if w.Code!=want||w.Header().Get("Cache-Control")!="no-store"{t.Fatal(query,w.Code)}
 }
}
