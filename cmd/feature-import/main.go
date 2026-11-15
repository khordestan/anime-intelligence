package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/khordestan/anime-intelligence/internal/platform/curation"
)
func run()error{
	input:=flag.String("input","","Reviewed public feature snapshot JSON")
	flag.Parse()
	f,err:=os.Open(*input);if err!=nil{return curation.ErrInvalid};defer f.Close()
	now:=time.Now().UTC()
	s,err:=curation.Decode(f,now);if err!=nil{return err}
	dsn:=os.Getenv("CURATION_DATABASE_URL");if dsn==""{return curation.ErrImport}
	ctx,cancel:=context.WithTimeout(context.Background(),time.Minute);defer cancel()
	conn,err:=pgx.Connect(ctx,dsn);if err!=nil{return curation.ErrImport};defer conn.Close(context.Background())
	result,err:=curation.Apply(ctx,conn,s,now);if err!=nil{return err}
	return json.NewEncoder(os.Stdout).Encode(result)
}
func main(){if run()!=nil{fmt.Fprintln(os.Stderr,"curated feature import failed");os.Exit(1)}}
