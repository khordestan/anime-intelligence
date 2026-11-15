package postgres_test

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"net/url"
	"os"
	"testing"

	"github.com/khordestan/anime-intelligence/internal/platform/identitymail"
	"github.com/khordestan/anime-intelligence/internal/platform/postgres"
)

func TestDeliveryOutcomesDescribeActualTerminalResult(t *testing.T) {
	for _, mode := range []string{"accepted", "accepted-fifth", "exhausted", "expired", "invalidated", "wrong-key", "retry"} {
		t.Run(mode, func(t *testing.T) {
			db, owner, _, _ := testDB(t)
			ctx := context.Background()
			s, err := db.Register(ctx, "outcome@example.test", "synthetic-outcome-password")
			if err != nil {
				t.Fatal(err)
			}
			outbox, _ := db.IdentityOutbox(envelopeCipher(t, 1))
			if _, err = outbox.IssueEmailVerification(ctx, s.Token); err != nil {
				t.Fatal(err)
			}
			switch mode {
			case "accepted-fifth", "exhausted":
				_, err = owner.Exec(ctx, `UPDATE identity_outbox SET attempts=4`)
			case "expired":
				_, err = owner.Exec(ctx, `UPDATE identity_outbox SET expires_at=now()-interval '1 second'`)
			case "invalidated":
				_, err = owner.Exec(ctx, `UPDATE auth_challenges SET revoked_at=now()`)
			case "wrong-key":
				outbox, _ = db.IdentityOutbox(envelopeCipher(t, 2))
			}
			if err != nil {
				t.Fatal(err)
			}
			sender := outboxSender(func(context.Context, identitymail.Message) error {
				if mode == "exhausted" || mode == "retry" {
					return errors.New("private provider details")
				}
				return nil
			})
			_, err = outbox.DeliverNext(ctx, sender)
			if mode == "wrong-key" {
				if err != identitymail.ErrEnvelope {
					t.Fatal(err)
				}
			} else if mode == "exhausted" || mode == "retry" {
				if err != identitymail.ErrDelivery {
					t.Fatal(err)
				}
			} else if err != nil {
				t.Fatal(err)
			}
			var outcome string
			var terminal bool
			if err = owner.QueryRow(ctx, `SELECT COALESCE(outcome,''),completed_at IS NOT NULL FROM identity_outbox`).Scan(&outcome, &terminal); err != nil {
				t.Fatal(err)
			}
			want := map[string]string{"accepted": "ACCEPTED", "accepted-fifth": "ACCEPTED", "exhausted": "EXHAUSTED", "expired": "EXPIRED", "invalidated": "INVALIDATED", "wrong-key": "", "retry": ""}[mode]
			if outcome != want || terminal != (want != "") {
				t.Fatalf("outcome=%s terminal=%v want=%s", outcome, terminal, want)
			}
			diagnostics, err := db.IdentityQueueDiagnostics(ctx)
			if err != nil {
				t.Fatal(err)
			}
			q := diagnostics.Deliveries.Outcomes
			if (want == "ACCEPTED" && q.Accepted != 1) || (want == "EXHAUSTED" && q.Exhausted != 1) || (want == "EXPIRED" && q.Expired != 1) || (want == "INVALIDATED" && q.Invalidated != 1) {
				t.Fatal("outcome missing from diagnostics", q)
			}
		})
	}
}
func TestRequestOutcomesKeepNoActionReasonsPrivate(t *testing.T) {
	for _, mode := range []string{"issued", "unknown", "already-verified", "cooldown", "revoked-session", "expired", "exhausted"} {
		t.Run(mode, func(t *testing.T) {
			db, owner, _, _ := testDB(t)
			ctx := context.Background()
			s, err := db.Register(ctx, "request-outcome@example.test", "synthetic-outcome-password")
			if err != nil {
				t.Fatal(err)
			}
			outbox, _ := db.IdentityOutbox(envelopeCipher(t, 1))
			purpose, value := identitymail.EmailVerification, s.Token
			switch mode {
			case "unknown":
				purpose, value = identitymail.PasswordReset, "unknown@example.test"
			case "already-verified":
				verifyAccount(t, db, s.Token)
			case "cooldown":
				if _, err = db.IssueEmailVerification(ctx, s.Token); err != nil {
					t.Fatal(err)
				}
			}
			if err = outbox.SubmitRequest(ctx, purpose, value); err != nil {
				t.Fatal(err)
			}
			switch mode {
			case "revoked-session":
				err = db.Logout(ctx, s.Token)
			case "expired":
				_, err = owner.Exec(ctx, `UPDATE identity_requests SET expires_at=now()-interval '1 second'`)
			case "exhausted":
				_, err = owner.Exec(ctx, `UPDATE identity_requests SET attempts=5`)
			}
			if err != nil {
				t.Fatal(err)
			}
			if _, err = outbox.ProcessRequest(ctx); err != nil {
				t.Fatal(err)
			}
			var outcome string
			if err = owner.QueryRow(ctx, `SELECT outcome FROM identity_requests`).Scan(&outcome); err != nil {
				t.Fatal(err)
			}
			want := "NO_ACTION"
			switch mode {
			case "issued":
				want = "ISSUED"
			case "expired":
				want = "EXPIRED"
			case "exhausted":
				want = "EXHAUSTED"
			}
			if outcome != want {
				t.Fatal("request outcome", outcome, want)
			}
			snapshot, err := db.IdentityQueueDiagnostics(ctx)
			if err != nil {
				t.Fatal(err)
			}
			q := snapshot.Requests.Outcomes
			if (want == "NO_ACTION" && q.NoAction != 1) || (want == "ISSUED" && q.Issued != 1) || (want == "EXPIRED" && q.Expired != 1) || (want == "EXHAUSTED" && q.Exhausted != 1) {
				t.Fatal("request outcome omitted", q)
			}
		})
	}
}
func TestOutcomeMigrationPreservesLegacyUncertaintyAndRejectsMissingSchema(t *testing.T) {
	db, owner, _, runtimeDSN := testDB(t)
	ctx := context.Background()
	outbox, _ := db.IdentityOutbox(envelopeCipher(t, 1))
	if err := outbox.SubmitRequest(ctx, identitymail.PasswordReset, "legacy@example.test"); err != nil {
		t.Fatal(err)
	}
	if _, err := owner.Exec(ctx, `UPDATE identity_requests SET payload=NULL,completed_at=now()`); err != nil {
		t.Fatal(err)
	}
	legacy, err := db.Register(ctx, "legacy-outcome@example.test", "synthetic-legacy-password")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = outbox.IssueEmailVerification(ctx, legacy.Token); err != nil {
		t.Fatal(err)
	}
	if _, err = owner.Exec(ctx, `UPDATE identity_outbox SET payload=NULL,completed_at=now()`); err != nil {
		t.Fatal(err)
	}
	diagnostics, err := db.IdentityQueueDiagnostics(ctx)
	if err != nil || diagnostics.Requests.Outcomes.LegacyUnknown != 1 || diagnostics.Deliveries.Outcomes.LegacyUnknown != 1 {
		t.Fatal("rolling old-writer completion misclassified", err)
	}
	// Run down/up directly to verify historical completed rows get UNKNOWN,
	// including when forced RLS applies to a non-superuser migration owner.
	down, err := os.ReadFile("../../../migrations/000010_identity_queue_outcomes.down.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = owner.Exec(ctx, string(down)); err != nil {
		t.Fatal(err)
	}
	if unsafe, err := postgres.Open(ctx, runtimeDSN); err == nil {
		unsafe.Close()
		t.Fatal("missing outcome schema accepted")
	}
	up, err := os.ReadFile("../../../migrations/000010_identity_queue_outcomes.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse(runtimeDSN)
	migrationRole := u.User.Username() + "_migration"
	quoted := pgx.Identifier{migrationRole}.Sanitize()
	if _, err = owner.Exec(ctx, `CREATE ROLE `+quoted+` NOLOGIN NOSUPERUSER NOBYPASSRLS`); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = owner.Exec(context.Background(), `REASSIGN OWNED BY `+quoted+` TO CURRENT_USER`)
		_, _ = owner.Exec(context.Background(), `DROP OWNED BY `+quoted)
		_, _ = owner.Exec(context.Background(), `DROP ROLE IF EXISTS `+quoted)
	})
	if _, err = owner.Exec(ctx, `GRANT USAGE,CREATE ON SCHEMA public TO `+quoted+`; ALTER TABLE identity_requests OWNER TO `+quoted+`; ALTER TABLE identity_outbox OWNER TO `+quoted); err != nil {
		t.Fatal(err)
	}
	migrationTx, err := owner.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer migrationTx.Rollback(context.Background())
	if _, err = migrationTx.Exec(ctx, `SET LOCAL ROLE `+quoted); err != nil {
		t.Fatal(err)
	}
	if _, err = migrationTx.Exec(ctx, string(up)); err != nil {
		t.Fatal(err)
	}
	if err = migrationTx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	var outcome string
	if err = owner.QueryRow(ctx, `SELECT outcome FROM identity_requests`).Scan(&outcome); err != nil || outcome != "LEGACY_UNKNOWN" {
		t.Fatal("legacy backfill guessed outcome", outcome, err)
	}
	if err = owner.QueryRow(ctx, `SELECT outcome FROM identity_outbox`).Scan(&outcome); err != nil || outcome != "LEGACY_UNKNOWN" {
		t.Fatal("legacy delivery backfill guessed outcome", outcome, err)
	}
	for _, query := range []string{
		`UPDATE identity_requests SET outcome='recipient@example.test'`,
		`UPDATE identity_requests SET outcome='ACCEPTED'`,
		`UPDATE identity_requests SET outcome='ISSUED',completed_at=NULL`,
		`UPDATE identity_outbox SET outcome='ISSUED'`,
		`UPDATE identity_outbox SET outcome='ACCEPTED',completed_at=NULL`,
	} {
		if _, err = owner.Exec(ctx, query); err == nil {
			t.Fatal("unbounded or inconsistent outcome accepted")
		}
	}
}
