// catalog-retract retires reviewed unused public identities, never canonical entities.
package main

import (
 "context"
 "encoding/json"
 "flag"
 "fmt"
 "os"
 "time"

 "github.com/jackc/pgx/v5"
 "github.com/khordestan/anime-intelligence/internal/platform/catalogidentity"
 "github.com/khordestan/anime-intelligence/internal/platform/catalogsync"
)
func run()error{
 input:=flag.String("input","","Reviewed identity retractions JSON (required)")
 apply:=flag.Bool("apply",false,"Persist reviewed retractions; default validates and rolls back")
 flag.Parse()
 f,err:=os.Open(*input);if err!=nil{return catalogidentity.ErrInvalid};defer f.Close()
 now:=time.Now().UTC();snapshot,err:=catalogidentity.DecodeRetractions(f,now);if err!=nil{return err}
 dsn:=os.Getenv("CATALOG_RETRACTION_DATABASE_URL");if dsn==""{return catalogidentity.ErrApply}
 ctx,cancel:=context.WithTimeout(context.Background(),time.Minute);defer cancel()
 config,err:=pgx.ParseConfig(dsn);if err!=nil{return catalogidentity.ErrApply}
 config.ConnectTimeout=5*time.Second;config.RuntimeParams["search_path"]="pg_catalog,public"
 conn,err:=pgx.ConnectConfig(ctx,config);if err!=nil{return catalogidentity.ErrApply}
 defer func(){cleanup,stop:=context.WithTimeout(context.Background(),5*time.Second);defer stop();_=conn.Close(cleanup)}()
 if catalogsync.CheckWriter(ctx,conn)!=nil{return catalogidentity.ErrApply}
 result,err:=catalogidentity.Retract(ctx,conn,snapshot,now,*apply);if err!=nil{return err}
 return json.NewEncoder(os.Stdout).Encode(result)
}
func main(){if run()!=nil{fmt.Fprintln(os.Stderr,"catalog identity retraction failed");os.Exit(1)}}
