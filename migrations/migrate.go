// Package migrations embeds the reviewed schema and applies it transactionally.
package migrations

import (
	"context"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"fmt"
	"io/fs"
	"sort"

	"github.com/jackc/pgx/v5"
)

//go:embed *.up.sql
var scripts embed.FS

// Apply uses a dedicated connection and one transaction for schema and ledger.
// Migration credentials must be separate from the restricted API runtime role.
func Apply(ctx context.Context, conn *pgx.Conn) error {
	tx, err := conn.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(742019683)`); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations(name text PRIMARY KEY,sha256 text NOT NULL,applied_at timestamptz NOT NULL DEFAULT now())`); err != nil {
		return err
	}
	names, err := fs.Glob(scripts, "*.up.sql")
	if err != nil {
		return err
	}
	sort.Strings(names)
	rows, err := tx.Query(ctx, `SELECT name,sha256 FROM schema_migrations ORDER BY name`)
	if err != nil {
		return err
	}
	applied := map[string]string{}
	for rows.Next() {
		var n, h string
		if err = rows.Scan(&n, &h); err != nil {
			rows.Close()
			return err
		}
		applied[n] = h
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return err
	}
	known := map[string]bool{}
	for _, n := range names {
		known[n] = true
	}
	for n := range applied {
		if !known[n] {
			return fmt.Errorf("unknown applied migration %s", n)
		}
	}
	missing := false
	for _, name := range names {
		sql, err := scripts.ReadFile(name)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(sql)
		digest := hex.EncodeToString(sum[:])
		if old, ok := applied[name]; ok {
			if old != digest {
				return fmt.Errorf("migration checksum mismatch: %s", name)
			}
			if missing {
				return fmt.Errorf("migration history has a gap before %s", name)
			}
			continue
		}
		missing = true
		if _, err = tx.Exec(ctx, string(sql)); err != nil {
			return fmt.Errorf("apply %s: %w", name, err)
		}
		if _, err = tx.Exec(ctx, `INSERT INTO schema_migrations(name,sha256) VALUES($1,$2)`, name, digest); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}
