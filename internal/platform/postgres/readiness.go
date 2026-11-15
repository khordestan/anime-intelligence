package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotReady = errors.New("database runtime is not ready")

// Ready checks the live connection role and required security schema without
// reading account data or modifying identity/session state. Errors are fixed.
func (d *DB) Ready(ctx context.Context) error {
	if d == nil || d.pool == nil {
		return ErrNotReady
	}
	if err := checkRuntime(ctx, d.pool); err != nil {
		return ErrNotReady
	}
	return nil
}
func checkRuntime(ctx context.Context, pool *pgxpool.Pool) error {
	connection, err := pool.Acquire(ctx)
	if err != nil {
		return err
	}
	defer connection.Release()
	c := connection.Conn()
	if err = validateRuntimeRole(ctx, c); err != nil {
		return err
	}
	var secured int
	err = c.QueryRow(ctx, `SELECT count(*) FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace
 WHERE n.nspname='public' AND c.relname IN ('users','auth_sessions','user_anime','watch_progress','user_ratings','user_preferences','recommendations','recommendation_feedback','user_follows','notifications','auth_challenges','identity_outbox','identity_requests')
 AND c.relrowsecurity AND c.relforcerowsecurity`).Scan(&secured)
	if err != nil || secured != 13 {
		return errors.New("required forced row-level security is missing; apply schema migrations")
	}
	if _, err = c.Exec(ctx, `SELECT login_failure_started_at FROM users LIMIT 0`); err != nil {
		return errors.New("required login security schema is missing; apply schema migrations")
	}
	if _, err = c.Exec(ctx, `SELECT outcome FROM identity_requests LIMIT 0; SELECT outcome FROM identity_outbox LIMIT 0`); err != nil {
		return errors.New("required identity queue outcome schema is missing; apply schema migrations")
	}
	return nil
}
