//go:build ignore

package main
import("fmt";"os";"time";"github.com/khordestan/anime-intelligence/internal/platform/catalogsync")
func main(){
 text,err:=catalogsync.Prometheus(catalogsync.Status{AsOf:time.Unix(2000,0),StaleAfterSeconds:86400,Titles:8,ObservedTitles:2,UnobservedTitles:6,StaleTitles:1})
 if err!=nil{fmt.Fprintln(os.Stderr,"catalog fixture failed");os.Exit(1)}
 fmt.Print(text)
}
