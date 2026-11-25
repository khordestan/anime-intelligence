// release-email-status reads aggregate scheduler state without starting a worker.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/khordestan/anime-intelligence/internal/platform/metrics"
	"github.com/khordestan/anime-intelligence/internal/platform/postgres"
)

func main() {
	format := flag.String("format", "json", "output format: json or prometheus")
	flag.Parse()
	dsn := os.Getenv("DATABASE_URL")
	if flag.NArg() != 0 || dsn == "" || (*format != "json" && *format != "prometheus") {
		fmt.Fprintln(os.Stderr, "release-email-status requires DATABASE_URL and accepts -format=json|prometheus")
		os.Exit(1)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	db, err := postgres.Open(ctx, dsn)
	if err != nil {
		fmt.Fprintln(os.Stderr, "release email diagnostics database initialization failed")
		os.Exit(1)
	}
	defer db.Close()
	state, err := db.ReleaseEmailDiagnostics(ctx)
	if err != nil {
		fmt.Fprintln(os.Stderr, "release email diagnostics query failed")
		os.Exit(1)
	}
	if *format == "prometheus" {
		err = metrics.WriteReleaseEmail(os.Stdout, state)
	} else {
		err = json.NewEncoder(os.Stdout).Encode(state)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "release email diagnostics output failed")
		os.Exit(1)
	}
}
