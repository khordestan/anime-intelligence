package postgres_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/khordestan/anime-intelligence/internal/httpapi"
	"github.com/khordestan/anime-intelligence/internal/platform/identitymail"
	"github.com/khordestan/anime-intelligence/internal/platform/postgres"
)

func TestIdentityRequestAdmissionIsAccountIndependentAndRestartSafe(t *testing.T) {
	db, owner, _, dsn := testDB(t)
	ctx := context.Background()
	s, err := db.Register(ctx, "request@example.test", "synthetic-request-password")
	if err != nil {
		t.Fatal(err)
	}
	cipher := envelopeCipher(t, 1)
	outbox, _ := db.IdentityOutbox(cipher)
	u, _ := url.Parse(dsn)
	role := pgx.Identifier{u.User.Username()}.Sanitize()
	if _, err = owner.Exec(ctx, `REVOKE SELECT ON users FROM `+role); err != nil {
		t.Fatal(err)
	}
	// Admission cannot discover known/unknown status: it lacks permission to read users.
	for _, email := range []string{s.User.Email, "unknown@example.test", ""} {
		if err = outbox.SubmitRequest(ctx, identitymail.PasswordReset, email); err != nil {
			t.Fatal("admission looked up account", err)
		}
	}
	if _, err = owner.Exec(ctx, `GRANT SELECT ON users TO `+role); err != nil {
		t.Fatal(err)
	}
	var count int
	if err = owner.QueryRow(ctx, `SELECT count(*) FROM auth_challenges`).Scan(&count); err != nil || count != 0 {
		t.Fatal("admission issued proof synchronously", err)
	}
	if err = owner.QueryRow(ctx, `SELECT count(*) FROM identity_requests WHERE payload IS NOT NULL AND octet_length(payload)=540`).Scan(&count); err != nil || count != 3 {
		t.Fatal("requests not committed", err)
	}
	second, err := postgres.Open(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	resumed, _ := second.IdentityOutbox(cipher)
	for i := 0; i < 3; i++ {
		if worked, err := resumed.ProcessRequest(ctx); !worked || err != nil {
			t.Fatal("restart lost request", worked, err)
		}
	}
	if worked, err := outbox.ProcessRequest(ctx); worked || err != nil {
		t.Fatal("consumed intent replayed", worked, err)
	}
	if err = owner.QueryRow(ctx, `SELECT count(*) FROM identity_requests WHERE completed_at IS NOT NULL AND payload IS NULL AND lease_hash IS NULL`).Scan(&count); err != nil || count != 3 {
		t.Fatal("intent not erased", err)
	}
	if err = owner.QueryRow(ctx, `SELECT count(*) FROM auth_challenges WHERE user_id=$1`, s.User.ID).Scan(&count); err != nil || count != 1 {
		t.Fatal("unknown account issued proof or duplicate work", err)
	}
	runtime, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Close(ctx)
	if err = runtime.QueryRow(ctx, `SELECT count(*) FROM identity_requests`).Scan(&count); err != nil || count != 0 {
		t.Fatal("request inbox exposed outside trusted worker", err)
	}
}
func TestIdentityRequestConsumptionAndIssuanceRollBackTogether(t *testing.T) {
	db, owner, _, dsn := testDB(t)
	ctx := context.Background()
	s, err := db.Register(ctx, "atomic-request@example.test", "synthetic-request-password")
	if err != nil {
		t.Fatal(err)
	}
	outbox, _ := db.IdentityOutbox(envelopeCipher(t, 1))
	if err = outbox.SubmitRequest(ctx, identitymail.EmailVerification, s.Token); err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse(dsn)
	role := pgx.Identifier{u.User.Username()}.Sanitize()
	if _, err = owner.Exec(ctx, `REVOKE INSERT ON identity_outbox FROM `+role); err != nil {
		t.Fatal(err)
	}
	if worked, err := outbox.ProcessRequest(ctx); !worked || !errors.Is(err, identitymail.ErrDelivery) {
		t.Fatal("failure not recorded", worked, err)
	}
	var pending bool
	var count int
	if err = owner.QueryRow(ctx, `SELECT attempts=1 AND completed_at IS NULL AND payload IS NOT NULL AND lease_hash IS NULL FROM identity_requests`).Scan(&pending); err != nil || !pending {
		t.Fatal("failed work consumed request", err)
	}
	if err = owner.QueryRow(ctx, `SELECT count(*) FROM auth_challenges`).Scan(&count); err != nil || count != 0 {
		t.Fatal("partial proof committed", err)
	}
	if _, err = owner.Exec(ctx, `GRANT INSERT ON identity_outbox TO `+role); err != nil {
		t.Fatal(err)
	}
	if worked, err := outbox.ProcessRequest(ctx); worked || err != nil {
		t.Fatal("request backoff bypassed", worked, err)
	}
	if _, err = owner.Exec(ctx, `UPDATE identity_requests SET available_at=now()-interval '1 second'`); err != nil {
		t.Fatal(err)
	}
	if worked, err := outbox.ProcessRequest(ctx); !worked || err != nil {
		t.Fatal("request retry failed", worked, err)
	}
	if err = owner.QueryRow(ctx, `SELECT count(*) FROM auth_challenges`).Scan(&count); err != nil || count != 1 {
		t.Fatal("retry duplicated issuance", err)
	}
	if err = owner.QueryRow(ctx, `SELECT count(*) FROM identity_outbox`).Scan(&count); err != nil || count != 1 {
		t.Fatal("delivery not atomic", err)
	}
}
func TestVerificationRequestStoresDigestAndRechecksSession(t *testing.T) {
	for _, mode := range []string{"valid", "logout", "refresh", "password-change", "deleted", "expired-request", "expired-session"} {
		t.Run(mode, func(t *testing.T) {
			db, owner, _, _ := testDB(t)
			ctx := context.Background()
			s, err := db.Register(ctx, "verify-request@example.test", "synthetic-request-password")
			if err != nil {
				t.Fatal(err)
			}
			cipher := envelopeCipher(t, 1)
			outbox, _ := db.IdentityOutbox(cipher)
			if err = outbox.SubmitRequest(ctx, identitymail.EmailVerification, s.Token); err != nil {
				t.Fatal(err)
			}
			var id string
			var payload []byte
			if err = owner.QueryRow(ctx, `SELECT id::text,payload FROM identity_requests`).Scan(&id, &payload); err != nil {
				t.Fatal(err)
			}
			purpose, value, err := cipher.OpenRequest(id, payload)
			if err != nil || purpose != identitymail.EmailVerification || len(value) != 64 || value == s.Token || bytes.Contains(payload, []byte(s.Token)) {
				t.Fatal("bearer token persisted", err)
			}
			var digest string
			if err = owner.QueryRow(ctx, `SELECT encode(token_hash,'hex') FROM auth_sessions WHERE user_id=$1`, s.User.ID).Scan(&digest); err != nil || value != digest {
				t.Fatal("intent did not bind presented credential", err)
			}
			switch mode {
			case "logout":
				err = db.Logout(ctx, s.Token)
			case "refresh":
				_, err = db.Refresh(ctx, s.Token)
			case "password-change":
				err = db.ChangePassword(ctx, s.Token, "synthetic-request-password", "synthetic-replacement-password")
			case "deleted":
				_, err = owner.Exec(ctx, `DELETE FROM users WHERE id=$1`, s.User.ID)
			case "expired-request":
				_, err = owner.Exec(ctx, `UPDATE identity_requests SET expires_at=now()-interval '1 second'`)
			case "expired-session":
				_, err = owner.Exec(ctx, `UPDATE auth_sessions SET created_at=now()-interval '1 hour',expires_at=now()-interval '1 second' WHERE user_id=$1`, s.User.ID)
			}
			if err != nil {
				t.Fatal(err)
			}
			_, err = outbox.ProcessRequest(ctx)
			if err != nil {
				t.Fatal(err)
			}
			var count int
			if err = owner.QueryRow(ctx, `SELECT count(*) FROM auth_challenges`).Scan(&count); err != nil {
				t.Fatal(err)
			}
			want := 0
			if mode == "valid" {
				want = 1
			}
			if count != want {
				t.Fatal("request bypassed session state", count)
			}
			if err = owner.QueryRow(ctx, `SELECT count(*) FROM identity_requests WHERE payload IS NOT NULL`).Scan(&count); err != nil || count != 0 {
				t.Fatal("completed intent retained ciphertext", err)
			}
		})
	}
}
func TestIdentityRequestCapacityIsSharedAcrossPools(t *testing.T) {
	db, owner, _, dsn := testDB(t)
	ctx := context.Background()
	one, _ := db.IdentityOutbox(envelopeCipher(t, 1))
	second, err := postgres.Open(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	two, _ := second.IdentityOutbox(envelopeCipher(t, 1))
	for i := 0; i < 254; i++ {
		if err = one.SubmitRequest(ctx, identitymail.PasswordReset, "capacity@example.test"); err != nil {
			t.Fatal(err)
		}
	}
	var accepted atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			outbox := one
			if i%2 == 1 {
				outbox = two
			}
			if err := outbox.SubmitRequest(ctx, identitymail.PasswordReset, "unknown@example.test"); err == nil {
				accepted.Add(1)
			} else if !errors.Is(err, identitymail.ErrDelivery) {
				t.Error(err)
			}
		}(i)
	}
	wg.Wait()
	if accepted.Load() != 2 {
		t.Fatal("capacity not shared", accepted.Load())
	}
	if _, err = owner.Exec(ctx, `UPDATE identity_requests SET expires_at=now()-interval '1 second' WHERE id=(SELECT id FROM identity_requests LIMIT 1)`); err != nil {
		t.Fatal(err)
	}
	if err = two.SubmitRequest(ctx, identitymail.PasswordReset, "capacity@example.test"); err != nil {
		t.Fatal("expired intent held capacity", err)
	}
}
func TestHTTPDurableAdmissionFailsClosedOnDatabaseWriteFailure(t *testing.T) {
	db, owner, _, dsn := testDB(t)
	ctx := context.Background()
	s, err := db.Register(ctx, "http-request@example.test", "synthetic-request-password")
	if err != nil {
		t.Fatal(err)
	}
	outbox, _ := db.IdentityOutbox(envelopeCipher(t, 1))
	service := identitymail.NewDurableService(outbox, outboxSender(func(context.Context, identitymail.Message) error { return nil }), slog.New(slog.NewTextHandler(io.Discard, nil)))
	defer service.Close()
	handler := httpapi.NewPostgresRouterWithIdentity(slog.Default(), db, service)
	request := func(email string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("POST", "/api/v1/auth/password-reset/request", strings.NewReader(`{"email":"`+email+`"}`))
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	known, unknown := request(s.User.Email), request("missing@example.test")
	if known.Code != 202 || unknown.Code != 202 || known.Body.String() != unknown.Body.String() || known.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("admission disclosed account state", known.Code, unknown.Code)
	}
	var count int
	if err = owner.QueryRow(ctx, `SELECT count(*) FROM identity_requests`).Scan(&count); err != nil || count != 2 {
		t.Fatal("202 preceded persistence", err)
	}
	u, _ := url.Parse(dsn)
	role := pgx.Identifier{u.User.Username()}.Sanitize()
	if _, err = owner.Exec(ctx, `REVOKE INSERT ON identity_requests FROM `+role); err != nil {
		t.Fatal(err)
	}
	failedKnown, failedUnknown := request(s.User.Email), request("missing@example.test")
	if failedKnown.Code != 503 || failedUnknown.Code != 503 || failedKnown.Body.String() != failedUnknown.Body.String() {
		t.Fatal("write failure acknowledged or disclosed account state")
	}
}

func TestRequestLeasePreventsConcurrentIssuanceAndRecoversAfterCrash(t *testing.T) {
	db, owner, ownerDSN, dsn := testDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	s, err := db.Register(ctx, "lease-request@example.test", "synthetic-request-password")
	if err != nil {
		t.Fatal(err)
	}
	one, _ := db.IdentityOutbox(envelopeCipher(t, 1))
	if err = one.SubmitRequest(ctx, identitymail.PasswordReset, s.User.Email); err != nil {
		t.Fatal(err)
	}
	second, err := postgres.Open(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	two, _ := second.IdentityOutbox(envelopeCipher(t, 1))
	locker, err := pgx.Connect(ctx, ownerDSN)
	if err != nil {
		t.Fatal(err)
	}
	defer locker.Close(context.Background())
	tx, err := locker.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(context.Background())
	if _, err = tx.Exec(ctx, `SELECT id FROM users WHERE id=$1 FOR UPDATE`, s.User.ID); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 2)
	go func() { _, err := one.ProcessRequest(ctx); done <- err }()
	awaitAttempts := func(want int) {
		t.Helper()
		for start := time.Now(); time.Since(start) < 3*time.Second; time.Sleep(10 * time.Millisecond) {
			var attempts int
			if err = owner.QueryRow(ctx, `SELECT attempts FROM identity_requests`).Scan(&attempts); err != nil {
				t.Fatal(err)
			}
			if attempts == want {
				return
			}
		}
		t.Fatal("lease not persisted")
	}
	awaitAttempts(1)
	if worked, err := two.ProcessRequest(ctx); worked || err != nil {
		t.Fatal("active lease claimed twice", worked, err)
	}
	if _, err = owner.Exec(ctx, `UPDATE identity_requests SET lease_until=now()-interval '1 second'`); err != nil {
		t.Fatal(err)
	}
	go func() { _, err := two.ProcessRequest(ctx); done <- err }()
	awaitAttempts(2)
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err = <-done; err != nil {
			t.Fatal(err)
		}
	}
	var count int
	if err = owner.QueryRow(ctx, `SELECT count(*) FROM auth_challenges WHERE user_id=$1`, s.User.ID).Scan(&count); err != nil || count != 1 {
		t.Fatal("reclaimed request duplicated issuance", err)
	}
}

func TestIdentityRequestCleanupAndWrongKeyPreservation(t *testing.T) {
	for _, mode := range []string{"wrong-key", "expired", "exhausted", "final-failure", "retention"} {
		t.Run(mode, func(t *testing.T) {
			db, owner, _, dsn := testDB(t)
			ctx := context.Background()
			session, err := db.Register(ctx, "cleanup-request@example.test", "synthetic-request-password")
			if err != nil {
				t.Fatal(err)
			}
			outbox, _ := db.IdentityOutbox(envelopeCipher(t, 1))
			if err = outbox.SubmitRequest(ctx, identitymail.PasswordReset, session.User.Email); err != nil {
				t.Fatal(err)
			}
			worker := outbox
			switch mode {
			case "wrong-key":
				worker, _ = db.IdentityOutbox(envelopeCipher(t, 2))
			case "expired":
				_, err = owner.Exec(ctx, `UPDATE identity_requests SET expires_at=now()-interval '1 second'`)
			case "exhausted":
				_, err = owner.Exec(ctx, `UPDATE identity_requests SET attempts=5`)
			case "final-failure":
				_, err = owner.Exec(ctx, `UPDATE identity_requests SET attempts=4`)
				if err != nil {
					t.Fatal(err)
				}
				u, _ := url.Parse(dsn)
				_, err = owner.Exec(ctx, `REVOKE INSERT ON identity_outbox FROM `+pgx.Identifier{u.User.Username()}.Sanitize())
			}
			if err != nil {
				t.Fatal(err)
			}
			worked, err := worker.ProcessRequest(ctx)
			if mode == "wrong-key" {
				if worked || err != identitymail.ErrEnvelope {
					t.Fatal("wrong key accepted", worked, err)
				}
				var preserved bool
				if err = owner.QueryRow(ctx, `SELECT payload IS NOT NULL AND attempts=0 FROM identity_requests`).Scan(&preserved); err != nil || !preserved {
					t.Fatal("wrong key destroyed intent", err)
				}
				if worked, err = outbox.ProcessRequest(ctx); !worked || err != nil {
					t.Fatal("key correction did not resume intent", worked, err)
				}
			} else if mode == "final-failure" {
				if !worked || err != identitymail.ErrDelivery {
					t.Fatal("attempt limit failure", worked, err)
				}
			} else if err != nil {
				t.Fatal(err)
			}
			var retained int
			if err = owner.QueryRow(ctx, `SELECT count(*) FROM identity_requests WHERE payload IS NOT NULL`).Scan(&retained); err != nil || retained != 0 {
				t.Fatal("terminal intent retained private payload", err)
			}
			if mode == "retention" {
				if _, err = owner.Exec(ctx, `UPDATE identity_requests SET completed_at=now()-interval '2 days'`); err != nil {
					t.Fatal(err)
				}
				if _, err = outbox.ProcessRequest(ctx); err != nil {
					t.Fatal(err)
				}
				if err = owner.QueryRow(ctx, `SELECT count(*) FROM identity_requests`).Scan(&retained); err != nil || retained != 0 {
					t.Fatal("old request metadata retained", err)
				}
			}
		})
	}
}
