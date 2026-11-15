package postgres_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/khordestan/anime-intelligence/internal/httpapi"
	"github.com/khordestan/anime-intelligence/internal/platform/password"
	"github.com/khordestan/anime-intelligence/internal/platform/postgres"
)

func TestPasswordChangeReauthenticatesAndRevokesAllSessions(t *testing.T) {
	db, owner, _, _ := testDB(t)
	ctx := context.Background()
	s, err := db.Register(ctx, "password-one@example.test", "synthetic-old-password")
	if err != nil {
		t.Fatal(err)
	}
	other, err := db.Login(ctx, s.User.Email, "synthetic-old-password")
	if err != nil {
		t.Fatal(err)
	}
	foreign, err := db.Register(ctx, "password-two@example.test", "synthetic-foreign-password")
	if err != nil {
		t.Fatal(err)
	}
	if err = db.ChangePassword(ctx, s.Token, "incorrect-password", "synthetic-new-password"); !errors.Is(err, postgres.ErrReauthentication) {
		t.Fatal("changed password without reauthentication", err)
	}
	if err = db.ChangePassword(ctx, s.Token, "synthetic-old-password", "short"); !errors.Is(err, password.ErrInvalid) {
		t.Fatal("accepted weak/invalid new password", err)
	}
	for _, token := range []string{s.Token, other.Token} {
		if _, err = db.Authenticate(ctx, token); err != nil {
			t.Fatal("failed change revoked session", err)
		}
	}
	if err = db.ChangePassword(ctx, s.Token, "synthetic-old-password", "synthetic-new-password"); err != nil {
		t.Fatal(err)
	}
	for _, token := range []string{s.Token, other.Token} {
		if _, err = db.Authenticate(ctx, token); !errors.Is(err, postgres.ErrUnauthorized) {
			t.Fatal("existing credential survived password change", err)
		}
	}
	if _, err = db.Login(ctx, s.User.Email, "synthetic-old-password"); !errors.Is(err, postgres.ErrUnauthorized) {
		t.Fatal("old password accepted", err)
	}
	if _, err = db.Login(ctx, s.User.Email, "synthetic-new-password"); err != nil {
		t.Fatal("new password failed", err)
	}
	if _, err = db.Authenticate(ctx, foreign.Token); err != nil {
		t.Fatal("changed another account's credentials", err)
	}
	if _, err = db.Login(ctx, foreign.User.Email, "synthetic-foreign-password"); err != nil {
		t.Fatal("changed another account's password", err)
	}
	var stored string
	if err = owner.QueryRow(ctx, `SELECT password_hash FROM users WHERE id=$1`, s.User.ID).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if stored == "synthetic-new-password" || !password.Verify(stored, "synthetic-new-password") {
		t.Fatal("new password was not stored as a valid hash")
	}
}

func TestConcurrentPasswordChangesDoNotLeaveOldCredentials(t *testing.T) {
	db, _, _, _ := testDB(t)
	ctx := context.Background()
	one, err := db.Register(ctx, "password-race@example.test", "synthetic-old-password")
	if err != nil {
		t.Fatal(err)
	}
	two, err := db.Login(ctx, one.User.Email, "synthetic-old-password")
	if err != nil {
		t.Fatal(err)
	}
	start := make(chan struct{})
	results := make(chan error, 2)
	for _, token := range []string{one.Token, two.Token} {
		go func(token string) {
			<-start
			results <- db.ChangePassword(ctx, token, "synthetic-old-password", "synthetic-new-password")
		}(token)
	}
	close(start)
	success := 0
	for range 2 {
		err := <-results
		if err == nil {
			success++
		} else if !errors.Is(err, postgres.ErrUnauthorized) {
			t.Fatal("concurrent change failed unexpectedly", err)
		}
	}
	if success != 1 {
		t.Fatal("expected one serialized change", success)
	}
	for _, token := range []string{one.Token, two.Token} {
		if _, err = db.Authenticate(ctx, token); !errors.Is(err, postgres.ErrUnauthorized) {
			t.Fatal("old session survived", err)
		}
	}
	if _, err = db.Login(ctx, one.User.Email, "synthetic-new-password"); err != nil {
		t.Fatal(err)
	}
}

func TestHTTPPasswordChangeRejectsInvalidBodiesAndOwnerOverrides(t *testing.T) {
	db, _, _, _ := testDB(t)
	ctx := context.Background()
	s, err := db.Register(ctx, "http-password@example.test", "synthetic-old-password")
	if err != nil {
		t.Fatal(err)
	}
	handler := httpapi.NewPostgresRouter(slog.New(slog.NewTextHandler(io.Discard, nil)), db)
	call := func(token, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("POST", "/api/v1/auth/password", strings.NewReader(body))
		if token != "" {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	if w := call("", `{}`); w.Code != 401 {
		t.Fatal("unprotected route", w.Code)
	}
	for _, body := range []string{
		`{"currentPassword":"synthetic-old-password","newPassword":"synthetic-new-password","userId":"spoofed"}`,
		`{"currentPassword":"synthetic-old-password","newPassword":"synthetic-new-password"} {}`,
	} {
		if w := call(s.Token, body); w.Code != 400 {
			t.Fatal("accepted malformed/body owner override", w.Code)
		}
	}
	if w := call(s.Token, `{"currentPassword":"incorrect-password","newPassword":"synthetic-new-password"}`); w.Code != 401 || !strings.Contains(w.Body.String(), "invalid_current_password") {
		t.Fatal("accepted wrong current password", w.Code)
	}
	if w := call(s.Token, `{"currentPassword":"synthetic-old-password","newPassword":"short"}`); w.Code != 400 {
		t.Fatal("accepted invalid replacement", w.Code)
	}
	if w := call(s.Token, `{"currentPassword":"synthetic-old-password","newPassword":"synthetic-new-password"}`); w.Code != 204 || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("password change response", w.Code, w.Body.String())
	}
	if w := call(s.Token, `{"currentPassword":"synthetic-new-password","newPassword":"another-synthetic-password"}`); w.Code != 401 {
		t.Fatal("old session survived HTTP password change", w.Code)
	}
}

func TestPasswordChangeClosesRefreshAndOldLoginRaces(t *testing.T) {
	for _, kind := range []string{"refresh", "login"} {
		t.Run(kind, func(t *testing.T) {
			db, _, _, _ := testDB(t)
			ctx := context.Background()
			current, err := db.Register(ctx, "password-credential-race@example.test", "synthetic-old-password")
			if err != nil {
				t.Fatal(err)
			}
			remote, err := db.Login(ctx, current.User.Email, "synthetic-old-password")
			if err != nil {
				t.Fatal(err)
			}
			start := make(chan struct{})
			changed := make(chan error, 1)
			type result struct {
				session postgres.Session
				err     error
			}
			issued := make(chan result, 1)
			go func() {
				<-start
				changed <- db.ChangePassword(ctx, current.Token, "synthetic-old-password", "synthetic-new-password")
			}()
			go func() {
				<-start
				var s postgres.Session
				var err error
				if kind == "refresh" {
					s, err = db.Refresh(ctx, remote.Token)
				} else {
					s, err = db.Login(ctx, current.User.Email, "synthetic-old-password")
				}
				issued <- result{s, err}
			}()
			close(start)
			if err := <-changed; err != nil {
				t.Fatal("password change:", err)
			}
			raced := <-issued
			if raced.err != nil && !errors.Is(raced.err, postgres.ErrUnauthorized) {
				t.Fatal("racing credential operation:", raced.err)
			}
			for _, token := range []string{current.Token, remote.Token, raced.session.Token} {
				if _, err = db.Authenticate(ctx, token); !errors.Is(err, postgres.ErrUnauthorized) {
					t.Fatal("old credential escaped password change", err)
				}
			}
			if _, err = db.Login(ctx, current.User.Email, "synthetic-new-password"); err != nil {
				t.Fatal("replacement password failed", err)
			}
		})
	}
}
