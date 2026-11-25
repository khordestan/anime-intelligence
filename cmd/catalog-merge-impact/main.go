// catalog-merge-impact inventories two canonical titles in a read-only operator session.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/khordestan/anime-intelligence/internal/platform/catalogidentity"
)

func run() error {
	sourceID := flag.String("source", "", "canonical UUID proposed for retirement")
	targetID := flag.String("target", "", "canonical UUID proposed to keep")
	flag.Parse()
	var source, target pgtype.UUID
	dsn := os.Getenv("CATALOG_MERGE_REVIEW_DATABASE_URL")
	if flag.NArg() != 0 || dsn == "" || source.Scan(*sourceID) != nil || target.Scan(*targetID) != nil ||
		!source.Valid || !target.Valid || source == target {
		return catalogidentity.ErrMergeImpact
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		return err
	}
	defer conn.Close(ctx)
	result, err := catalogidentity.InspectMergeImpact(ctx, conn, source, target)
	if err != nil {
		return err
	}
	return json.NewEncoder(os.Stdout).Encode(result)
}

func main() {
	if run() != nil {
		fmt.Fprintln(os.Stderr, "catalog merge impact review failed")
		os.Exit(1)
	}
}
