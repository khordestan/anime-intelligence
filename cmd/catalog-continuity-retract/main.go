// catalog-continuity-retract previews or applies reviewed continuity withdrawals.
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
	prequel := flag.String("prequel", "", "canonical prequel UUID")
	sequel := flag.String("sequel", "", "canonical sequel UUID")
	reference := flag.String("reference", "", "public review reference")
	apply := flag.Bool("apply", false, "apply the reviewed retraction")
	var ids []pgtype.UUID
	flag.Func("expected-id", "relation UUID from preview, repeat for each ID", func(value string) error {
		var id pgtype.UUID
		if err := id.Scan(value); err != nil || len(ids) >= 2 {
			return errors.New("use up to two valid relation UUIDs")
		}
		ids = append(ids, id)
		return nil
	})
	flag.Parse()
	dsn := os.Getenv("CATALOG_CONTINUITY_DATABASE_URL")
	if flag.NArg() != 0 || dsn == "" || (*apply && len(ids) == 0) {
		return errors.New("missing command inputs")
	}
	var first, second pgtype.UUID
	if first.Scan(*prequel) != nil || second.Scan(*sequel) != nil {
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
	if err := conn.QueryRow(ctx, `SELECT public.review_continuity_retraction($1,$2,$3,$4,$5)`,
		first, second, ids, *reference, *apply).Scan(&found); err != nil {
		return err
	}
	return json.NewEncoder(os.Stdout).Encode(struct {
		Prequel     string        `json:"prequelId"`
		Sequel      string        `json:"sequelId"`
		RelationIDs []pgtype.UUID `json:"relationIds"`
		Applied     bool          `json:"applied"`
	}{*prequel, *sequel, found, *apply})
}

func main() {
	if run() != nil {
		fmt.Fprintln(os.Stderr, "catalog continuity review failed")
		os.Exit(1)
	}
}
