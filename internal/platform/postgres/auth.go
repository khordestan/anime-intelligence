package postgres

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"net/mail"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/khordestan/anime-intelligence/internal/platform/password"
)

type User struct {
	EmailVerified bool   `json:"emailVerified"`
	ID            string `json:"id"`
	Email         string `json:"email"`
	DisplayName   string `json:"displayName"`
}
type Session struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expiresAt"`
	User      User      `json:"user"`
}

var ErrEmail = errors.New("invalid email")

func makeDummyHash() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return password.Hash(base64.RawURLEncoding.EncodeToString(b))
}
func normalizeEmail(email string) (string, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if len(email) > 254 {
		return "", ErrEmail
	}
	a, err := mail.ParseAddress(email)
	if err != nil || a.Address != email || !strings.Contains(email, "@") {
		return "", ErrEmail
	}
	return email, nil
}
func tokenDigest(token string) (string, bool) {
	if len(token) != 43 {
		return "", false
	}
	b, err := base64.RawURLEncoding.Strict().DecodeString(token)
	if err != nil || len(b) != 32 || len(token) != 43 {
		return "", false
	}
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:]), true
}
func newSessionToken() (string, string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", "", err
	}
	token := base64.RawURLEncoding.EncodeToString(b)
	digest, _ := tokenDigest(token)
	return token, digest, nil
}
func issue(ctx context.Context, tx pgx.Tx, u User) (Session, error) {
	token, digest, err := newSessionToken()
	if err != nil {
		return Session{}, err
	}
	s := Session{Token: token, User: u}
	err = tx.QueryRow(ctx, `INSERT INTO auth_sessions(user_id,token_hash,expires_at) VALUES($1,decode($2,'hex'),now()+interval '24 hours') RETURNING expires_at`, u.ID, digest).Scan(&s.ExpiresAt)
	return s, err
}
func (d *DB) Register(ctx context.Context, email, value string) (Session, error) {
	email, err := normalizeEmail(email)
	if err != nil {
		return Session{}, err
	}
	encoded, err := password.Hash(value)
	if err != nil {
		return Session{}, err
	}
	tx, err := d.begin(ctx)
	if err != nil {
		return Session{}, err
	}
	defer rollback(tx)
	var uid string
	if err = tx.QueryRow(ctx, `SELECT gen_random_uuid()::text`).Scan(&uid); err != nil {
		return Session{}, err
	}
	if err = identity(ctx, tx, uid); err != nil {
		return Session{}, err
	}
	u := User{ID: uid, Email: email}
	_, err = tx.Exec(ctx, `INSERT INTO users(id,email,password_hash) VALUES($1,$2,$3)`, uid, email, encoded)
	var pe *pgconn.PgError
	if errors.As(err, &pe) && pe.Code == "23505" {
		return Session{}, ErrConflict
	}
	if err != nil {
		return Session{}, err
	}
	s, err := issue(ctx, tx, u)
	if err != nil {
		return Session{}, err
	}
	return s, tx.Commit(ctx)
}
func (d *DB) Login(ctx context.Context, email, value string) (Session, error) {
	email, err := normalizeEmail(email)
	if err != nil {
		return Session{}, ErrUnauthorized
	}
	if len(value) > password.MaxBytes {
		return Session{}, ErrUnauthorized
	}
	tx, err := d.begin(ctx)
	if err != nil {
		return Session{}, err
	}
	defer rollback(tx)
	if _, err = tx.Exec(ctx, `SELECT set_config('app.login_email',$1,true)`, email); err != nil {
		return Session{}, err
	}
	var u User
	var encoded *string
	var disabled, locked bool
	err = tx.QueryRow(ctx, `SELECT id::text,email,display_name,email_verified_at IS NOT NULL,password_hash,disabled_at IS NOT NULL,COALESCE(locked_until>now(),false) FROM users WHERE lower(email)=$1`, email).Scan(&u.ID, &u.Email, &u.DisplayName, &u.EmailVerified, &encoded, &disabled, &locked)
	if errors.Is(err, pgx.ErrNoRows) {
		password.Verify(d.dummyHash, value)
		return Session{}, ErrUnauthorized
	}
	if err != nil {
		return Session{}, err
	}
	hash := d.dummyHash
	if encoded != nil {
		hash = *encoded
	}
	valid := password.Verify(hash, value)
	if encoded == nil || disabled || locked {
		return Session{}, ErrUnauthorized
	}
	if err = identity(ctx, tx, u.ID); err != nil {
		return Session{}, err
	}
	// Recheck account state under a lock after password verification.
	err = tx.QueryRow(ctx, `SELECT id::text,email,display_name,email_verified_at IS NOT NULL FROM users WHERE id=$1 AND password_hash=$2 AND disabled_at IS NULL AND (locked_until IS NULL OR locked_until<=clock_timestamp()) FOR UPDATE`, u.ID, hash).Scan(&u.ID, &u.Email, &u.DisplayName, &u.EmailVerified)
	if errors.Is(err, pgx.ErrNoRows) {
		return Session{}, ErrUnauthorized
	}
	if err != nil {
		return Session{}, err
	}
	if !valid {
		if err = recordPasswordFailure(ctx, tx, u.ID); err != nil {
			return Session{}, err
		}
		// Persist the denial before returning the generic authentication error.
		if err = tx.Commit(ctx); err != nil {
			return Session{}, err
		}
		return Session{}, ErrUnauthorized
	}
	if _, err = tx.Exec(ctx, `UPDATE users SET last_login_at=now(),failed_login_count=0,login_failure_started_at=NULL,locked_until=NULL WHERE id=$1`, u.ID); err != nil {
		return Session{}, err
	}
	s, err := issue(ctx, tx, u)
	if err != nil {
		return Session{}, err
	}
	return s, tx.Commit(ctx)
}
func (d *DB) Logout(ctx context.Context, token string) error {
	tx, u, err := d.sessionTx(ctx, token)
	if err != nil {
		return err
	}
	defer rollback(tx)
	digest, _ := tokenDigest(token)
	if _, err = tx.Exec(ctx, `UPDATE auth_sessions SET revoked_at=now() WHERE user_id=$1 AND token_hash=decode($2,'hex')`, u.ID, digest); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func (d *DB) Refresh(ctx context.Context, token string) (Session, error) {
	tx, u, err := d.sessionTx(ctx, token)
	if err != nil {
		return Session{}, err
	}
	defer rollback(tx)
	digest, _ := tokenDigest(token)
	next, nextDigest, err := newSessionToken()
	if err != nil {
		return Session{}, err
	}
	s := Session{Token: next, User: u}
	// Retain the logical session ID across rotation. A revocation aimed at this
	// session therefore also invalidates a replacement token issued concurrently.
	err = tx.QueryRow(ctx, `UPDATE auth_sessions SET token_hash=decode($2,'hex'),expires_at=now()+interval '24 hours',last_used_at=now()
 WHERE user_id=$1 AND token_hash=decode($3,'hex') AND revoked_at IS NULL RETURNING expires_at`, u.ID, nextDigest, digest).Scan(&s.ExpiresAt)
	if err != nil {
		return Session{}, err
	}
	return s, tx.Commit(ctx)
}
