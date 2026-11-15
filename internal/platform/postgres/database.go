// Package postgres provides the multi-user runtime. All private operations
// resolve an opaque session and execute with transaction-local RLS context.
package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/khordestan/anime-intelligence/internal/platform/store"
)

var ErrUnauthorized = store.ErrUnauthorized
var ErrConflict = errors.New("account unavailable")

type DB struct {
	pool      *pgxpool.Pool
	dummyHash string
}

func Open(ctx context.Context, dsn string) (*DB, error) {
	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, errors.New("invalid database configuration")
	}
	config.MaxConns = 10
	config.ConnConfig.RuntimeParams["search_path"] = "pg_catalog,public"
	config.ConnConfig.ConnectTimeout = 5 * time.Second
	// A compromised or stale connection must never carry identity into a request.
	config.AfterConnect = func(ctx context.Context, c *pgx.Conn) error {
		var unsafe bool
		err := c.QueryRow(ctx, `SELECT r.rolsuper OR r.rolbypassrls OR EXISTS (
   SELECT 1 FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace
   WHERE n.nspname='public' AND c.relkind='r' AND pg_has_role(current_user,c.relowner,'USAGE')
  ) FROM pg_roles r WHERE r.rolname=current_user`).Scan(&unsafe)
		if err != nil {
			return err
		}
		if unsafe {
			return errors.New("database runtime requires a non-owner role without superuser or BYPASSRLS")
		}
		_, err = c.Exec(ctx, `SELECT set_config('app.user_id','',false), set_config('app.login_email','',false), set_config('app.session_token_hash','',false)`)
		return err
	}
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, err
	}
	if err = pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	var secured int
	err = pool.QueryRow(ctx, `SELECT count(*) FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace
 WHERE n.nspname='public' AND c.relname IN ('users','auth_sessions','user_anime','watch_progress','user_ratings')
 AND c.relrowsecurity AND c.relforcerowsecurity`).Scan(&secured)
	if err != nil || secured != 5 {
		pool.Close()
		return nil, errors.New("required forced row-level security is missing; apply schema migrations")
	}
	dummy, err := makeDummyHash()
	if err != nil {
		pool.Close()
		return nil, err
	}
	return &DB{pool: pool, dummyHash: dummy}, nil
}
func (d *DB) Close() { d.pool.Close() }

func (d *DB) begin(ctx context.Context) (pgx.Tx, error) {
	tx, err := d.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	_, err = tx.Exec(ctx, `SELECT set_config('app.user_id','',true),set_config('app.login_email','',true),set_config('app.session_token_hash','',true)`)
	if err != nil {
		rollback(tx)
		return nil, err
	}
	return tx, nil
}
func rollback(tx pgx.Tx) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = tx.Rollback(ctx)
}
func identity(ctx context.Context, tx pgx.Tx, userID string) error {
	_, err := tx.Exec(ctx, `SELECT set_config('app.user_id',$1,true),set_config('app.login_email','',true),set_config('app.session_token_hash','',true)`, userID)
	return err
}
func (d *DB) sessionTx(ctx context.Context, token string) (pgx.Tx, User, error) {
	digest, ok := tokenDigest(token)
	if !ok {
		return nil, User{}, ErrUnauthorized
	}
	tx, err := d.begin(ctx)
	if err != nil {
		return nil, User{}, err
	}
	fail := func(e error) (pgx.Tx, User, error) { rollback(tx); return nil, User{}, e }
	if _, err = tx.Exec(ctx, `SELECT set_config('app.session_token_hash',$1,true)`, digest); err != nil {
		return fail(err)
	}
	var uid string
	// Lock the session until the protected operation commits. Logout/refresh
	// takes the same lock, preventing a revoked credential from racing a write.
	err = tx.QueryRow(ctx, `SELECT user_id::text FROM auth_sessions WHERE token_hash=decode($1,'hex') AND revoked_at IS NULL AND expires_at>now()`, digest).Scan(&uid)
	if errors.Is(err, pgx.ErrNoRows) {
		return fail(ErrUnauthorized)
	}
	if err != nil {
		return fail(err)
	}
	if err = identity(ctx, tx, uid); err != nil {
		return fail(err)
	}
	err = tx.QueryRow(ctx, `SELECT user_id::text FROM auth_sessions WHERE user_id=$1 AND token_hash=decode($2,'hex') AND revoked_at IS NULL AND expires_at>now() FOR UPDATE`, uid, digest).Scan(&uid)
	if errors.Is(err, pgx.ErrNoRows) {
		return fail(ErrUnauthorized)
	}
	if err != nil {
		return fail(err)
	}
	var u User
	err = tx.QueryRow(ctx, `SELECT id::text,email,display_name FROM users WHERE id=$1 AND disabled_at IS NULL AND (locked_until IS NULL OR locked_until<=now()) FOR SHARE`, uid).Scan(&u.ID, &u.Email, &u.DisplayName)
	if errors.Is(err, pgx.ErrNoRows) {
		return fail(ErrUnauthorized)
	}
	if err != nil {
		return fail(err)
	}
	_, err = tx.Exec(ctx, `UPDATE auth_sessions SET last_used_at=now() WHERE user_id=$1 AND token_hash=decode($2,'hex') AND last_used_at<now()-interval '5 minutes'`, uid, digest)
	if err != nil {
		return fail(err)
	}
	return tx, u, nil
}
func (d *DB) Authenticate(ctx context.Context, token string) (User, error) {
	tx, u, err := d.sessionTx(ctx, token)
	if err != nil {
		return User{}, err
	}
	defer rollback(tx)
	return u, tx.Commit(ctx)
}

// Repository binds a presented bearer token, never a client-provided user ID.
func (d *DB) Repository(token string) *Repository { return &Repository{db: d, token: token} }
