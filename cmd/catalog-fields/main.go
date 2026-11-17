// catalog-fields previews or applies reviewed public label-field choices.
package main

import (
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
 input:=flag.String("input","","Reviewed field-choice manifest")
 apply:=flag.Bool("apply",false,"Commit reviewed choices; default rolls back")
 flag.Parse()
 if *input==""||flag.NArg()!=0{return catalogsync.ErrFieldReview}
 file,err:=os.Open(*input);if err!=nil{return catalogsync.ErrFieldReview};defer file.Close()
 now:=time.Now().UTC()
 review,err:=catalogsync.DecodeFieldReview(file,now);if err!=nil{return err}
 dsn:=os.Getenv("CATALOG_FIELD_DATABASE_URL");if dsn==""{return catalogsync.ErrFieldApply}
 ctx,cancel:=context.WithTimeout(context.Background(),time.Minute);defer cancel()
 config,err:=pgx.ParseConfig(dsn);if err!=nil{return catalogsync.ErrFieldApply}
 config.ConnectTimeout=5*time.Second;config.RuntimeParams["search_path"]="pg_catalog,public"
 conn,err:=pgx.ConnectConfig(ctx,config);if err!=nil{return catalogsync.ErrFieldApply}
 defer func(){cleanup,stop:=context.WithTimeout(context.Background(),5*time.Second);defer stop();_=conn.Close(cleanup)}()
 result,err:=catalogsync.ReviewFields(ctx,conn,review,now,*apply);if err!=nil{return err}
 return json.NewEncoder(os.Stdout).Encode(result)
}
func main(){if run()!=nil{fmt.Fprintln(os.Stderr,"catalog field review failed");os.Exit(1)}}
