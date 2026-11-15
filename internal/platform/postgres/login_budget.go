package postgres

import (
	"context"
	"github.com/jackc/pgx/v5"
)

// The caller holds the account's exclusive lock and has rechecked the same
// password hash and current enabled/unlocked state. The budget is shared by all
// API instances. Active locks are never extended by further failed requests.
func recordFailedLogin(ctx context.Context, tx pgx.Tx, userID string) error {
	_, err := tx.Exec(ctx, `WITH budget AS (
 SELECT id,
 CASE WHEN login_failure_started_at IS NULL OR login_failure_started_at<=statement_timestamp()-interval '15 minutes'
 THEN 1 ELSE LEAST(failed_login_count,9)+1 END AS failures,
 CASE WHEN login_failure_started_at IS NULL OR login_failure_started_at<=statement_timestamp()-interval '15 minutes'
 THEN statement_timestamp() ELSE login_failure_started_at END AS started
 FROM users WHERE id=$1
 ) UPDATE users u SET failed_login_count=b.failures,login_failure_started_at=b.started,
 locked_until=CASE WHEN b.failures>=10 THEN statement_timestamp()+interval '15 minutes' ELSE NULL END
 FROM budget b WHERE u.id=b.id`, userID)
	return err
}
