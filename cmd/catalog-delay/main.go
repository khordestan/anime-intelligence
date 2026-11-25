// catalog-delay publishes reviewed, source-attributed episode delay reports.
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
	var review catalogsync.ReviewedDelay
	flag.StringVar(&review.Provider, "provider", "", "approved source key")
	flag.StringVar(&review.ExternalID, "external-id", "", "exact source title ID")
	flag.StringVar(&review.AnimeID, "anime-id", "", "expected canonical title UUID")
	flag.IntVar(&review.Episode, "episode", 0, "episode number")
	flag.StringVar(&review.Track, "track", "", "exact release track")
	flag.IntVar(&review.ExpectedRevision, "revision", 0, "current reviewed episode revision")
	flag.StringVar(&review.Reference, "reference", "", "public HTTPS notice reference")
	observed := flag.String("observed-at", "", "source observation time in RFC3339")
	apply := flag.Bool("apply", false, "persist reviewed publication")
	flag.Parse()
	dsn := os.Getenv("CATALOG_DATABASE_URL")
	var err error
	review.ObservedAt, err = time.Parse(time.RFC3339, *observed)
	if err != nil || dsn == "" || flag.NArg() != 0 {
		return catalogsync.ErrDelayReview
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	config, err := pgx.ParseConfig(dsn)
	if err != nil {
		return err
	}
	config.ConnectTimeout = 5 * time.Second
	config.RuntimeParams["search_path"] = "pg_catalog,public"
	conn, err := pgx.ConnectConfig(ctx, config)
	if err != nil {
		return err
	}
	defer conn.Close(ctx)
	result, err := catalogsync.PublishReviewedDelay(ctx, conn, review, time.Now(), *apply)
	if err != nil {
		return err
	}
	return json.NewEncoder(os.Stdout).Encode(result)
}

func main() {
	if run() != nil {
		fmt.Fprintln(os.Stderr, "catalog delay review failed")
		os.Exit(1)
	}
}
