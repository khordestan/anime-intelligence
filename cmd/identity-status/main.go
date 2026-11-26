// identity-status reports queue aggregates without private data through a restricted DB
// connection. It does not start workers, decrypt envelopes, or change records.
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

const usage = "usage: identity-status [-format=json|prometheus] [-output-file=/absolute/path.prom] [-payload-cutoff=RFC3339]"

func run(args []string, dsn string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("identity-status", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	format := flags.String("format", "json", "snapshot output format")
	var cutoff time.Time
	flags.Func("payload-cutoff", "count retained encrypted rows created at or before a database-time cutoff", func(value string) error {
		parsed, err := time.Parse(time.RFC3339Nano, value)
		if err != nil || parsed.IsZero() {
			return postgres.ErrIdentityPayloadCutoff
		}
		cutoff = parsed
		return nil
	})
	outputFile := flags.String("output-file", "", "atomic Prometheus textfile destination")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			if _, writeErr := fmt.Fprintln(stdout, usage); writeErr != nil {
				fmt.Fprintln(stderr, "identity queue diagnostics output failed")
				return 1
			}
			return 0
		}
		fmt.Fprintln(stderr, usage)
		return 1
	}
	if flags.NArg() != 0 || (!cutoff.IsZero() && *format != "json") || (*format != "json" && *format != "prometheus") || (*outputFile != "" && (*format != "prometheus" || !metrics.ValidTextfilePath(*outputFile))) {
		fmt.Fprintln(stderr, usage)
		return 1
	}

	if dsn == "" {
		fmt.Fprintln(stderr, "identity queue diagnostics require DATABASE_URL")
		return 1
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	db, err := postgres.Open(ctx, dsn)
	if err != nil {
		fmt.Fprintln(stderr, "identity queue diagnostics database initialization failed")
		return 1
	}
	defer db.Close()
	var snapshot postgres.QueueDiagnostics
	if cutoff.IsZero() {
		snapshot, err = db.IdentityQueueDiagnostics(ctx)
	} else {
		snapshot, err = db.IdentityQueueDiagnosticsThrough(ctx, cutoff)
	}
	if err != nil {
		fmt.Fprintln(stderr, "identity queue diagnostics query failed")
		return 1
	}
	if *outputFile != "" {
		err = writeSnapshotFile(*outputFile, snapshot)
	} else {
		err = writeSnapshot(stdout, *format, snapshot)
	}
	if err != nil {
		fmt.Fprintln(stderr, "identity queue diagnostics output failed")
		return 1
	}
	return 0
}

func writeSnapshot(w io.Writer, format string, snapshot postgres.QueueDiagnostics) error {
	switch format {
	case "json":
		return json.NewEncoder(w).Encode(snapshot)
	case "prometheus":
		return metrics.WriteIdentity(w, snapshot)
	default:
		return errors.New("unsupported diagnostic output format")
	}
}
