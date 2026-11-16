package store_test
import("context";"errors";"testing";"github.com/khordestan/anime-intelligence/internal/platform/store")
func TestDirectLocalCatalogBoundsAndCancellation(t *testing.T){
 s,err:=store.New("");if err!=nil{t.Fatal(err)};repo:=store.Local{Store:s}
 for _,bounds:=range [][2]int{{0,0},{101,0},{1,-1},{1,100001}}{
 if _,err:=repo.Catalog(context.Background(),"",bounds[0],bounds[1]);!errors.Is(err,store.ErrCatalogFilter){t.Fatal("direct catalog bypassed bounds",bounds,err)}
 }
 ctx,cancel:=context.WithCancel(context.Background());cancel()
 if _,err:=repo.Catalog(ctx,"",24,0);!errors.Is(err,context.Canceled){t.Fatal("cancelled direct read",err)}
 if _,err:=repo.FilterCatalog(ctx,store.CatalogFilter{Genre:"Missing"},24,0);!errors.Is(err,context.Canceled){t.Fatal("cancelled sparse read",err)}
 items,err:=repo.Catalog(context.Background(),"solo",24,0)
 if err!=nil||len(items)!=1||items[0].Title!="Solo Leveling"{t.Fatal("ordinary search changed",items,err)}
}
