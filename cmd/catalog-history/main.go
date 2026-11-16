// catalog-history prints field names and observation times from reviewed archives.
package main

import(
 "context"
 "encoding/json"
 "flag"
 "fmt"
 "os"
 "time"

 "github.com/jackc/pgx/v5"
 "github.com/khordestan/anime-intelligence/internal/platform/catalogsync"
)
func run()error{
 provider:=flag.String("provider","","Exact provider key")
 external:=flag.String("external-id","","Exact provider title ID")
 limit:=flag.Int("limit",20,"Maximum observations (1-100)")
 beforeText:=flag.String("before","","Exclusive observation cursor (RFC3339)")
 flag.Parse()
 var before *time.Time
 if *beforeText!=""{at,err:=time.Parse(time.RFC3339Nano,*beforeText);if err!=nil{return catalogsync.ErrHistory};before=&at}
 dsn:=os.Getenv("CATALOG_HISTORY_DATABASE_URL");if dsn==""{return catalogsync.ErrHistory}
 ctx,cancel:=context.WithTimeout(context.Background(),10*time.Second);defer cancel()
 config,err:=pgx.ParseConfig(dsn);if err!=nil{return catalogsync.ErrHistory}
 config.ConnectTimeout=5*time.Second;config.RuntimeParams["search_path"]="pg_catalog,public"
 conn,err:=pgx.ConnectConfig(ctx,config);if err!=nil{return catalogsync.ErrHistory}
 defer func(){cleanup,stop:=context.WithTimeout(context.Background(),5*time.Second);defer stop();_=conn.Close(cleanup)}()
 if catalogsync.CheckWriter(ctx,conn)!=nil{return catalogsync.ErrHistory}
 result,err:=catalogsync.History(ctx,conn,*provider,*external,*limit,before);if err!=nil{return err}
 return json.NewEncoder(os.Stdout).Encode(result)
}
func main(){if run()!=nil{fmt.Fprintln(os.Stderr,"catalog history unavailable");os.Exit(1)}}
