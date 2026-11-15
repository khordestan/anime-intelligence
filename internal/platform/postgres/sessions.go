package postgres

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
)

// SessionInfo intentionally omits both raw credentials and token digests.
type SessionInfo struct {
	ID         string    `json:"id"`
	CreatedAt  time.Time `json:"createdAt"`
	ExpiresAt  time.Time `json:"expiresAt"`
	LastUsedAt time.Time `json:"lastUsedAt"`
	Current    bool      `json:"current"`
}

// Sessions returns the current session followed by the newest active sessions.
// Bound the response even when an account has accumulated many logins.
func (d *DB) Sessions(ctx context.Context, token string) ([]SessionInfo, bool, error) {
	tx, u, err := d.sessionTx(ctx, token)
	if err != nil {
		return nil, false, err
	}
	defer rollback(tx)
	digest, _ := tokenDigest(token)
	rows, err := tx.Query(ctx, `SELECT id::text,created_at,expires_at,last_used_at,token_hash=decode($2,'hex')
 FROM auth_sessions WHERE user_id=$1 AND revoked_at IS NULL AND expires_at>now()
 ORDER BY (token_hash=decode($2,'hex')) DESC,created_at DESC,id DESC LIMIT 101`, u.ID, digest)
	if err != nil {
		return nil, false, err
	}
	items := []SessionInfo{}
	for rows.Next() {
		var item SessionInfo
		if err = rows.Scan(&item.ID, &item.CreatedAt, &item.ExpiresAt, &item.LastUsedAt, &item.Current); err != nil {
			rows.Close()
			return nil, false, err
		}
		items = append(items, item)
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return nil, false, err
	}
	more := len(items) > 100
	if more {
		items = items[:100]
	}
	return items, more, tx.Commit(ctx)
}

// RevokeSession serializes account-wide credential mutations before locking
// session rows, preventing cross-session revocation/deletion deadlocks.
func (d *DB) RevokeSession(ctx context.Context, token, id string) (bool, error) {
	tx, u, err := d.sessionTxWithUserLock(ctx, token, pgx.TxOptions{}, true)
	if err != nil {
		return false, err
	}
	defer rollback(tx)
	tag, err := tx.Exec(ctx, `UPDATE auth_sessions SET revoked_at=now()
 WHERE user_id=$1 AND id=$2 AND revoked_at IS NULL AND expires_at>now()`, u.ID, id)
	if err != nil {
		return false, err
	}
	if err = tx.Commit(ctx); err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

// RevokeOtherSessions affects every other active session, including sessions
// beyond the bounded listing. The presented session remains valid.
func (d *DB) RevokeOtherSessions(ctx context.Context, token string) (int64, error) {
	tx, u, err := d.sessionTxWithUserLock(ctx, token, pgx.TxOptions{}, true)
	if err != nil {
		return 0, err
	}
	defer rollback(tx)
	digest, _ := tokenDigest(token)
	tag, err := tx.Exec(ctx, `UPDATE auth_sessions SET revoked_at=now()
 WHERE user_id=$1 AND token_hash<>decode($2,'hex') AND revoked_at IS NULL AND expires_at>now()`, u.ID, digest)
	if err != nil {
		return 0, err
	}
	if err = tx.Commit(ctx); err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}
