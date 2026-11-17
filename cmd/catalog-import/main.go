// catalog-import is an explicit offline administrator operation, not a worker.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/khordestan/anime-intelligence/internal/modules/catalog/feed"
	"github.com/khordestan/anime-intelligence/internal/platform/catalogsync"
)
func run() error {
	input:=flag.String("input","","Reviewed snapshot JSON file (required)")
	preview:=flag.Bool("preview",false,"Validate complete database import and roll back all changes")
	archive:=flag.Bool("archive",false,"Retain reviewed structured observations only when historical storage is permitted")
 schedules:=flag.Bool("schedules-only",false,"Import only schedules for pre-mapped canonical titles; preserve all metadata")
 flag.Parse()
 if flag.NArg()!=0||(*schedules&&*archive){return feed.ErrInvalid}
	file,err:=os.Open(*input)
	if err!=nil {return feed.ErrInvalid}
	defer file.Close()
	now:=time.Now().UTC()
	snapshot,err:=feed.Decode(file,now)
	if err!=nil {return err}
	dsn:=os.Getenv("CATALOG_DATABASE_URL")
	if dsn=="" {return catalogsync.ErrImport}
	ctx,cancel:=context.WithTimeout(context.Background(),2*time.Minute)
	defer cancel()
	config,err:=pgx.ParseConfig(dsn)
	if err!=nil{return catalogsync.ErrImport}
	config.ConnectTimeout=5*time.Second;config.RuntimeParams["search_path"]="pg_catalog,public"
	conn,err:=pgx.ConnectConfig(ctx,config)
	if err!=nil {return catalogsync.ErrImport}
	defer func(){cleanup,stop:=context.WithTimeout(context.Background(),5*time.Second);defer stop();_=conn.Close(cleanup)}()
	result,err:=catalogsync.ImportWithOptions(ctx,conn,snapshot,now,catalogsync.ImportOptions{Preview:*preview,Archive:*archive,SchedulesOnly:*schedules})
	if err!=nil {return err}
	return json.NewEncoder(os.Stdout).Encode(result)
}
func main() {
	if run()!=nil {fmt.Fprintln(os.Stderr,"catalog import failed");os.Exit(1)}
}
