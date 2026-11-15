package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/khordestan/anime-intelligence/internal/platform/password"
)

// AccountExport contains personal product data only. Credential hashes,
// session digests, lockout metadata, and other accounts are never included.
type AccountExport struct {
	SchemaVersion int                        `json:"schemaVersion"`
	ExportedAt    time.Time                  `json:"exportedAt"`
	User          User                       `json:"user"`
	Data          map[string]json.RawMessage `json:"data"`
}

func (d *DB) ExportAccount(ctx context.Context, token string) (AccountExport, error) {
	var out AccountExport
	var err error
	for range 3 {
		out, err = d.exportAccountSnapshot(ctx, token)
		var conflict *pgconn.PgError
		if !errors.As(err, &conflict) || (conflict.Code != "40001" && conflict.Code != "40P01") || ctx.Err() != nil {
			return out, err
		}
	}
	return out, err
}
func (d *DB) exportAccountSnapshot(ctx context.Context, token string) (AccountExport, error) {
	// One repeatable-read snapshot keeps tracker/progress/rating data consistent
	// even when another session updates the account while export is running.
	tx, u, err := d.sessionTxOptions(ctx, token, pgx.TxOptions{IsoLevel: pgx.RepeatableRead})
	if err != nil {
		return AccountExport{}, err
	}
	defer rollback(tx)
	out := AccountExport{SchemaVersion: 1, User: u, Data: map[string]json.RawMessage{}}
	if err = tx.QueryRow(ctx, `SELECT transaction_timestamp()`).Scan(&out.ExportedAt); err != nil {
		return AccountExport{}, err
	}
	for _, table := range []struct{ key, name string }{
		{"tracker", "user_anime"}, {"ratings", "user_ratings"}, {"progress", "watch_progress"},
		{"preferences", "user_preferences"}, {"recommendations", "recommendations"},
		{"feedback", "recommendation_feedback"}, {"follows", "user_follows"}, {"notifications", "notifications"},
	} {
		var data []byte
		// The identifiers come only from this fixed server-owned allowlist.
		err = tx.QueryRow(ctx, `SELECT COALESCE(jsonb_agg(to_jsonb(item) ORDER BY item.id),'[]'::jsonb) FROM (SELECT * FROM `+pgx.Identifier{table.name}.Sanitize()+` WHERE user_id=$1) item`, u.ID).Scan(&data)
		if err != nil {
			return AccountExport{}, err
		}
		out.Data[table.key] = data
	}
	return out, tx.Commit(ctx)
}

// DeleteAccount requires both a current authenticated session and the current
// password. The database's foreign keys cascade all user-owned records.
func (d *DB) DeleteAccount(ctx context.Context, token, value string) error {
	if len(value) > password.MaxBytes {
		return ErrReauthentication
	}
	tx, u, err := d.sessionTxWithUserLock(ctx, token, pgx.TxOptions{}, true)
	if err != nil {
		return err
	}
	defer rollback(tx)
	var encoded *string
	err = tx.QueryRow(ctx, `SELECT password_hash FROM users WHERE id=$1`, u.ID).Scan(&encoded)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrUnauthorized
	}
	if err != nil {
		return err
	}
	hash := d.dummyHash
	if encoded != nil {
		hash = *encoded
	}
	if !password.Verify(hash, value) || encoded == nil {
		return denyReauthentication(ctx, tx, u.ID)
	}
	tag, err := tx.Exec(ctx, `DELETE FROM users WHERE id=$1 AND password_hash=$2`, u.ID, hash)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return ErrUnauthorized
	}
	return tx.Commit(ctx)
}

var ErrReauthentication = errors.New("current password is incorrect")

// ChangePassword requires reauthentication and atomically invalidates every
// credential issued before the password change, including the caller's.
func (d *DB) ChangePassword(ctx context.Context, token, current, next string) error {
	if len(current) > password.MaxBytes {
		return ErrReauthentication
	}
	tx, u, err := d.sessionTxWithUserLock(ctx, token, pgx.TxOptions{}, true)
	if err != nil {
		return err
	}
	defer rollback(tx)
	var encoded *string
	if err = tx.QueryRow(ctx, `SELECT password_hash FROM users WHERE id=$1`, u.ID).Scan(&encoded); err != nil {
		return err
	}
	hash := d.dummyHash
	if encoded != nil {
		hash = *encoded
	}
	if !password.Verify(hash, current) || encoded == nil {
		return denyReauthentication(ctx, tx, u.ID)
	}
	replacement, err := password.Hash(next)
	if err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `UPDATE users SET password_hash=$2,updated_at=now(),failed_login_count=0,login_failure_started_at=NULL,locked_until=NULL WHERE id=$1`, u.ID, replacement); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `UPDATE auth_sessions SET revoked_at=now() WHERE user_id=$1 AND revoked_at IS NULL`, u.ID); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `UPDATE auth_challenges SET revoked_at=now() WHERE user_id=$1 AND consumed_at IS NULL AND revoked_at IS NULL`, u.ID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// The operation already holds the exclusive account lock. Persist shared
// failure state before returning; a rollback must never erase a charged guess.
func denyReauthentication(ctx context.Context, tx pgx.Tx, userID string) error {
	if err := recordPasswordFailure(ctx, tx, userID); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	return ErrReauthentication
}
