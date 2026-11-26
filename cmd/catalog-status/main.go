package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/khordestan/anime-intelligence/internal/platform/catalogsync"
	"github.com/khordestan/anime-intelligence/internal/platform/metrics"
)

func run(args []string, dsn string, stdout io.Writer) error {
	flags := flag.NewFlagSet("catalog-status", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	stale := flags.Duration("stale-after", 24*time.Hour, "Freshness threshold, from one minute to thirty days")
	format := flags.String("format", "json", "Output format: json or prometheus")
	outputFile := flags.String("output-file", "", "atomic Prometheus textfile destination")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			_, err := fmt.Fprintln(stdout, "usage: catalog-status [-stale-after=24h] [-format=json|prometheus] [-output-file=/absolute/path.prom]")
			return err
		}
		return catalogsync.ErrStatus
	}
	if flags.NArg() != 0 || (*outputFile != "" && (*format != "prometheus" || !metrics.ValidTextfilePath(*outputFile))) {
		return catalogsync.ErrStatus
	}
	if (*format != "json" && *format != "prometheus") || *stale < time.Minute || *stale > 30*24*time.Hour {
		return catalogsync.ErrStatus
	}

	if dsn == "" {
		return catalogsync.ErrStatus
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	config, err := pgx.ParseConfig(dsn)
	if err != nil {
		return catalogsync.ErrStatus
	}
	config.ConnectTimeout = 5 * time.Second
	config.RuntimeParams["search_path"] = "pg_catalog,public"
	conn, err := pgx.ConnectConfig(ctx, config)
	if err != nil {
		return catalogsync.ErrStatus
	}
	defer func() {
		cleanup, stop := context.WithTimeout(context.Background(), 5*time.Second)
		defer stop()
		_ = conn.Close(cleanup)
	}()
	if catalogsync.CheckWriter(ctx, conn) != nil {
		return catalogsync.ErrStatus
	}
	status, err := catalogsync.Inspect(ctx, conn, time.Now(), *stale)
	if err != nil {
		return err
	}
	if *format == "prometheus" {
		text, err := catalogsync.Prometheus(status)
		if err != nil {
			return err
		}
		if *outputFile != "" {
			return metrics.WriteTextfile(*outputFile, func(w io.Writer) error {
				_, err := io.WriteString(w, text)
				return err
			})
		}
		_, err = io.WriteString(stdout, text)
		return err
	}
	return json.NewEncoder(stdout).Encode(status)
}
func main() {
	if run(os.Args[1:], os.Getenv("CATALOG_DATABASE_URL"), os.Stdout) != nil {
		fmt.Fprintln(os.Stderr, "catalog status unavailable")
		os.Exit(1)
	}
}
