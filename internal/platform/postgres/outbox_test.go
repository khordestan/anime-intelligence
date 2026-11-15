package postgres_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"io"
	"log/slog"
	"net/url"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/khordestan/anime-intelligence/internal/platform/identitymail"
	"github.com/khordestan/anime-intelligence/internal/platform/postgres"
)

type outboxSender func(context.Context, identitymail.Message) error

func (f outboxSender) Send(ctx context.Context, m identitymail.Message) error { return f(ctx, m) }
func envelopeCipher(t *testing.T, b byte) *identitymail.EnvelopeCipher {
	t.Helper()
	c, err := identitymail.NewEnvelopeCipher(base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{b}, 32)))
	if err != nil {
		t.Fatal(err)
	}
	return c
}
func TestOutboxAtomicIssuanceRestartRetryAndIsolation(t *testing.T) {
	db, owner, _, runtimeDSN := testDB(t)
	ctx := context.Background()
	session, err := db.Register(ctx, "outbox-one@example.test", "synthetic-outbox-password")
	if err != nil {
		t.Fatal(err)
	}
	cipher := envelopeCipher(t, 1)
	outbox, _ := db.IdentityOutbox(cipher)
	message, err := outbox.IssueEmailVerification(ctx, session.Token)
	if err != nil || message != nil {
		t.Fatal("proof was not queued privately", err)
	}
	var cid string
	var payload []byte
	if err = owner.QueryRow(ctx, `SELECT challenge_id::text,payload FROM identity_outbox WHERE user_id=$1`, session.User.ID).Scan(&cid, &payload); err != nil {
		t.Fatal(err)
	}
	decoded, err := cipher.Open(cid, payload)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(payload, []byte(decoded.Token)) || bytes.Contains(payload, []byte(decoded.Recipient)) {
		t.Fatal("plaintext persisted")
	}
	// A new pool/worker resumes the already-issued proof without quota use.
	second, err := postgres.Open(ctx, runtimeDSN)
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	resumed, _ := second.IdentityOutbox(cipher)
	calls := 0
	sender := outboxSender(func(_ context.Context, m identitymail.Message) error {
		calls++
		if m != decoded {
			t.Fatal("retry changed proof")
		}
		if calls == 1 {
			return errors.New("private provider error")
		}
		return nil
	})
	if worked, err := resumed.DeliverNext(ctx, sender); !worked || err != identitymail.ErrDelivery {
		t.Fatal("failed delivery not recorded", worked, err)
	}
	if worked, err := outbox.DeliverNext(ctx, sender); worked || err != nil {
		t.Fatal("backoff bypassed", worked, err)
	}
	if _, err = owner.Exec(ctx, `UPDATE identity_outbox SET available_at=now()-interval '1 second' WHERE user_id=$1`, session.User.ID); err != nil {
		t.Fatal(err)
	}
	if worked, err := outbox.DeliverNext(ctx, sender); !worked || err != nil {
		t.Fatal("restart did not deliver", worked, err)
	}
	var cleared bool
	var attempts, count int
	if err = owner.QueryRow(ctx, `SELECT payload IS NULL AND completed_at IS NOT NULL AND lease_hash IS NULL,attempts FROM identity_outbox WHERE user_id=$1`, session.User.ID).Scan(&cleared, &attempts); err != nil || !cleared || attempts != 2 {
		t.Fatal("completion did not erase credential", err)
	}
	if err = owner.QueryRow(ctx, `SELECT count(*) FROM auth_challenges WHERE user_id=$1`, session.User.ID).Scan(&count); err != nil || count != 1 {
		t.Fatal("retry consumed issuance quota", err)
	}
	if err = db.VerifyEmail(ctx, decoded.Token); err != nil {
		t.Fatal("queued proof invalid", err)
	}
	runtime, err := pgx.Connect(ctx, runtimeDSN)
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Close(ctx)
	if err = runtime.QueryRow(ctx, `SELECT count(*) FROM identity_outbox`).Scan(&count); err != nil || count != 0 {
		t.Fatal("outbox visible without trusted context", err)
	}
	// Failure to persist encrypted delivery rolls back issuance too.
	u, _ := url.Parse(runtimeDSN)
	role := pgx.Identifier{u.User.Username()}.Sanitize()
	if _, err = owner.Exec(ctx, `REVOKE INSERT ON identity_outbox FROM `+role); err != nil {
		t.Fatal(err)
	}
	if _, err = outbox.IssuePasswordReset(ctx, session.User.Email); err == nil {
		t.Fatal("outbox failure accepted")
	}
	if err = owner.QueryRow(ctx, `SELECT count(*) FROM auth_challenges WHERE user_id=$1 AND purpose='PASSWORD_RESET'`, session.User.ID).Scan(&count); err != nil || count != 0 {
		t.Fatal("failed outbox committed proof", err)
	}
}
func TestOutboxLeasesPreventConcurrentDeliveryAndStaleAcknowledgement(t *testing.T) {
	db, owner, _, dsn := testDB(t)
	ctx := context.Background()
	s, err := db.Register(ctx, "lease@example.test", "synthetic-lease-password")
	if err != nil {
		t.Fatal(err)
	}
	outbox, _ := db.IdentityOutbox(envelopeCipher(t, 1))
	if _, err = outbox.IssueEmailVerification(ctx, s.Token); err != nil {
		t.Fatal(err)
	}
	second, err := postgres.Open(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	other, _ := second.IdentityOutbox(envelopeCipher(t, 1))
	started := make(chan struct{})
	release := make(chan struct{})
	done := make(chan error, 1)
	defer func() {
		select {
		case <-release:
		default:
			close(release)
		}
	}()
	go func() {
		_, err := outbox.DeliverNext(ctx, outboxSender(func(context.Context, identitymail.Message) error { close(started); <-release; return nil }))
		done <- err
	}()
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("claim not started")
	}
	forbidden := outboxSender(func(context.Context, identitymail.Message) error { t.Error("active lease delivered twice"); return nil })
	if worked, err := other.DeliverNext(ctx, forbidden); worked || err != nil {
		t.Fatal("active lease not respected", worked, err)
	}
	if _, err = owner.Exec(ctx, `UPDATE identity_outbox SET lease_until=now()-interval '1 second' WHERE user_id=$1`, s.User.ID); err != nil {
		t.Fatal(err)
	}
	if worked, err := other.DeliverNext(ctx, outboxSender(func(context.Context, identitymail.Message) error { return errors.New("temporary") })); !worked || err != identitymail.ErrDelivery {
		t.Fatal("expired lease not recovered", worked, err)
	}
	close(release)
	if err = <-done; err != nil {
		t.Fatal(err)
	}
	var pending bool
	var attempts int
	if err = owner.QueryRow(ctx, `SELECT completed_at IS NULL AND payload IS NOT NULL,attempts FROM identity_outbox WHERE user_id=$1`, s.User.ID).Scan(&pending, &attempts); err != nil || !pending || attempts != 2 {
		t.Fatal("stale acknowledgement erased new lease", err)
	}
}
func TestOutboxRejectsInvalidProofsAndPreservesWrongKeyWork(t *testing.T) {
	for _, mode := range []string{"wrong-key", "expired", "revoked", "consumed", "disabled", "exhausted", "deleted"} {
		t.Run(mode, func(t *testing.T) {
			db, owner, _, _ := testDB(t)
			ctx := context.Background()
			s, err := db.Register(ctx, "cleanup@example.test", "synthetic-cleanup-password")
			if err != nil {
				t.Fatal(err)
			}
			outbox, _ := db.IdentityOutbox(envelopeCipher(t, 1))
			if _, err = outbox.IssueEmailVerification(ctx, s.Token); err != nil {
				t.Fatal(err)
			}
			query := ""
			switch mode {
			case "wrong-key":
				outbox, _ = db.IdentityOutbox(envelopeCipher(t, 2))
			case "expired":
				query = `UPDATE identity_outbox SET expires_at=now()-interval '1 second' WHERE user_id=$1`
			case "revoked":
				query = `UPDATE auth_challenges SET revoked_at=now() WHERE user_id=$1`
			case "consumed":
				query = `UPDATE auth_challenges SET consumed_at=now() WHERE user_id=$1`
			case "disabled":
				query = `UPDATE users SET disabled_at=now() WHERE id=$1`
			case "exhausted":
				query = `UPDATE identity_outbox SET attempts=5 WHERE user_id=$1`
			case "deleted":
				query = `DELETE FROM users WHERE id=$1`
			}
			if query != "" {
				if _, err = owner.Exec(ctx, query, s.User.ID); err != nil {
					t.Fatal(err)
				}
			}
			sender := outboxSender(func(context.Context, identitymail.Message) error { t.Fatal("invalid proof delivered"); return nil })
			worked, err := outbox.DeliverNext(ctx, sender)
			if worked {
				t.Fatal("invalid delivery attempted")
			}
			if mode == "wrong-key" {
				if err != identitymail.ErrEnvelope {
					t.Fatal("wrong key accepted", err)
				}
				var preserved bool
				if err = owner.QueryRow(ctx, `SELECT payload IS NOT NULL AND attempts=0 FROM identity_outbox WHERE user_id=$1`, s.User.ID).Scan(&preserved); err != nil || !preserved {
					t.Fatal("wrong key destroyed pending work", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			var retained int
			if err = owner.QueryRow(ctx, `SELECT count(*) FROM identity_outbox WHERE user_id=$1 AND payload IS NOT NULL`, s.User.ID).Scan(&retained); err != nil || retained != 0 {
				t.Fatal("invalid ciphertext retained", err)
			}
		})
	}
}

func TestDurableServiceResumesWithoutNewRequestAndErasesCompletedPayload(t *testing.T) {
	db, owner, _, _ := testDB(t)
	ctx := context.Background()
	session, err := db.Register(ctx, "resume@example.test", "synthetic-resume-password")
	if err != nil {
		t.Fatal(err)
	}
	outbox, _ := db.IdentityOutbox(envelopeCipher(t, 1))
	if _, err = outbox.IssueEmailVerification(ctx, session.Token); err != nil {
		t.Fatal(err)
	}
	captured := captureSender{make(chan identitymail.Message, 1)}
	service := identitymail.NewDurableService(outbox, captured, slog.New(slog.NewTextHandler(io.Discard, nil)))
	proof := awaitMessage(t, captured.messages)
	service.Close()
	var cleared bool
	if err = owner.QueryRow(ctx, `SELECT completed_at IS NOT NULL AND payload IS NULL FROM identity_outbox WHERE user_id=$1`, session.User.ID).Scan(&cleared); err != nil || !cleared {
		t.Fatal("shutdown lost accepted delivery acknowledgement", err)
	}
	if err = db.VerifyEmail(ctx, proof.Token); err != nil {
		t.Fatal(err)
	}
	if _, err = owner.Exec(ctx, `UPDATE identity_outbox SET completed_at=now()-interval '2 days' WHERE user_id=$1`, session.User.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = outbox.DeliverNext(ctx, captured); err != nil {
		t.Fatal(err)
	}
	var count int
	if err = owner.QueryRow(ctx, `SELECT count(*) FROM identity_outbox WHERE user_id=$1`, session.User.ID).Scan(&count); err != nil || count != 0 {
		t.Fatal("old metadata not removed", err)
	}
}
func TestOutboxAttemptLimitErasesPayload(t *testing.T) {
	db, owner, _, _ := testDB(t)
	ctx := context.Background()
	session, err := db.Register(ctx, "attempts@example.test", "synthetic-attempt-password")
	if err != nil {
		t.Fatal(err)
	}
	outbox, _ := db.IdentityOutbox(envelopeCipher(t, 1))
	if _, err = outbox.IssueEmailVerification(ctx, session.Token); err != nil {
		t.Fatal(err)
	}
	if _, err = owner.Exec(ctx, `UPDATE identity_outbox SET attempts=4 WHERE user_id=$1`, session.User.ID); err != nil {
		t.Fatal(err)
	}
	sender := outboxSender(func(context.Context, identitymail.Message) error { return errors.New("private error") })
	if worked, err := outbox.DeliverNext(ctx, sender); !worked || err != identitymail.ErrDelivery {
		t.Fatal("final delivery attempt", worked, err)
	}
	var cleared bool
	if err = owner.QueryRow(ctx, `SELECT attempts=5 AND payload IS NULL AND completed_at IS NOT NULL FROM identity_outbox WHERE user_id=$1`, session.User.ID).Scan(&cleared); err != nil || !cleared {
		t.Fatal("attempt cap did not clear ciphertext", err)
	}
	if worked, err := outbox.DeliverNext(ctx, sender); worked || err != nil {
		t.Fatal("exhausted job retried", worked, err)
	}
}
