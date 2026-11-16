package postgres_test
import("context";"testing";"github.com/khordestan/anime-intelligence/internal/platform/store")
func TestCatalogReleaseYearRanges(t *testing.T){
 db,owner,_,_:=testDB(t);ctx:=context.Background()
 if _,err:=owner.Exec(ctx,`INSERT INTO anime(canonical_title,release_year) VALUES('Synthetic Years Unknown',NULL),('Synthetic Years Older',1990),('Synthetic Years Middle',2000),('Synthetic Years Newer',2025)`);err!=nil{t.Fatal(err)}
 min,max:=1990,2000
 for _,spec:=range []struct{f store.CatalogFilter;want int}{
 {store.CatalogFilter{MinYear:&min,MaxYear:&max},2},{store.CatalogFilter{MaxYear:&max},2},{store.CatalogFilter{MinYear:&max},2},{store.CatalogFilter{MinYear:&max,MaxYear:&max},1},
 }{
 spec.f.Query="Synthetic Years";items,err:=db.Repository("").FilterCatalog(ctx,spec.f,100,0)
 if err!=nil||len(items)!=spec.want{t.Fatal("year range",len(items),spec.want,err)}
 }
 if _,err:=db.Repository("").FilterCatalog(ctx,store.CatalogFilter{MinYear:&max,MaxYear:&min},24,0);err!=store.ErrCatalogFilter{t.Fatal("invalid year range accepted")}
}
