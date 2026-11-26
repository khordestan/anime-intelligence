// catalog-merge-placeholder folds a reviewed duplicate with no dependent records into an active title.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

func run() error {
	retired := flag.String("retired", "", "canonical placeholder UUID")
	survivor := flag.String("survivor", "", "canonical survivor UUID")
	reference := flag.String("reference", "", "public review reference")
	apply := flag.Bool("apply", false, "apply the reviewed merge")
	var ids []pgtype.UUID
	flag.Func("expected-mapping-id", "mapping UUID from preview, repeat for each ID", func(value string) error {
		var id pgtype.UUID
		if err := id.Scan(value); err != nil || len(ids) >= 100 {
			return errors.New("use up to 100 valid mapping UUIDs")
		}
		ids = append(ids, id)
		return nil
	})
	flag.Parse()
	dsn := os.Getenv("CATALOG_PLACEHOLDER_MERGE_DATABASE_URL")
	if flag.NArg() != 0 || dsn == "" {
		return errors.New("missing command inputs")
	}
	var first, second pgtype.UUID
	if first.Scan(*retired) != nil || second.Scan(*survivor) != nil {
		return errors.New("invalid canonical UUID")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		return err
	}
	defer conn.Close(ctx)
	if ids == nil {
		ids = []pgtype.UUID{}
	}
	var found []pgtype.UUID
	if err := conn.QueryRow(ctx, `SELECT public.review_catalog_placeholder_merge($1,$2,$3,$4,$5)`,
		first, second, ids, *reference, *apply).Scan(&found); err != nil {
		return err
	}
	return json.NewEncoder(os.Stdout).Encode(struct {
		Retired    string        `json:"retiredId"`
		Survivor   string        `json:"survivorId"`
		MappingIDs []pgtype.UUID `json:"mappingIds"`
		Applied    bool          `json:"applied"`
	}{*retired, *survivor, found, *apply})
}

func main() {
	if run() != nil {
		fmt.Fprintln(os.Stderr, "catalog placeholder merge review failed")
		os.Exit(1)
	}
}
