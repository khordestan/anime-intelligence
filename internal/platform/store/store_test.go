package store
import("path/filepath";"testing")
func TestLibraryPersistsAcrossRestart(t *testing.T) {
 path:=filepath.Join(t.TempDir(),"state","local.json");first,err:=New(path);if err!=nil{t.Fatal(err)}
 anime,ok:=first.GetAnime("10000000-0000-4000-8000-000000000001");if !ok{t.Fatal("seed catalog entry missing")}
 entry,err:=first.Add(anime.ID,"PLANNING",0);if err!=nil{t.Fatal(err)}
 if _,ok,err=first.Rate(anime.ID,5);err!=nil||!ok{t.Fatalf("rate: ok=%v err=%v",ok,err)}
 second,err:=New(path);if err!=nil{t.Fatal(err)};items:=second.Library()
 if len(items)!=1{t.Fatalf("want 1 library item, got %d",len(items))}
 if items[0].ID!=entry.ID||items[0].Rating!=5{t.Fatalf("unexpected persisted entry: %+v",items[0])}
}
