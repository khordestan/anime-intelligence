package postgres_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/khordestan/anime-intelligence/internal/httpapi"
	"github.com/khordestan/anime-intelligence/internal/platform/identitymail"
)

type captureSender struct{ messages chan identitymail.Message }

func (s captureSender) Send(ctx context.Context, m identitymail.Message) error {
	select {
	case s.messages <- m:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
func awaitMessage(t *testing.T, ch <-chan identitymail.Message) identitymail.Message {
	t.Helper()
	select {
	case msg := <-ch:
		return msg
	case <-time.After(3 * time.Second):
		t.Fatal("identity delivery did not arrive")
		return identitymail.Message{}
	}
}
func TestHTTPIdentityDeliveryDoesNotDiscloseTokensOrAccountExistence(t *testing.T) {
	db, _, _, _ := testDB(t)
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	s, err := db.Register(ctx, "identity-http@example.test", "synthetic-original-password")
	if err != nil {
		t.Fatal(err)
	}
	mail := captureSender{make(chan identitymail.Message, 8)}
	service := identitymail.NewService(db, mail, logger)
	defer service.Close()
	handler := httpapi.NewPostgresRouterWithIdentity(logger, db, service)
	request := func(path, token, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("POST", "/api/v1/auth/"+path, strings.NewReader(body))
		if token != "" {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	missing := request("password-reset/request", "", `{"email":"missing@example.test"}`)
	existing := request("password-reset/request", "", `{"email":"identity-http@example.test"}`)
	if missing.Code != 202 || existing.Code != 202 || missing.Body.String() != existing.Body.String() || existing.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("reset request disclosed account state")
	}
	reset := awaitMessage(t, mail.messages)
	if strings.Contains(existing.Body.String(), reset.Token) || strings.Contains(existing.Body.String(), s.User.ID) {
		t.Fatal("reset token/account ID leaked in response")
	}
	if w := request("email-verification/request", "", ""); w.Code != 401 {
		t.Fatal("verification issuance was public", w.Code)
	}
	if w := request("email-verification/request", s.Token, `{"email":"other@example.test"}`); w.Code != 400 {
		t.Fatal("recipient override accepted", w.Code)
	}
	if w := request("email-verification/request", s.Token, ""); w.Code != 202 {
		t.Fatal(w.Code, w.Body.String())
	}
	verification := awaitMessage(t, mail.messages)
	if verification.Recipient != s.User.Email {
		t.Fatal("verification sent to untrusted recipient")
	}
	body, _ := json.Marshal(map[string]string{"token": verification.Token})
	if w := request("email-verification/confirm", "", string(body)); w.Code != 204 {
		t.Fatal("verify:", w.Code, w.Body.String())
	}
	if w := request("email-verification/confirm", "", string(body)); w.Code != 400 {
		t.Fatal("verification replay accepted", w.Code)
	}
	if user, err := db.Authenticate(ctx, s.Token); err != nil || !user.EmailVerified {
		t.Fatal("verified flag not updated", err)
	}
	body, _ = json.Marshal(map[string]string{"token": reset.Token, "newPassword": "synthetic-replacement-password"})
	if w := request("password-reset/confirm", "", string(body)); w.Code != 204 {
		t.Fatal("reset:", w.Code, w.Body.String())
	}
	if w := request("password-reset/confirm", "", string(body)); w.Code != 400 {
		t.Fatal("reset replay accepted", w.Code)
	}
	if _, err = db.Login(ctx, s.User.Email, "synthetic-replacement-password"); err != nil {
		t.Fatal("replacement login failed", err)
	}
	for _, invalid := range []string{`{"token":"bad","userId":"spoofed"}`, `{"token":"bad"} {}`} {
		if w := request("email-verification/confirm", "", invalid); w.Code != 400 {
			t.Fatal("non-strict identity JSON", w.Code)
		}
	}
}

func TestIdentityDeliveryDisabledFailsClosed(t *testing.T) {
	db, _, _, _ := testDB(t)
	if _, err := db.Register(context.Background(), "known@example.test", "synthetic-original-password"); err != nil {
		t.Fatal(err)
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := httpapi.NewPostgresRouter(logger, db)
	for _, email := range []string{"known@example.test", "missing@example.test"} {
		r := httptest.NewRequest("POST", "/api/v1/auth/password-reset/request", strings.NewReader(`{"email":"`+email+`"}`))
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		if w.Code != 503 || strings.Contains(w.Body.String(), email) {
			t.Fatal("disabled delivery not uniform", w.Code)
		}
	}
}

func TestChallengeIssuanceHourlyCapAndDisabledAccount(t *testing.T) {
	db, owner, _, _ := testDB(t)
	ctx := context.Background()
	s, err := db.Register(ctx, "challenge-quota@example.test", "synthetic-original-password")
	if err != nil {
		t.Fatal(err)
	}
	for range 5 {
		msg, err := db.IssuePasswordReset(ctx, s.User.Email)
		if err != nil || msg == nil {
			t.Fatal("challenge budget ended too soon", err)
		}
		if _, err = owner.Exec(ctx, `UPDATE auth_challenges SET created_at=now()-interval '2 minutes' WHERE user_id=$1`, s.User.ID); err != nil {
			t.Fatal(err)
		}
	}
	if msg, err := db.IssuePasswordReset(ctx, s.User.Email); err != nil || msg != nil {
		t.Fatal("hourly issuance cap exceeded", err)
	}
	if _, err = owner.Exec(ctx, `UPDATE users SET disabled_at=now() WHERE id=$1`, s.User.ID); err != nil {
		t.Fatal(err)
	}
	if msg, err := db.IssuePasswordReset(ctx, s.User.Email); err != nil || msg != nil {
		t.Fatal("disabled account received reset", err)
	}
}
