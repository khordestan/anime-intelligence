package postgres_test

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/khordestan/anime-intelligence/internal/platform/identitymail"
	"github.com/khordestan/anime-intelligence/internal/platform/password"
	"github.com/khordestan/anime-intelligence/internal/platform/postgres"
)

func TestIdentityChallengesArePurposeBoundSingleUseAndPrivate(t *testing.T) {
	db, owner, _, runtimeDSN := testDB(t)
	ctx := context.Background()
	one, err := db.Register(ctx, "challenge-one@example.test", "synthetic-original-password")
	if err != nil {
		t.Fatal(err)
	}
	other, err := db.Login(ctx, one.User.Email, "synthetic-original-password")
	if err != nil {
		t.Fatal(err)
	}
	two, err := db.Register(ctx, "challenge-two@example.test", "synthetic-second-password")
	if err != nil {
		t.Fatal(err)
	}
	reset, err := db.IssuePasswordReset(ctx, " CHALLENGE-ONE@example.test ")
	if err != nil || reset == nil {
		t.Fatal("issue reset", err)
	}
	verification, err := db.IssueEmailVerification(ctx, one.Token)
	if err != nil || verification == nil {
		t.Fatal("issue verification", err)
	}
	if one.User.EmailVerified || reset.Recipient != one.User.Email || len(reset.Token) != 43 {
		t.Fatal("challenge contract")
	}
	encoded, err := json.Marshal(reset)
	if err != nil || strings.Contains(string(encoded), reset.Token) || strings.Contains(string(encoded), one.User.Email) {
		t.Fatal("delivery envelope can leak through JSON", err)
	}
	var stored []byte
	if err = owner.QueryRow(ctx, `SELECT token_hash FROM auth_challenges WHERE user_id=$1 AND purpose='PASSWORD_RESET'`, one.User.ID).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	expected := sha256.Sum256([]byte(identitymail.PasswordReset + ":" + reset.Token))
	if string(stored) != string(expected[:]) {
		t.Fatal("raw token or wrong digest stored")
	}
	raw, err := pgx.Connect(ctx, runtimeDSN)
	if err != nil {
		t.Fatal(err)
	}
	defer raw.Close(ctx)
	var count int
	if err = raw.QueryRow(ctx, `SELECT count(*) FROM auth_challenges`).Scan(&count); err != nil || count != 0 {
		t.Fatal("unscoped RLS exposed challenge credentials", err)
	}
	tx, err := raw.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(ctx, `SELECT set_config('app.user_id',$1,true)`, two.User.ID); err != nil {
		t.Fatal(err)
	}
	if err = tx.QueryRow(ctx, `SELECT count(*) FROM auth_challenges WHERE user_id=$1`, one.User.ID).Scan(&count); err != nil || count != 0 {
		t.Fatal("cross-user RLS exposed challenges", err)
	}
	_ = tx.Rollback(ctx)
	if err = db.VerifyEmail(ctx, reset.Token); !errors.Is(err, postgres.ErrInvalidChallenge) {
		t.Fatal("reset token accepted for verification", err)
	}
	if err = db.ResetPassword(ctx, verification.Token, "synthetic-replacement-password"); !errors.Is(err, postgres.ErrInvalidChallenge) {
		t.Fatal("verification token reset a password", err)
	}
	if err = db.VerifyEmail(ctx, verification.Token); err != nil {
		t.Fatal(err)
	}
	if err = db.VerifyEmail(ctx, verification.Token); !errors.Is(err, postgres.ErrInvalidChallenge) {
		t.Fatal("verification replay", err)
	}
	if u, err := db.Authenticate(ctx, one.Token); err != nil || !u.EmailVerified {
		t.Fatal("verification status not reflected in sessions", err)
	}
	if err = db.ResetPassword(ctx, reset.Token, "short"); !errors.Is(err, password.ErrInvalid) {
		t.Fatal("invalid replacement accepted", err)
	}
	if err = db.ResetPassword(ctx, reset.Token, "synthetic-replacement-password"); err != nil {
		t.Fatal(err)
	}
	if err = db.ResetPassword(ctx, reset.Token, "another-replacement-password"); !errors.Is(err, postgres.ErrInvalidChallenge) {
		t.Fatal("reset replay", err)
	}
	for _, token := range []string{one.Token, other.Token} {
		if _, err = db.Authenticate(ctx, token); !errors.Is(err, postgres.ErrUnauthorized) {
			t.Fatal("reset left old sessions valid", err)
		}
	}
	if _, err = db.Login(ctx, one.User.Email, "synthetic-original-password"); !errors.Is(err, postgres.ErrUnauthorized) {
		t.Fatal("old password survived reset", err)
	}
	if _, err = db.Login(ctx, one.User.Email, "synthetic-replacement-password"); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Authenticate(ctx, two.Token); err != nil {
		t.Fatal("reset affected another account", err)
	}
}

func TestChallengeCooldownSupersessionExpiryAndAccountBinding(t *testing.T) {
	db, owner, _, _ := testDB(t)
	ctx := context.Background()
	s, err := db.Register(ctx, "challenge-lifecycle@example.test", "synthetic-original-password")
	if err != nil {
		t.Fatal(err)
	}
	if msg, err := db.IssuePasswordReset(ctx, "missing@example.test"); err != nil || msg != nil {
		t.Fatal("issued unknown-account challenge", err)
	}
	if msg, err := db.IssuePasswordReset(ctx, "malformed"); err != nil || msg != nil {
		t.Fatal("issued malformed-email challenge", err)
	}
	first, err := db.IssuePasswordReset(ctx, s.User.Email)
	if err != nil || first == nil {
		t.Fatal(err)
	}
	if msg, err := db.IssuePasswordReset(ctx, s.User.Email); err != nil || msg != nil {
		t.Fatal("cooldown did not suppress duplicate mail", err)
	}
	if _, err = owner.Exec(ctx, `UPDATE auth_challenges SET created_at=now()-interval '2 minutes' WHERE user_id=$1`, s.User.ID); err != nil {
		t.Fatal(err)
	}
	next, err := db.IssuePasswordReset(ctx, s.User.Email)
	if err != nil || next == nil {
		t.Fatal(err)
	}
	if err = db.ResetPassword(ctx, first.Token, "synthetic-replacement-password"); !errors.Is(err, postgres.ErrInvalidChallenge) {
		t.Fatal("superseded token accepted", err)
	}
	if _, err = owner.Exec(ctx, `UPDATE users SET email='changed@example.test' WHERE id=$1`, s.User.ID); err != nil {
		t.Fatal(err)
	}
	if err = db.ResetPassword(ctx, next.Token, "synthetic-replacement-password"); !errors.Is(err, postgres.ErrInvalidChallenge) {
		t.Fatal("token accepted after email changed", err)
	}
	if _, err = owner.Exec(ctx, `UPDATE users SET email=$2 WHERE id=$1`, s.User.ID, s.User.Email); err != nil {
		t.Fatal(err)
	}
	if _, err = owner.Exec(ctx, `UPDATE auth_challenges SET created_at=now()-interval '1 hour',expires_at=now()-interval '1 second' WHERE user_id=$1`, s.User.ID); err != nil {
		t.Fatal(err)
	}
	if err = db.ResetPassword(ctx, next.Token, "synthetic-replacement-password"); !errors.Is(err, postgres.ErrInvalidChallenge) {
		t.Fatal("expired token accepted", err)
	}
	for _, token := range []string{"", strings.Repeat("x", 1000), "malformed"} {
		if err = db.VerifyEmail(ctx, token); !errors.Is(err, postgres.ErrInvalidChallenge) {
			t.Fatal("malformed challenge accepted", err)
		}
	}
}

func TestPasswordChangeInvalidatesPendingChallengesAndDeletionCascades(t *testing.T) {
	db, owner, _, _ := testDB(t)
	ctx := context.Background()
	s, err := db.Register(ctx, "challenge-password@example.test", "synthetic-original-password")
	if err != nil {
		t.Fatal(err)
	}
	reset, err := db.IssuePasswordReset(ctx, s.User.Email)
	if err != nil || reset == nil {
		t.Fatal(err)
	}
	verify, err := db.IssueEmailVerification(ctx, s.Token)
	if err != nil || verify == nil {
		t.Fatal(err)
	}
	if err = db.ChangePassword(ctx, s.Token, "synthetic-original-password", "synthetic-replacement-password"); err != nil {
		t.Fatal(err)
	}
	if err = db.ResetPassword(ctx, reset.Token, "synthetic-old-reset-password"); !errors.Is(err, postgres.ErrInvalidChallenge) {
		t.Fatal("reset token survived password change", err)
	}
	if err = db.VerifyEmail(ctx, verify.Token); !errors.Is(err, postgres.ErrInvalidChallenge) {
		t.Fatal("verification token survived password change", err)
	}
	next, err := db.Login(ctx, s.User.Email, "synthetic-replacement-password")
	if err != nil {
		t.Fatal(err)
	}
	if err = db.DeleteAccount(ctx, next.Token, "synthetic-replacement-password"); err != nil {
		t.Fatal(err)
	}
	var count int
	if err = owner.QueryRow(ctx, `SELECT count(*) FROM auth_challenges WHERE user_id=$1`, s.User.ID).Scan(&count); err != nil || count != 0 {
		t.Fatal("account deletion left identity credentials", err)
	}
}

func TestConcurrentChallengeRedemptionSucceedsOnlyOnce(t *testing.T) {
	for _, purpose := range []string{identitymail.PasswordReset, identitymail.EmailVerification} {
		t.Run(purpose, func(t *testing.T) {
			db, _, _, _ := testDB(t)
			ctx := context.Background()
			s, err := db.Register(ctx, "challenge-race@example.test", "synthetic-original-password")
			if err != nil {
				t.Fatal(err)
			}
			var msg *identitymail.Message
			if purpose == identitymail.PasswordReset {
				msg, err = db.IssuePasswordReset(ctx, s.User.Email)
			} else {
				msg, err = db.IssueEmailVerification(ctx, s.Token)
			}
			if err != nil || msg == nil {
				t.Fatal(err)
			}
			start := make(chan struct{})
			results := make(chan error, 2)
			for range 2 {
				go func() {
					<-start
					if purpose == identitymail.PasswordReset {
						results <- db.ResetPassword(ctx, msg.Token, "synthetic-replacement-password")
					} else {
						results <- db.VerifyEmail(ctx, msg.Token)
					}
				}()
			}
			close(start)
			success := 0
			for range 2 {
				err := <-results
				if err == nil {
					success++
				} else if !errors.Is(err, postgres.ErrInvalidChallenge) {
					t.Fatal(err)
				}
			}
			if success != 1 {
				t.Fatal("token consumed more than once", success)
			}
		})
	}
}
