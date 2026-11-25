// release-email-status reads aggregate scheduler state without starting a worker.
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

	"github.com/khordestan/anime-intelligence/internal/platform/metrics"
	"github.com/khordestan/anime-intelligence/internal/platform/postgres"
)

func main() { os.Exit(run(os.Args[1:], os.Getenv("DATABASE_URL"), os.Stdout, os.Stderr)) }

const usage = "usage: release-email-status [-format=json|prometheus] [-output-file=/absolute/path.prom]"

func run(args []string, dsn string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("release-email-status", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	format := flags.String("format", "json", "output format: json or prometheus")
	outputFile := flags.String("output-file", "", "atomic Prometheus textfile destination")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			if _, err := fmt.Fprintln(stdout, usage); err == nil {
				return 0
			}
			fmt.Fprintln(stderr, "release email diagnostics output failed")
			return 1
		}
		fmt.Fprintln(stderr, usage)
		return 1
	}
	if flags.NArg() != 0 || (*format != "json" && *format != "prometheus") ||
		(*outputFile != "" && (*format != "prometheus" || !metrics.ValidTextfilePath(*outputFile))) {
		fmt.Fprintln(stderr, usage)
		return 1
	}
	if dsn == "" {
		fmt.Fprintln(stderr, "release email diagnostics require DATABASE_URL")
		return 1
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	db, err := postgres.Open(ctx, dsn)
	if err != nil {
		fmt.Fprintln(stderr, "release email diagnostics database initialization failed")
		return 1
	}
	defer db.Close()
	state, err := db.ReleaseEmailDiagnostics(ctx)
	if err != nil {
		fmt.Fprintln(stderr, "release email diagnostics query failed")
		return 1
	}
	switch {
	case *outputFile != "":
		err = metrics.WriteTextfile(*outputFile, func(w io.Writer) error {
			return metrics.WriteReleaseEmail(w, state)
		})
	case *format == "prometheus":
		err = metrics.WriteReleaseEmail(stdout, state)
	default:
		err = json.NewEncoder(stdout).Encode(state)
	}
	if err != nil {
		fmt.Fprintln(stderr, "release email diagnostics output failed")
		return 1
	}
	return 0
}
