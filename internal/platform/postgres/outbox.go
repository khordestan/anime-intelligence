package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/khordestan/anime-intelligence/internal/platform/identitymail"
)

// DeliveryOutbox is a trusted worker boundary. Never expose its rows to HTTP.
// Create it once with a deployment key held outside PostgreSQL.
type DeliveryOutbox struct {
	db     *DB
	cipher *identitymail.EnvelopeCipher
}

func (d *DB) IdentityOutbox(cipher *identitymail.EnvelopeCipher) (*DeliveryOutbox, error) {
	if cipher == nil {
		return nil, identitymail.ErrEnvelope
	}
	return &DeliveryOutbox{d, cipher}, nil
}
func (o *DeliveryOutbox) IssuePasswordReset(ctx context.Context, email string) (*identitymail.Message, error) {
	return o.db.issuePasswordReset(ctx, email, o.cipher)
}
func (o *DeliveryOutbox) IssueEmailVerification(ctx context.Context, token string) (*identitymail.Message, error) {
	return o.db.issueEmailVerification(ctx, token, o.cipher)
}
func enqueueDelivery(ctx context.Context, tx pgx.Tx, cipher *identitymail.EnvelopeCipher, m *identitymail.Message) error {
	digest, _ := challengeDigest(m.Purpose, m.Token)
	var id, uid string
	if err := tx.QueryRow(ctx, `SELECT id::text,user_id::text FROM auth_challenges WHERE user_id=app_current_user_id() AND token_hash=decode($1,'hex')`, digest).Scan(&id, &uid); err != nil {
		return err
	}
	payload, err := cipher.Seal(id, *m)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO identity_outbox(user_id,challenge_id,payload,expires_at) VALUES($1,$2,$3,$4)`, uid, id, payload, m.ExpiresAt)
	return err
}

type deliveryClaim struct {
	id, lease string
	attempts  int
	message   identitymail.Message
}

func (o *DeliveryOutbox) cleanup(ctx context.Context) error {
	tx, err := o.db.begin(ctx)
	if err != nil {
		return err
	}
	defer rollback(tx)
	if _, err = tx.Exec(ctx, `SELECT set_config('app.delivery_worker','on',true)`); err != nil {
		return err
	}
	// Bound cleanup, erase expired/exhausted credentials and retain only one day
	// of completed delivery metadata. Neither query touches account/proof locks.
	if _, err = tx.Exec(ctx, `UPDATE identity_outbox SET completed_at=now(),payload=NULL,lease_hash=NULL,lease_until=NULL WHERE id IN (SELECT id FROM identity_outbox WHERE completed_at IS NULL AND (expires_at<=now() OR (attempts>=5 AND (lease_until IS NULL OR lease_until<=now()))) LIMIT 100 FOR UPDATE SKIP LOCKED)`); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `DELETE FROM identity_outbox WHERE id IN (SELECT id FROM identity_outbox WHERE completed_at<now()-interval '1 day' LIMIT 100 FOR UPDATE SKIP LOCKED)`); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func (o *DeliveryOutbox) claim(ctx context.Context) (*deliveryClaim, error) {
	if err := o.cleanup(ctx); err != nil {
		return nil, err
	}
	tx, err := o.db.begin(ctx)
	if err != nil {
		return nil, err
	}
	defer rollback(tx)
	if _, err = tx.Exec(ctx, `SELECT set_config('app.delivery_worker','on',true)`); err != nil {
		return nil, err
	}
	var id, uid string
	err = tx.QueryRow(ctx, `SELECT id::text,user_id::text FROM identity_outbox WHERE completed_at IS NULL AND payload IS NOT NULL AND expires_at>now() AND attempts<5 AND available_at<=now() AND (lease_until IS NULL OR lease_until<=now()) ORDER BY available_at,created_at LIMIT 1`).Scan(&id, &uid)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, tx.Commit(ctx)
	}
	if err != nil {
		return nil, err
	}
	// Match the account-before-proof-before-outbox ordering used by issuance and
	// credential changes. No database locks are held while contacting SMTP.
	if err = identity(ctx, tx, uid); err != nil {
		return nil, err
	}
	var email string
	var disabled bool
	err = tx.QueryRow(ctx, `SELECT email,disabled_at IS NOT NULL FROM users WHERE id=$1 FOR SHARE`, uid).Scan(&email, &disabled)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var cid, purpose, digest string
	var expiry time.Time
	var invalid bool
	err = tx.QueryRow(ctx, `SELECT c.id::text,c.purpose,encode(c.token_hash,'hex'),c.expires_at,c.consumed_at IS NOT NULL OR c.revoked_at IS NOT NULL OR c.email<>$3 OR c.expires_at<=now() FROM auth_challenges c JOIN identity_outbox o ON o.challenge_id=c.id AND o.user_id=c.user_id WHERE o.id=$1 AND c.user_id=$2 FOR SHARE OF c`, id, uid, email).Scan(&cid, &purpose, &digest, &expiry, &invalid)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var payload []byte
	var attempts int
	err = tx.QueryRow(ctx, `SELECT payload,attempts FROM identity_outbox WHERE id=$1 AND user_id=$2 AND completed_at IS NULL AND payload IS NOT NULL AND attempts<5 AND available_at<=now() AND expires_at>now() AND (lease_until IS NULL OR lease_until<=now()) FOR UPDATE SKIP LOCKED`, id, uid).Scan(&payload, &attempts)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	message, decodeErr := o.cipher.Open(cid, payload)
	actual, _ := challengeDigest(message.Purpose, message.Token)
	if disabled || invalid || (decodeErr == nil && (message.Purpose != purpose || message.Recipient != email || !message.ExpiresAt.Equal(expiry) || actual != digest)) {
		_, err = tx.Exec(ctx, `UPDATE identity_outbox SET completed_at=now(),payload=NULL,lease_hash=NULL,lease_until=NULL WHERE id=$1 AND user_id=$2`, id, uid)
		if err != nil {
			return nil, err
		}
		return nil, tx.Commit(ctx)
	}
	// A wrong deployment key must not destroy pending work. Preserve ciphertext
	// for key correction; the service emits a fixed error without private fields.
	if decodeErr != nil {
		return nil, identitymail.ErrEnvelope
	}
	lease, leaseDigest, err := newSessionToken()
	if err != nil {
		return nil, err
	}
	if _, err = tx.Exec(ctx, `UPDATE identity_outbox SET attempts=attempts+1,lease_hash=decode($3,'hex'),lease_until=now()+interval '1 minute' WHERE id=$1 AND user_id=$2`, id, uid, leaseDigest); err != nil {
		return nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &deliveryClaim{id: id, lease: lease, attempts: attempts + 1, message: message}, nil
}
func (o *DeliveryOutbox) finish(ctx context.Context, c *deliveryClaim, accepted bool) error {
	digest, ok := tokenDigest(c.lease)
	if !ok {
		return identitymail.ErrDelivery
	}
	tx, err := o.db.begin(ctx)
	if err != nil {
		return err
	}
	defer rollback(tx)
	if _, err = tx.Exec(ctx, `SELECT set_config('app.delivery_worker','on',true)`); err != nil {
		return err
	}
	// Matching the lease prevents a stale worker acknowledging another claim.
	if accepted || c.attempts >= 5 {
		_, err = tx.Exec(ctx, `UPDATE identity_outbox SET completed_at=now(),payload=NULL,lease_hash=NULL,lease_until=NULL WHERE id=$1 AND lease_hash=decode($2,'hex')`, c.id, digest)
	} else {
		delay := 30 * (1 << (c.attempts - 1))
		_, err = tx.Exec(ctx, `UPDATE identity_outbox SET available_at=now()+$3*interval '1 second',lease_hash=NULL,lease_until=NULL WHERE id=$1 AND lease_hash=decode($2,'hex')`, c.id, digest, delay)
	}
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func (o *DeliveryOutbox) DeliverNext(ctx context.Context, sender identitymail.Sender) (bool, error) {
	c, err := o.claim(ctx)
	if err != nil || c == nil {
		return false, err
	}
	deadline := time.Now().Add(15 * time.Second)
	if c.message.ExpiresAt.Before(deadline) {
		deadline = c.message.ExpiresAt
	}
	attemptCtx, cancel := context.WithDeadline(ctx, deadline)
	if ctx.Err() != nil || !time.Now().Before(c.message.ExpiresAt) {
		err = identitymail.ErrDelivery
	} else {
		err = sender.Send(attemptCtx, c.message)
	}
	cancel()
	// Acknowledge accepted DATA even during shutdown; a failed acknowledgement
	// may still duplicate delivery after lease expiry (at-least-once semantics).
	finishCtx, stop := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer stop()
	if finishErr := o.finish(finishCtx, c, err == nil); finishErr != nil {
		return true, identitymail.ErrDelivery
	}
	if err != nil {
		return true, identitymail.ErrDelivery
	}
	return true, nil
}

// Avoid accidentally formatting encrypted payloads or lease credentials.
func (*deliveryClaim) String() string { return "identity delivery claim (redacted)" }
