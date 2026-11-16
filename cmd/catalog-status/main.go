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
func run() error {
	stale:=flag.Duration("stale-after",24*time.Hour,"Freshness threshold, from one minute to thirty days")
	format:=flag.String("format","json","Output format: json or prometheus")
	flag.Parse()
 if *format!="json"&&*format!="prometheus"{return catalogsync.ErrStatus}
	dsn:=os.Getenv("CATALOG_DATABASE_URL")
	if dsn=="" {return catalogsync.ErrStatus}
	ctx,cancel:=context.WithTimeout(context.Background(),10*time.Second)
	defer cancel()
	conn,err:=pgx.Connect(ctx,dsn)
	if err!=nil {return catalogsync.ErrStatus}
	defer conn.Close(context.Background())
	status,err:=catalogsync.Inspect(ctx,conn,time.Now(),*stale)
	if err!=nil {return err}
 if *format=="prometheus"{text,err:=catalogsync.Prometheus(status);if err!=nil{return err};_,err=fmt.Fprint(os.Stdout,text);return err}
	return json.NewEncoder(os.Stdout).Encode(status)
}
func main() {
	if run()!=nil {fmt.Fprintln(os.Stderr,"catalog status unavailable");os.Exit(1)}
}
