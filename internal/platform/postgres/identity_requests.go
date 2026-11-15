package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/khordestan/anime-intelligence/internal/platform/identitymail"
)

// SubmitRequest commits an encrypted, bounded intent without account lookup.
// A successful response acknowledges this commit, never SMTP delivery.
func (o *DeliveryOutbox) SubmitRequest(ctx context.Context, purpose, value string) error {
	if purpose == identitymail.EmailVerification {
		digest, ok := tokenDigest(value)
		if !ok {
			return identitymail.ErrEnvelope
		}
		value = digest
	}
	tx, err := o.db.begin(ctx)
	if err != nil {
		return err
	}
	defer rollback(tx)
	if _, err = tx.Exec(ctx, `SELECT set_config('app.identity_request_worker','on',true)`); err != nil {
		return err
	}
	// Serialize admission across pools/replicas. No account identity participates
	// in capacity decisions; workers never take this admission advisory lock.
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(742019684)`); err != nil {
		return err
	}
	var full bool
	if err = tx.QueryRow(ctx, `SELECT count(*)>=$1 FROM identity_requests WHERE completed_at IS NULL AND expires_at>now()`, identityRequestCapacity).Scan(&full); err != nil {
		return err
	}
	if full {
		return identitymail.ErrDelivery
	}
	var id string
	if err = tx.QueryRow(ctx, `SELECT gen_random_uuid()::text`).Scan(&id); err != nil {
		return err
	}
	payload, err := o.cipher.SealRequest(id, purpose, value)
	if err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO identity_requests(id,payload) VALUES($1,$2)`, id, payload); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

type requestClaim struct {
	id, lease, purpose, value string
	attempts                  int
}

func (*requestClaim) String() string { return "identity request claim (redacted)" }
func (o *DeliveryOutbox) claimRequest(ctx context.Context) (*requestClaim, error) {
	tx, err := o.db.begin(ctx)
	if err != nil {
		return nil, err
	}
	defer rollback(tx)
	if _, err = tx.Exec(ctx, `SELECT set_config('app.identity_request_worker','on',true)`); err != nil {
		return nil, err
	}
	if _, err = tx.Exec(ctx, `UPDATE identity_requests SET payload=NULL,completed_at=now(),lease_hash=NULL,lease_until=NULL,outcome=CASE WHEN expires_at<=now() THEN 'EXPIRED' ELSE 'EXHAUSTED' END WHERE id IN (SELECT id FROM identity_requests WHERE completed_at IS NULL AND (expires_at<=now() OR (attempts>=5 AND (lease_until IS NULL OR lease_until<=now()))) LIMIT 100 FOR UPDATE SKIP LOCKED)`); err != nil {
		return nil, err
	}
	if _, err = tx.Exec(ctx, `DELETE FROM identity_requests WHERE id IN (SELECT id FROM identity_requests WHERE completed_at<now()-interval '1 day' LIMIT 100 FOR UPDATE SKIP LOCKED)`); err != nil {
		return nil, err
	}
	var id string
	var payload []byte
	var attempts int
	err = tx.QueryRow(ctx, `SELECT id::text,payload,attempts FROM identity_requests WHERE completed_at IS NULL AND payload IS NOT NULL AND expires_at>now() AND available_at<=now() AND attempts<5 AND (lease_until IS NULL OR lease_until<=now()) ORDER BY available_at,created_at LIMIT 1 FOR UPDATE SKIP LOCKED`).Scan(&id, &payload, &attempts)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, tx.Commit(ctx)
	}
	if err != nil {
		return nil, err
	}
	purpose, value, err := o.cipher.OpenRequest(id, payload)
	if err != nil {
		return nil, identitymail.ErrEnvelope
	}
	lease, digest, err := newSessionToken()
	if err != nil {
		return nil, err
	}
	if _, err = tx.Exec(ctx, `UPDATE identity_requests SET attempts=attempts+1,lease_hash=decode($2,'hex'),lease_until=now()+interval '1 minute' WHERE id=$1`, id, digest); err != nil {
		return nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &requestClaim{id: id, lease: lease, purpose: purpose, value: value, attempts: attempts + 1}, nil
}
func (o *DeliveryOutbox) consumeRequest(ctx context.Context, c *requestClaim) error {
	var tx pgx.Tx
	var user User
	var err error
	if c.purpose == identitymail.PasswordReset {
		tx, user, err = o.db.passwordResetTx(ctx, c.value)
	} else {
		tx, user, err = o.db.sessionDigestTxWithUserLock(ctx, c.value, pgx.TxOptions{}, true)
		if errors.Is(err, ErrUnauthorized) {
			tx, err = o.db.begin(ctx)
			user = User{}
		}
	}
	if err != nil {
		return err
	}
	defer rollback(tx)
	if _, err = tx.Exec(ctx, `SELECT set_config('app.identity_request_worker','on',true)`); err != nil {
		return err
	}
	digest, _ := tokenDigest(c.lease)
	var id string
	err = tx.QueryRow(ctx, `SELECT id::text FROM identity_requests WHERE id=$1 AND lease_hash=decode($2,'hex') AND completed_at IS NULL AND expires_at>clock_timestamp() FOR UPDATE`, c.id, digest).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	outcome := "NO_ACTION"
	if user.ID != "" && !(c.purpose == identitymail.EmailVerification && user.EmailVerified) {
		message, err := issueChallenge(ctx, tx, user.ID, user.Email, c.purpose)
		if err != nil {
			return err
		}
		if message != nil {
			if err = enqueueDelivery(ctx, tx, o.cipher, message); err != nil {
				return err
			}
			outcome = "ISSUED"
		}
	}
	// Admission remains generic. Worker-only outcome distinguishes actual proof
	// issuance from no-op requests without retaining a recipient or reason.
	// Proof issuance, delivery insertion and consumption either all commit or none.
	if _, err = tx.Exec(ctx, `UPDATE identity_requests SET completed_at=now(),payload=NULL,lease_hash=NULL,lease_until=NULL,outcome=$3 WHERE id=$1 AND lease_hash=decode($2,'hex')`, id, digest, outcome); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func (o *DeliveryOutbox) retryRequest(ctx context.Context, c *requestClaim) error {
	tx, err := o.db.begin(ctx)
	if err != nil {
		return err
	}
	defer rollback(tx)
	if _, err = tx.Exec(ctx, `SELECT set_config('app.identity_request_worker','on',true)`); err != nil {
		return err
	}
	digest, _ := tokenDigest(c.lease)
	if c.attempts >= 5 {
		_, err = tx.Exec(ctx, `UPDATE identity_requests SET payload=NULL,completed_at=now(),lease_hash=NULL,lease_until=NULL,outcome='EXHAUSTED' WHERE id=$1 AND lease_hash=decode($2,'hex')`, c.id, digest)
	} else {
		_, err = tx.Exec(ctx, `UPDATE identity_requests SET available_at=now()+$3*interval '1 second',lease_hash=NULL,lease_until=NULL WHERE id=$1 AND lease_hash=decode($2,'hex')`, c.id, digest, 5*(1<<(c.attempts-1)))
	}
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func (o *DeliveryOutbox) ProcessRequest(ctx context.Context) (bool, error) {
	c, err := o.claimRequest(ctx)
	if err != nil || c == nil {
		return false, err
	}
	if err = o.consumeRequest(ctx, c); err != nil {
		// A crash leaves the lease to expire. A live failure schedules bounded retry;
		// acknowledgement loss cannot duplicate issuance because consumption shares
		// the same transaction as the proof and encrypted delivery.
		if ctx.Err() == nil {
			_ = o.retryRequest(ctx, c)
		}
		return true, identitymail.ErrDelivery
	}
	return true, nil
}
