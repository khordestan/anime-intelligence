package postgres_test
import("context";"testing";"github.com/khordestan/anime-intelligence/internal/platform/store")
func TestCatalogYearOrderingKeepsUnknownLast(t *testing.T){
 db,owner,_,_:=testDB(t);ctx:=context.Background()
 if _,err:=owner.Exec(ctx,`INSERT INTO anime(canonical_title,release_year) VALUES('Synthetic Sort Unknown',NULL),('Synthetic Sort Older',2000),('Synthetic Sort Newer',2025),('Synthetic Sort Another',2025)`);err!=nil{t.Fatal(err)}
 for _,spec:=range []struct{sort string;want []string}{
 {"NEWEST",[]string{"Synthetic Sort Another","Synthetic Sort Newer","Synthetic Sort Older","Synthetic Sort Unknown"}},
 {"OLDEST",[]string{"Synthetic Sort Older","Synthetic Sort Another","Synthetic Sort Newer","Synthetic Sort Unknown"}},
 }{
 items,err:=db.Repository("").FilterCatalog(ctx,store.CatalogFilter{Query:"Synthetic Sort",Sort:spec.sort},100,0)
 if err!=nil||len(items)!=4{t.Fatal(items,err)}
 for i,want:=range spec.want{if items[i].Title!=want{t.Fatal("year ordering",spec.sort,items)}}
 page,err:=db.Repository("").FilterCatalog(ctx,store.CatalogFilter{Query:"Synthetic Sort",Sort:spec.sort},1,1)
 if err!=nil||len(page)!=1||page[0].Title!=spec.want[1]{t.Fatal("sort before pagination",err)}
 }
 if _,err:=db.Repository("").FilterCatalog(ctx,store.CatalogFilter{Sort:"invalid"},24,0);err!=store.ErrCatalogFilter{t.Fatal("invalid sorting accepted")}
}
