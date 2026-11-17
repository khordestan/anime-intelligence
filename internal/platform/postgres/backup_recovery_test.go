package postgres_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/khordestan/anime-intelligence/internal/platform/identitymail"
	"github.com/khordestan/anime-intelligence/internal/platform/postgres"
	"github.com/khordestan/anime-intelligence/migrations"
)

// Opt-in because this exercise needs real pg_dump/pg_restore binaries as well
// as a disposable administrator database. CI runs it in a PostgreSQL 17 image.
// It never dumps TEST_DATABASE_URL: testDB provisions a synthetic source first.
func TestBackupRestoreRecovery(t *testing.T) {
	if os.Getenv("TEST_POSTGRES_RECOVERY") != "1" {
		t.Skip("TEST_POSTGRES_RECOVERY=1 enables the isolated logical recovery drill")
	}
	if os.Getenv("TEST_DATABASE_URL") == "" {
		t.Fatal("enabled recovery drill requires a disposable TEST_DATABASE_URL")
	}
	for _, program := range []string{"pg_dump", "pg_restore"} {
		if _, err := exec.LookPath(program); err != nil {
			t.Fatal("recovery drill requires PostgreSQL client tools")
		}
	}
	db, owner, ownerDSN, runtimeDSN := testDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	const password = "synthetic-recovery-password"
	one, err := db.Register(ctx, "restore-one@example.test", password)
	if err != nil {
		t.Fatal(err)
	}
	two, err := db.Register(ctx, "restore-two@example.test", password)
	if err != nil {
		t.Fatal(err)
	}
	mail, err := db.Register(ctx, "restore-mail@example.test", password)
	if err != nil {
		t.Fatal(err)
	}
	disabled, err := db.Register(ctx, "restore-disabled@example.test", password)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = owner.Exec(ctx, `UPDATE users SET disabled_at=now() WHERE id=$1`, disabled.User.ID); err != nil {
		t.Fatal(err)
	}
	verifyAccount(t, db, one.Token)
	verifyAccount(t, db, two.Token)
	const anime = "10000000-0000-4000-8000-000000000001"
	for i, s := range []postgres.Session{one, two} {
		if _, err = db.Repository(s.Token).Add(ctx, anime, "WATCHING", i+2); err != nil {
			t.Fatal(err)
		}
		if _, ok, err := db.Repository(s.Token).Rate(ctx, anime, i+3); err != nil || !ok {
			t.Fatal("seed recovery rating", err)
		}
	}
	for _, query := range []string{
        `INSERT INTO watch_cycles(user_id,anime_id,cycle_number,episodes_watched,total_episodes)
          SELECT a.uid,'10000000-0000-4000-8000-000000000001',1,12,12 FROM (VALUES($1::uuid),($2::uuid)) a(uid)`,
        `UPDATE user_anime SET watch_cycle=2 WHERE user_id=$1 OR user_id=$2`,
		`INSERT INTO user_preferences(user_id,feature_id,preference_weight,origin)
          SELECT a.uid,f.id,0.5,'DECLARED' FROM (VALUES($1::uuid),($2::uuid)) a(uid)
          CROSS JOIN (SELECT id FROM anime_features ORDER BY feature_key LIMIT 1) f`,
		`INSERT INTO recommendations(user_id,anime_id,category,score,algorithm_version)
          SELECT a.uid,'10000000-0000-4000-8000-000000000001','PERSONAL_MATCH',0.75,'synthetic-recovery-v1'
          FROM (VALUES($1::uuid),($2::uuid)) a(uid)`,
		`INSERT INTO recommendation_feedback(recommendation_id,user_id,feedback)
          SELECT id,user_id,'INTERESTED' FROM recommendations WHERE user_id=$1 OR user_id=$2`,
		`INSERT INTO user_follows(user_id,anime_id)
          SELECT a.uid,'10000000-0000-4000-8000-000000000001' FROM (VALUES($1::uuid),($2::uuid)) a(uid)`,
		`INSERT INTO notifications(user_id,title,body)
          VALUES($1,'Synthetic notice','Synthetic body'),($2,'Synthetic notice','Synthetic body')`,
	} {
		if _, err = owner.Exec(ctx, query, one.User.ID, two.User.ID); err != nil {
			t.Fatal("seed recovery resources", err)
		}
	}
	before, err := db.ExportAccount(ctx, one.Token)
	if err != nil {
		t.Fatal(err)
	}
	if len(before.Data) != 9 {
		t.Fatal("incomplete recovery export fixture")
	}
	for _, rows := range before.Data {
		if string(rows) == "[]" {
			t.Fatal("empty recovery export fixture")
		}
	}
	revoked, err := db.Login(ctx, one.User.Email, password)
	if err != nil {
		t.Fatal(err)
	}
	if err = db.Logout(ctx, revoked.Token); err != nil {
		t.Fatal(err)
	}
	oldCipher := envelopeCipher(t, 1)
	outbox, _ := db.IdentityOutbox(oldCipher)
	if _, err = outbox.IssueEmailVerification(ctx, mail.Token); err != nil {
		t.Fatal(err)
	}
	if err = outbox.SubmitRequest(ctx, identitymail.PasswordReset, one.User.Email); err != nil {
		t.Fatal(err)
	}
	var challengeID string
	var payload []byte
	if err = owner.QueryRow(ctx, `SELECT challenge_id::text,payload FROM identity_outbox WHERE user_id=$1`, mail.User.ID).Scan(&challengeID, &payload); err != nil {
		t.Fatal(err)
	}
	oldMessage, err := oldCipher.Open(challengeID, payload)
	if err != nil {
		t.Fatal(err)
	}
	archive := filepath.Join(t.TempDir(), "synthetic.dump")
	file, err := os.OpenFile(archive, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal("create private synthetic archive")
	}
	dump := recoveryCommand(t, ctx, "pg_dump", ownerDSN, "--format=custom", "--no-owner", "--no-privileges")
	dump.Stdout = file
	dumpErr := dump.Run()
	closeErr := file.Close()
	if dumpErr != nil || closeErr != nil {
		t.Fatal("synthetic pg_dump failed (connection and tool output redacted)")
	}
	// The archive must remain a historical snapshot. These later changes are
	// deliberately absent: disaster recovery must reconcile them out of band.
	if err = db.ChangePassword(ctx, one.Token, password, "synthetic-post-backup-password"); err != nil {
		t.Fatal(err)
	}
	if err = db.DeleteAccount(ctx, two.Token, password); err != nil {
		t.Fatal(err)
	}

	t.Run("compatible-key-resume", func(t *testing.T) {
		restored, restoredOwner, restoredRuntime := restoreSyntheticArchive(t, ctx, archive, ownerDSN, runtimeDSN)
		for _, token := range []string{one.Token, two.Token} {
			if _, err := restored.Authenticate(ctx, token); err != nil {
				t.Fatal("snapshot session missing", err)
			}
		}
		if _, err := restored.Authenticate(ctx, revoked.Token); !errors.Is(err, postgres.ErrUnauthorized) {
			t.Fatal("pre-backup revoked session revived", err)
		}
		after, err := restored.ExportAccount(ctx, one.Token)
		if err != nil || after.User != before.User || !reflect.DeepEqual(after.Data, before.Data) {
			t.Fatal("restored export differs from backup snapshot", err)
		}
		if _, err = restored.Login(ctx, one.User.Email, password); err != nil {
			t.Fatal("snapshot password hash missing", err)
		}
		assertRestoredIsolation(t, ctx, restored, restoredRuntime, one.Token, two.Token)

		newOnly, _ := restored.IdentityOutbox(envelopeCipher(t, 2))
		forbidden := outboxSender(func(context.Context, identitymail.Message) error {
			t.Fatal("mail sent without compatible backup key")
			return nil
		})
		if worked, err := newOnly.ProcessRequest(ctx); worked || err != identitymail.ErrEnvelope {
			t.Fatal("missing old request key did not fail closed", worked, err)
		}
		if worked, err := newOnly.DeliverNext(ctx, forbidden); worked || err != identitymail.ErrEnvelope {
			t.Fatal("missing old mail key did not fail closed", worked, err)
		}
		var unchanged bool
		if err = restoredOwner.QueryRow(ctx, `SELECT payload=$1 FROM identity_outbox WHERE challenge_id=$2`, payload, challengeID).Scan(&unchanged); err != nil || !unchanged {
			t.Fatal("undecryptable restored mail was changed", err)
		}
		var requestPreserved bool
		if err = restoredOwner.QueryRow(ctx, `SELECT count(*)=1 AND bool_and(payload IS NOT NULL AND completed_at IS NULL AND attempts=0) FROM identity_requests`).Scan(&requestPreserved); err != nil || !requestPreserved {
			t.Fatal("undecryptable restored request was changed", err)
		}
		ring, err := identitymail.NewEnvelopeKeyring(
			base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{2}, 32)),
			[]string{base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{1}, 32))})
		if err != nil {
			t.Fatal(err)
		}
		compatible, _ := restored.IdentityOutbox(ring)
		if worked, err := compatible.ProcessRequest(ctx); !worked || err != nil {
			t.Fatal("restored intent did not resume", worked, err)
		}
		delivered := make(map[string]identitymail.Message)
		sender := outboxSender(func(_ context.Context, m identitymail.Message) error {
			delivered[m.Recipient] = m
			return nil
		})
		for range 2 {
			if worked, err := compatible.DeliverNext(ctx, sender); !worked || err != nil {
				t.Fatal("restored delivery did not resume", worked, err)
			}
		}
		if delivered[mail.User.Email] != oldMessage || len(delivered) != 2 {
			t.Fatal("restored delivery changed original proof or lost work")
		}
		if err = restored.VerifyEmail(ctx, oldMessage.Token); err != nil {
			t.Fatal("restored verification proof failed", err)
		}
		if err = restored.ResetPassword(ctx, delivered[one.User.Email].Token, "synthetic-restored-reset"); err != nil {
			t.Fatal("restored reset proof failed", err)
		}
		if err = restored.ResetPassword(ctx, delivered[one.User.Email].Token, "synthetic-replay-password"); !errors.Is(err, postgres.ErrInvalidChallenge) {
			t.Fatal("restored proof replay accepted", err)
		}
	})

	t.Run("offline-fenced", func(t *testing.T) {
		restored, restoredOwner, _ := restoreSyntheticArchive(t, ctx, archive, ownerDSN, runtimeDSN)
		script, err := os.ReadFile("../../../deployments/recovery-fence.sql")
		if err != nil {
			t.Fatal(err)
		}
		// A restricted role cannot partially fence a database. Failure rolls
		// back all effects, and no open failed transaction reaches the pool.
		fenceRuntime, err := pgx.Connect(ctx, recoveryRuntimeDSN(t, runtimeDSN, restoredOwner.Config().Database))
		if err != nil {
			t.Fatal(err)
		}
		if _, err = fenceRuntime.Exec(ctx, string(script)); err == nil {
			t.Fatal("runtime role performed administrator recovery")
		}
		_ = fenceRuntime.Close(ctx)
		if _, err = restored.Authenticate(ctx, one.Token); err != nil {
			t.Fatal("failed restricted fence changed account state", err)
		}
		// Fail after session/account changes and prove the whole fence rolls
		// back. Explicit ROLLBACK also releases locks after a psql-style error.
		if _, err = restoredOwner.Exec(ctx, `CREATE FUNCTION public.synthetic_fence_failure() RETURNS trigger
          LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'synthetic failure'; END $$;
          CREATE TRIGGER synthetic_fence_failure BEFORE DELETE ON identity_requests
          FOR EACH ROW EXECUTE FUNCTION public.synthetic_fence_failure()`); err != nil {
			t.Fatal(err)
		}
		if _, err = restoredOwner.Exec(ctx, string(script)); err == nil {
			t.Fatal("injected fence failure not reported")
		}
		if _, err = restoredOwner.Exec(ctx, `ROLLBACK;
          DROP TRIGGER synthetic_fence_failure ON identity_requests;
          DROP FUNCTION public.synthetic_fence_failure()`); err != nil {
			t.Fatal(err)
		}
		if _, err = restored.Authenticate(ctx, one.Token); err != nil {
			t.Fatal("failed administrator fence committed partial account changes", err)
		}
		var retained int
		if err = restoredOwner.QueryRow(ctx, `SELECT count(*) FROM identity_outbox WHERE payload IS NOT NULL`).Scan(&retained); err != nil || retained != 1 {
			t.Fatal("failed fence lost encrypted delivery", err)
		}
		for range 2 { // Deliberately idempotent when repeated before reopening.
			if _, err = restoredOwner.Exec(ctx, string(script)); err != nil {
				t.Fatal("offline recovery fence failed", err)
			}
		}
		for _, s := range []postgres.Session{one, two, mail, revoked, disabled} {
			if _, err = restored.Authenticate(ctx, s.Token); !errors.Is(err, postgres.ErrUnauthorized) {
				t.Fatal("restored bearer survived fence", err)
			}
			if _, err = restored.Login(ctx, s.User.Email, password); !errors.Is(err, postgres.ErrUnauthorized) {
				t.Fatal("historical password bypassed forced recovery", err)
			}
		}
		if err = restored.VerifyEmail(ctx, oldMessage.Token); !errors.Is(err, postgres.ErrInvalidChallenge) {
			t.Fatal("historical proof survived fence", err)
		}
		for _, table := range []string{"identity_requests", "identity_outbox"} {
			var n int
			if err = restoredOwner.QueryRow(ctx, `SELECT count(*) FROM `+pgx.Identifier{table}.Sanitize()).Scan(&n); err != nil || n != 0 {
				t.Fatal("fence retained queued work", err)
			}
		}
		// Fencing alone cannot identify an account deleted after the snapshot.
		// Reconcile that known deletion before opening fresh recovery intake.
		if _, err = restoredOwner.Exec(ctx, `DELETE FROM users WHERE id=$1`, two.User.ID); err != nil {
			t.Fatal("reconcile post-backup deletion", err)
		}
		if proof, err := restored.IssuePasswordReset(ctx, two.User.Email); err != nil || proof != nil {
			t.Fatal("reconciled deleted account could recover", err)
		}
		if proof, err := restored.IssuePasswordReset(ctx, disabled.User.Email); err != nil || proof != nil {
			t.Fatal("disabled account could recover", err)
		}
		// Fresh recovery needs only a new key, never the discarded backup key.
		fresh, _ := restored.IdentityOutbox(envelopeCipher(t, 2))
		if err = fresh.SubmitRequest(ctx, identitymail.PasswordReset, one.User.Email); err != nil {
			t.Fatal(err)
		}
		if worked, err := fresh.ProcessRequest(ctx); !worked || err != nil {
			t.Fatal("fresh reset not issued after fence", worked, err)
		}
		var message identitymail.Message
		if worked, err := fresh.DeliverNext(ctx, outboxSender(func(_ context.Context, m identitymail.Message) error { message = m; return nil })); !worked || err != nil {
			t.Fatal("fresh reset not delivered", worked, err)
		}
		if message.Recipient != one.User.Email || message.Token == oldMessage.Token {
			t.Fatal("incorrect fresh recovery message")
		}
		if err = restored.ResetPassword(ctx, message.Token, "synthetic-fenced-reset"); err != nil {
			t.Fatal("fresh reset failed to unlock account", err)
		}
		login, err := restored.Login(ctx, one.User.Email, "synthetic-fenced-reset")
		if err != nil {
			t.Fatal("fresh credentials rejected after recovery", err)
		}
		after, err := restored.ExportAccount(ctx, login.Token)
		if err != nil || !reflect.DeepEqual(after.Data, before.Data) {
			t.Fatal("fencing changed product data", err)
		}
		if _, err = restored.Login(ctx, two.User.Email, password); !errors.Is(err, postgres.ErrUnauthorized) {
			t.Fatal("one reset unlocked another account", err)
		}
	})
}

// libpq receives the password only via the child environment, never arguments.
// Tool output is discarded: it can contain connection/row details on failure.
func recoveryCommand(t *testing.T, ctx context.Context, program, dsn string, args ...string) *exec.Cmd {
	t.Helper()
	u, err := url.Parse(dsn)
	if err != nil || u.User == nil {
		t.Fatal("invalid recovery connection")
	}
	password, _ := u.User.Password()
	u.User = url.User(u.User.Username())
	cmd := exec.CommandContext(ctx, program, append([]string{"--dbname=" + u.String(), "--no-password"}, args...)...)
	for _, value := range os.Environ() {
		if !strings.HasPrefix(value, "PG") {
			cmd.Env = append(cmd.Env, value)
		}
	}
	cmd.Env = append(cmd.Env, "PGPASSWORD="+password, "PGPASSFILE="+os.DevNull, "PGCONNECT_TIMEOUT=5")
	return cmd
}

func recoveryRuntimeDSN(t *testing.T, dsn, database string) string {
	t.Helper()
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatal("invalid fixture connection")
	}
	u.Path = "/" + database
	return u.String()
}

func restoreSyntheticArchive(t *testing.T, ctx context.Context, archive, ownerDSN, runtimeDSN string) (*postgres.DB, *pgx.Conn, string) {
	t.Helper()
	admin, err := pgx.Connect(ctx, os.Getenv("TEST_DATABASE_URL"))
	if err != nil {
		t.Fatal("connect recovery administrator")
	}
	suffix := make([]byte, 8)
	if _, err = rand.Read(suffix); err != nil {
		t.Fatal(err)
	}
	name := "anime_restore_" + hex.EncodeToString(suffix)
	t.Cleanup(func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_, _ = admin.Exec(cleanup, `DROP DATABASE IF EXISTS `+pgx.Identifier{name}.Sanitize()+` WITH (FORCE)`)
		_ = admin.Close(cleanup)
	})
	if _, err = admin.Exec(ctx, `CREATE DATABASE `+pgx.Identifier{name}.Sanitize()+` TEMPLATE template0 ENCODING 'UTF8'`); err != nil {
		t.Fatal("create isolated recovery database")
	}
	restoredDSN := recoveryRuntimeDSN(t, ownerDSN, name)
	if err = recoveryCommand(t, ctx, "pg_restore", restoredDSN, "--no-owner", "--no-privileges", "--single-transaction", "--exit-on-error", archive).Run(); err != nil {
		t.Fatal("synthetic pg_restore failed (connection and tool output redacted)")
	}
	owner, err := pgx.Connect(ctx, restoredDSN)
	if err != nil {
		t.Fatal("connect restored owner")
	}
	t.Cleanup(func() { _ = owner.Close(context.Background()) })
	var count int
	if err = owner.QueryRow(ctx, `SELECT count(*) FROM schema_migrations`).Scan(&count); err != nil || count != 20 {
		t.Fatal("migration ledger not restored", err)
	}
	if err = migrations.Apply(ctx, owner); err != nil {
		t.Fatal("restored migration checksum validation failed", err)
	}
	// Archives deliberately exclude ACLs/roles. Reapply the reviewed production
	// grants to the test's existing random restricted role, never the owner.
	runtimeURL, _ := url.Parse(runtimeDSN)
	script, err := os.ReadFile("../../../deployments/runtime-role.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = owner.Exec(ctx, strings.ReplaceAll(string(script), "anime_runtime", runtimeURL.User.Username())); err != nil {
		t.Fatal("reprovision restored runtime grants", err)
	}
	restoredRuntime := recoveryRuntimeDSN(t, runtimeDSN, name)
	db, err := postgres.Open(ctx, restoredRuntime)
	if err != nil {
		t.Fatal("restored restricted runtime startup failed", err)
	}
	t.Cleanup(db.Close)
	if err = db.Ready(ctx); err != nil {
		t.Fatal("restored runtime readiness failed", err)
	}
	return db, owner, restoredRuntime
}

func assertRestoredIsolation(t *testing.T, ctx context.Context, db *postgres.DB, runtimeDSN, one, two string) {
	t.Helper()
	first, err := db.Repository(one).Library(ctx)
	if err != nil || len(first) != 1 || first[0].EpisodesWatched != 2 || first[0].Rating != 3 {
		t.Fatal("first restored library incorrect", err)
	}
	second, err := db.Repository(two).Library(ctx)
	if err != nil || len(second) != 1 || second[0].EpisodesWatched != 3 || second[0].Rating != 4 {
		t.Fatal("second restored library incorrect", err)
	}
	episodes := 99
	if _, ok, err := db.Repository(two).Update(ctx, first[0].ID, nil, &episodes); err != nil || ok {
		t.Fatal("restored cross-account update accepted", err)
	}
	raw, err := pgx.Connect(ctx, runtimeDSN)
	if err != nil {
		t.Fatal("connect restored unscoped role")
	}
	defer raw.Close(ctx)
	for _, table := range []string{"users", "auth_sessions", "auth_challenges", "identity_requests", "identity_outbox", "user_anime", "watch_cycles", "user_ratings", "watch_progress", "user_preferences", "recommendations", "recommendation_feedback", "user_follows", "notifications"} {
		var n int
		if err = raw.QueryRow(ctx, `SELECT count(*) FROM `+pgx.Identifier{table}.Sanitize()).Scan(&n); err != nil || n != 0 {
			t.Fatalf("restored unscoped RLS failed for %s", table)
		}
	}
}
