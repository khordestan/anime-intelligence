package postgres_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/khordestan/anime-intelligence/internal/platform/postgres"
)

func TestSensitivePasswordChecksShareThePersistedFailureBudget(t *testing.T) {
	db, owner, _, runtimeDSN := testDB(t)
	ctx := context.Background()
	const password = "synthetic-reauth-budget-password"
	session, err := db.Register(ctx, "reauth-budget@example.test", password)
	if err != nil {
		t.Fatal(err)
	}
	second, err := postgres.Open(ctx, runtimeDSN)
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	// Start near the threshold to prove these endpoints cannot bypass the
	// already-tested shared login window, without wasting Argon2 test resources.
	if _, err = owner.Exec(ctx, `UPDATE users SET failed_login_count=8,login_failure_started_at=now() WHERE id=$1`, session.User.ID); err != nil {
		t.Fatal(err)
	}
	if err = db.ChangePassword(ctx, session.Token, "incorrect-synthetic-password", "synthetic-replacement-password"); !errors.Is(err, postgres.ErrReauthentication) {
		t.Fatal("change-password failure", err)
	}
	if _, err = db.Authenticate(ctx, session.Token); err != nil {
		t.Fatal("pre-threshold denial revoked the session", err)
	}
	if err = second.DeleteAccount(ctx, session.Token, "incorrect-synthetic-password"); !errors.Is(err, postgres.ErrReauthentication) {
		t.Fatal("deletion failure", err)
	}
	var count int
	var until *time.Time
	if err = owner.QueryRow(ctx, `SELECT failed_login_count,locked_until FROM users WHERE id=$1`, session.User.ID).Scan(&count, &until); err != nil || count != 10 || until == nil {
		t.Fatal("sensitive actions did not persist shared failures", count, until, err)
	}
	lockedUntil := *until
	if _, err = second.Login(ctx, session.User.Email, password); !errors.Is(err, postgres.ErrUnauthorized) {
		t.Fatal("login bypassed reauthentication lock", err)
	}
	if err = db.DeleteAccount(ctx, session.Token, password); !errors.Is(err, postgres.ErrUnauthorized) {
		t.Fatal("deletion bypassed lock", err)
	}
	if err = db.ChangePassword(ctx, session.Token, password, "synthetic-replacement-password"); !errors.Is(err, postgres.ErrUnauthorized) {
		t.Fatal("password change bypassed lock", err)
	}
	if err = owner.QueryRow(ctx, `SELECT failed_login_count,locked_until FROM users WHERE id=$1`, session.User.ID).Scan(&count, &until); err != nil || count != 10 || until == nil || !until.Equal(lockedUntil) {
		t.Fatal("locked action extended/reset lock", err)
	}
	if _, err = owner.Exec(ctx, `UPDATE users SET locked_until=now()-interval '1 second',login_failure_started_at=now()-interval '16 minutes' WHERE id=$1`, session.User.ID); err != nil {
		t.Fatal(err)
	}
	if err = db.ChangePassword(ctx, session.Token, "incorrect-synthetic-password", "synthetic-replacement-password"); !errors.Is(err, postgres.ErrReauthentication) {
		t.Fatal(err)
	}
	if err = owner.QueryRow(ctx, `SELECT failed_login_count,locked_until FROM users WHERE id=$1`, session.User.ID).Scan(&count, &until); err != nil || count != 1 || until != nil {
		t.Fatal("expired reauthentication window did not reset", err)
	}
	if err = second.ChangePassword(ctx, session.Token, password, "synthetic-replacement-password"); err != nil {
		t.Fatal("correct reauthentication blocked", err)
	}
	var started *time.Time
	if err = owner.QueryRow(ctx, `SELECT failed_login_count,locked_until,login_failure_started_at FROM users WHERE id=$1`, session.User.ID).Scan(&count, &until, &started); err != nil || count != 0 || until != nil || started != nil {
		t.Fatal("successful password change retained failure budget", err)
	}
	if _, err = db.Authenticate(ctx, session.Token); !errors.Is(err, postgres.ErrUnauthorized) {
		t.Fatal("password change left old session valid", err)
	}
	if _, err = db.Login(ctx, session.User.Email, "synthetic-replacement-password"); err != nil {
		t.Fatal("replacement credentials rejected", err)
	}
}
