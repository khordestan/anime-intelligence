// identity-status reports queue aggregates without private data through a restricted DB
// connection. It does not start workers, decrypt envelopes, or change records.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/khordestan/anime-intelligence/internal/platform/postgres"
)

func main() { os.Exit(run(os.Getenv("DATABASE_URL"), os.Stdout, os.Stderr)) }
func run(dsn string, stdout, stderr io.Writer) int {
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
	snapshot, err := db.IdentityQueueDiagnostics(ctx)
	if err != nil {
		fmt.Fprintln(stderr, "identity queue diagnostics query failed")
		return 1
	}
	if err = json.NewEncoder(stdout).Encode(snapshot); err != nil {
		fmt.Fprintln(stderr, "identity queue diagnostics output failed")
		return 1
	}
	return 0
}
