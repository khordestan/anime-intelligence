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
	flag.Parse()
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
	conn,err:=pgx.Connect(ctx,dsn)
	if err!=nil {return catalogsync.ErrImport}
	defer conn.Close(context.Background())
	result,err:=catalogsync.Import(ctx,conn,snapshot,now)
	if err!=nil {return err}
	return json.NewEncoder(os.Stdout).Encode(result)
}
func main() {
	if run()!=nil {fmt.Fprintln(os.Stderr,"catalog import failed");os.Exit(1)}
}
