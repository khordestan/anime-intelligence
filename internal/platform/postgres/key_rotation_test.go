package postgres_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"testing"

	"github.com/khordestan/anime-intelligence/internal/platform/identitymail"
	"github.com/khordestan/anime-intelligence/internal/platform/postgres"
)

func TestRollingKeyRotationResumesOldInboxAndMail(t *testing.T) {
	db, owner, _, dsn := testDB(t)
	ctx := context.Background()
	oldKey := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{1}, 32))
	newKey := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{2}, 32))
	oldCipher := envelopeCipher(t, 1)
	newCipher := envelopeCipher(t, 2)
	oldWriter, _ := db.IdentityOutbox(oldCipher)
	mailAccount, err := db.Register(ctx, "old-mail@example.test", "synthetic-rotation-password")
	if err != nil {
		t.Fatal(err)
	}
	requestAccount, err := db.Register(ctx, "old-request@example.test", "synthetic-rotation-password")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = oldWriter.IssueEmailVerification(ctx, mailAccount.Token); err != nil {
		t.Fatal(err)
	}
	if err = oldWriter.SubmitRequest(ctx, identitymail.PasswordReset, requestAccount.User.Email); err != nil {
		t.Fatal(err)
	}
	second, err := postgres.Open(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	newOnly, _ := second.IdentityOutbox(newCipher)
	neverSend := outboxSender(func(context.Context, identitymail.Message) error { t.Fatal("undecryptable mail delivered"); return nil })
	if worked, err := newOnly.ProcessRequest(ctx); worked || err != identitymail.ErrEnvelope {
		t.Fatal("removed old inbox key accepted", worked, err)
	}
	if worked, err := newOnly.DeliverNext(ctx, neverSend); worked || err != identitymail.ErrEnvelope {
		t.Fatal("removed old mail key accepted", worked, err)
	}
	ring, err := identitymail.NewEnvelopeKeyring(newKey, []string{oldKey})
	if err != nil {
		t.Fatal(err)
	}
	rotated, _ := second.IdentityOutbox(ring)
	if worked, err := rotated.ProcessRequest(ctx); !worked || err != nil {
		t.Fatal("rotation did not resume old intent", worked, err)
	}
	var id string
	var payload []byte
	if err = owner.QueryRow(ctx, `SELECT challenge_id::text,payload FROM identity_outbox WHERE user_id=$1`, requestAccount.User.ID).Scan(&id, &payload); err != nil {
		t.Fatal(err)
	}
	if _, err = newCipher.Open(id, payload); err != nil {
		t.Fatal("resumed request not encrypted with new primary", err)
	}
	if _, err = oldCipher.Open(id, payload); err != identitymail.ErrEnvelope {
		t.Fatal("new delivery used old primary")
	}
	delivered := make(map[string]identitymail.Message)
	sender := outboxSender(func(_ context.Context, m identitymail.Message) error { delivered[m.Recipient] = m; return nil })
	for i := 0; i < 2; i++ {
		if worked, err := rotated.DeliverNext(ctx, sender); !worked || err != nil {
			t.Fatal("mixed-version mail not delivered", worked, err)
		}
	}
	if len(delivered) != 2 {
		t.Fatal("lost pre-rotation work")
	}
	if err = second.VerifyEmail(ctx, delivered[mailAccount.User.Email].Token); err != nil {
		t.Fatal("old proof changed", err)
	}
	if err = second.ResetPassword(ctx, delivered[requestAccount.User.Email].Token, "synthetic-rotated-password"); err != nil {
		t.Fatal("resumed reset proof invalid", err)
	}
	// During the preparation phase, old-primary nodes must also read the future
	// primary key before any node starts writing new-key ciphertext.
	prepared, err := identitymail.NewEnvelopeKeyring(oldKey, []string{newKey})
	if err != nil {
		t.Fatal(err)
	}
	preparedOld, _ := db.IdentityOutbox(prepared)
	if err = rotated.SubmitRequest(ctx, identitymail.EmailVerification, requestAccount.Token); err != nil {
		t.Fatal(err)
	}
	// Reset revoked that bearer: accepting a queued digest does not restore it.
	if worked, err := preparedOld.ProcessRequest(ctx); !worked || err != nil {
		t.Fatal("prepared old node could not read new-key intent", worked, err)
	}
	var pending int
	if err = owner.QueryRow(ctx, `SELECT count(*) FROM identity_requests WHERE payload IS NOT NULL`).Scan(&pending); err != nil || pending != 0 {
		t.Fatal("new-key request not consumed", err)
	}
}
