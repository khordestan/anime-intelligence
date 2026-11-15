package postgres

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/khordestan/anime-intelligence/internal/platform/identitymail"
	"github.com/khordestan/anime-intelligence/internal/platform/password"
)

var ErrInvalidChallenge = errors.New("invalid or expired challenge")

func challengeDigest(purpose, token string) (string, bool) {
	if purpose != identitymail.PasswordReset && purpose != identitymail.EmailVerification {
		return "", false
	}
	if _, ok := tokenDigest(token); !ok {
		return "", false
	}
	sum := sha256.Sum256([]byte(purpose + ":" + token))
	return hex.EncodeToString(sum[:]), true
}
func issueChallenge(ctx context.Context, tx pgx.Tx, userID, email, purpose string) (*identitymail.Message, error) {
	var recent bool
	if err := tx.QueryRow(ctx, `SELECT count(*)>=5 OR COALESCE(max(created_at)>now()-interval '1 minute',false) FROM auth_challenges WHERE user_id=$1 AND purpose=$2 AND created_at>now()-interval '1 hour'`, userID, purpose).Scan(&recent); err != nil {
		return nil, err
	}
	if recent {
		return nil, nil
	}
	if _, err := tx.Exec(ctx, `UPDATE auth_challenges SET revoked_at=now() WHERE user_id=$1 AND purpose=$2 AND consumed_at IS NULL AND revoked_at IS NULL`, userID, purpose); err != nil {
		return nil, err
	}
	token, _, err := newSessionToken()
	if err != nil {
		return nil, err
	}
	digest, _ := challengeDigest(purpose, token)
	ttl := int64(30 * 60)
	if purpose == identitymail.EmailVerification {
		ttl = 60 * 60
	}
	message := &identitymail.Message{Purpose: purpose, Recipient: email, Token: token}
	err = tx.QueryRow(ctx, `INSERT INTO auth_challenges(user_id,purpose,email,token_hash,expires_at)
 VALUES($1,$2,$3,decode($4,'hex'),now()+$5*interval '1 second') RETURNING expires_at`, userID, purpose, email, digest, ttl).Scan(&message.ExpiresAt)
	return message, err
}

// IssuePasswordReset is called only by the asynchronous delivery worker. The
// HTTP response never depends on account existence or SMTP latency.
func (d *DB) IssuePasswordReset(ctx context.Context, email string) (*identitymail.Message, error) {
	email, err := normalizeEmail(email)
	if err != nil {
		return nil, nil
	}
	tx, err := d.begin(ctx)
	if err != nil {
		return nil, err
	}
	defer rollback(tx)
	if _, err = tx.Exec(ctx, `SELECT set_config('app.login_email',$1,true)`, email); err != nil {
		return nil, err
	}
	var uid string
	err = tx.QueryRow(ctx, `SELECT id::text FROM users WHERE lower(email)=$1 AND disabled_at IS NULL AND password_hash IS NOT NULL`, email).Scan(&uid)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if err = identity(ctx, tx, uid); err != nil {
		return nil, err
	}
	// Recheck under the same exclusive account lock used by password changes.
	err = tx.QueryRow(ctx, `SELECT email FROM users WHERE id=$1 AND lower(email)=$2 AND disabled_at IS NULL AND password_hash IS NOT NULL FOR UPDATE`, uid, email).Scan(&email)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	message, err := issueChallenge(ctx, tx, uid, email, identitymail.PasswordReset)
	if err != nil {
		return nil, err
	}
	return message, tx.Commit(ctx)
}
func (d *DB) IssueEmailVerification(ctx context.Context, token string) (*identitymail.Message, error) {
	tx, u, err := d.sessionTxWithUserLock(ctx, token, pgx.TxOptions{}, true)
	if err != nil {
		return nil, err
	}
	defer rollback(tx)
	if u.EmailVerified {
		return nil, nil
	}
	message, err := issueChallenge(ctx, tx, u.ID, u.Email, identitymail.EmailVerification)
	if err != nil {
		return nil, err
	}
	return message, tx.Commit(ctx)
}

func (d *DB) challengeTx(ctx context.Context, purpose, token string) (pgx.Tx, User, string, error) {
	digest, ok := challengeDigest(purpose, token)
	if !ok {
		return nil, User{}, "", ErrInvalidChallenge
	}
	tx, err := d.begin(ctx)
	if err != nil {
		return nil, User{}, "", err
	}
	fail := func(err error) (pgx.Tx, User, string, error) { rollback(tx); return nil, User{}, "", err }
	if _, err = tx.Exec(ctx, `SELECT set_config('app.challenge_token_hash',$1,true),set_config('app.challenge_purpose',$2,true)`, digest, purpose); err != nil {
		return fail(err)
	}
	var uid string
	err = tx.QueryRow(ctx, `SELECT user_id::text FROM auth_challenges WHERE token_hash=decode($1,'hex') AND purpose=$2 AND consumed_at IS NULL AND revoked_at IS NULL AND expires_at>now()`, digest, purpose).Scan(&uid)
	if errors.Is(err, pgx.ErrNoRows) {
		return fail(ErrInvalidChallenge)
	}
	if err != nil {
		return fail(err)
	}
	if err = identity(ctx, tx, uid); err != nil {
		return fail(err)
	}
	var u User
	err = tx.QueryRow(ctx, `SELECT id::text,email,display_name,email_verified_at IS NOT NULL FROM users WHERE id=$1 AND disabled_at IS NULL FOR UPDATE`, uid).Scan(&u.ID, &u.Email, &u.DisplayName, &u.EmailVerified)
	if errors.Is(err, pgx.ErrNoRows) {
		return fail(ErrInvalidChallenge)
	}
	if err != nil {
		return fail(err)
	}
	var id string
	err = tx.QueryRow(ctx, `SELECT id::text FROM auth_challenges WHERE user_id=$1 AND purpose=$2 AND token_hash=decode($3,'hex') AND email=$4 AND consumed_at IS NULL AND revoked_at IS NULL AND expires_at>now() FOR UPDATE`, uid, purpose, digest, u.Email).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return fail(ErrInvalidChallenge)
	}
	if err != nil {
		return fail(err)
	}
	return tx, u, id, nil
}
func (d *DB) ResetPassword(ctx context.Context, token, next string) error {
	tx, u, id, err := d.challengeTx(ctx, identitymail.PasswordReset, token)
	if err != nil {
		return err
	}
	defer rollback(tx)
	hash, err := password.Hash(next)
	if err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `UPDATE users SET password_hash=$2,updated_at=now(),failed_login_count=0,locked_until=NULL WHERE id=$1`, u.ID, hash); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `UPDATE auth_sessions SET revoked_at=now() WHERE user_id=$1 AND revoked_at IS NULL`, u.ID); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `UPDATE auth_challenges SET consumed_at=now() WHERE id=$1 AND user_id=$2`, id, u.ID); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `UPDATE auth_challenges SET revoked_at=now() WHERE user_id=$1 AND consumed_at IS NULL AND revoked_at IS NULL`, u.ID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func (d *DB) VerifyEmail(ctx context.Context, token string) error {
	tx, u, id, err := d.challengeTx(ctx, identitymail.EmailVerification, token)
	if err != nil {
		return err
	}
	defer rollback(tx)
	if _, err = tx.Exec(ctx, `UPDATE users SET email_verified_at=COALESCE(email_verified_at,now()),updated_at=now() WHERE id=$1 AND email=$2`, u.ID, u.Email); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `UPDATE auth_challenges SET consumed_at=now() WHERE id=$1 AND user_id=$2`, id, u.ID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
