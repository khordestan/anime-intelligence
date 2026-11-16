package postgres_test
import("context";"testing";"github.com/khordestan/anime-intelligence/internal/platform/store")
func TestEpisodeCountRangeDistinguishesUnknownAndZero(t *testing.T){
 db,owner,_,_:=testDB(t);ctx:=context.Background()
 if _,err:=owner.Exec(ctx,`INSERT INTO anime(canonical_title,episode_count) VALUES('Synthetic Range Unknown',NULL),('Synthetic Range Zero',0),('Synthetic Range Twelve',12),('Synthetic Range Twenty',20)`);err!=nil{t.Fatal(err)}
 zero,twelve:=0,12
 for _,spec:=range []struct{filter store.CatalogFilter;want int}{
 {store.CatalogFilter{MinEpisodes:&zero},3},{store.CatalogFilter{MaxEpisodes:&zero},1},{store.CatalogFilter{MinEpisodes:&twelve,MaxEpisodes:&twelve},1},{store.CatalogFilter{MaxEpisodes:&twelve},2},
 }{
 spec.filter.Query="Synthetic Range";items,err:=db.Repository("").FilterCatalog(ctx,spec.filter,100,0)
 if err!=nil||len(items)!=spec.want{t.Fatal("range results",len(items),spec.want,err)}
 }
 if _,err:=db.Repository("").FilterCatalog(ctx,store.CatalogFilter{MinEpisodes:&twelve,MaxEpisodes:&zero},24,0);err!=store.ErrCatalogFilter{t.Fatal("reversed range accepted")}
}
