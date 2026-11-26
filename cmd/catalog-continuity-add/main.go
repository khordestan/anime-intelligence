// catalog-continuity-add previews or applies an approved prequel-to-sequel link.
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
	provider := flag.String("provider", "", "approved provider key")
	reference := flag.String("reference", "", "public review reference")
	apply := flag.Bool("apply", false, "apply the reviewed link")
	flag.Parse()
	dsn := os.Getenv("CATALOG_CONTINUITY_DATABASE_URL")
	if flag.NArg() != 0 || dsn == "" {
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
	var id pgtype.UUID
	if err := conn.QueryRow(ctx, `SELECT public.review_continuity_addition($1,$2,$3,$4,$5)`,
		first, second, *provider, *reference, *apply).Scan(&id); err != nil {
		return err
	}
	return json.NewEncoder(os.Stdout).Encode(struct {
		Prequel    string      `json:"prequelId"`
		Sequel     string      `json:"sequelId"`
		RelationID pgtype.UUID `json:"relationId"`
		Applied    bool        `json:"applied"`
	}{*prequel, *sequel, id, *apply})
}

func main() {
	if run() != nil {
		fmt.Fprintln(os.Stderr, "catalog continuity review failed")
		os.Exit(1)
	}
}
