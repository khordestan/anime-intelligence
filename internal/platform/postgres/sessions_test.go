package postgres_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/khordestan/anime-intelligence/internal/httpapi"
	"github.com/khordestan/anime-intelligence/internal/platform/postgres"
)

func TestRemoteSessionRevocationSurvivesRefresh(t *testing.T) {
	db, _, _, _ := testDB(t)
	ctx := context.Background()
	current, err := db.Register(ctx, "session-owner@example.test", "synthetic-session-password")
	if err != nil {
		t.Fatal(err)
	}
	other, err := db.Login(ctx, current.User.Email, "synthetic-session-password")
	if err != nil {
		t.Fatal(err)
	}
	foreign, err := db.Register(ctx, "session-foreign@example.test", "synthetic-foreign-password")
	if err != nil {
		t.Fatal(err)
	}
	items, more, err := db.Sessions(ctx, current.Token)
	if err != nil || more || len(items) != 2 || !items[0].Current {
		t.Fatal("session listing:", items, more, err)
	}
	target := items[1].ID
	rotated, err := db.Refresh(ctx, other.Token)
	if err != nil {
		t.Fatal(err)
	}
	refreshed, _, err := db.Sessions(ctx, rotated.Token)
	if err != nil || len(refreshed) != 2 || refreshed[0].ID != target || !refreshed[0].Current {
		t.Fatal("refresh changed logical session ID", err)
	}
	if _, err = db.Authenticate(ctx, other.Token); !errors.Is(err, postgres.ErrUnauthorized) {
		t.Fatal("old credential survived rotation", err)
	}
	encoded, err := json.Marshal(items)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"token_hash", "tokenHash", current.Token, other.Token, foreign.User.ID} {
		if strings.Contains(string(encoded), secret) {
			t.Fatal("session list disclosed credential/account data")
		}
	}
	if found, err := db.RevokeSession(ctx, foreign.Token, target); err != nil || found {
		t.Fatal("cross-user revocation accepted", err)
	}
	if _, err = db.Authenticate(ctx, rotated.Token); err != nil {
		t.Fatal("cross-user attempt changed target", err)
	}
	if found, err := db.RevokeSession(ctx, current.Token, target); err != nil || !found {
		t.Fatal("remote revocation:", err)
	}
	if _, err = db.Authenticate(ctx, rotated.Token); !errors.Is(err, postgres.ErrUnauthorized) {
		t.Fatal("replacement token escaped revocation", err)
	}
	if items, more, err = db.Sessions(ctx, current.Token); err != nil || more || len(items) != 1 || !items[0].Current {
		t.Fatal("revoked session remained active", err)
	}
	if found, err := db.RevokeSession(ctx, current.Token, items[0].ID); err != nil || !found {
		t.Fatal("self revocation:", err)
	}
	if _, err = db.Authenticate(ctx, current.Token); !errors.Is(err, postgres.ErrUnauthorized) {
		t.Fatal("self-revoked token accepted", err)
	}
	if _, err = db.Authenticate(ctx, foreign.Token); err != nil {
		t.Fatal("revocation affected another user", err)
	}
}

func TestSessionListingBoundAndRevokeAllOthers(t *testing.T) {
	db, owner, _, _ := testDB(t)
	ctx := context.Background()
	s, err := db.Register(ctx, "many-sessions@example.test", "synthetic-session-password")
	if err != nil {
		t.Fatal(err)
	}
	foreign, err := db.Register(ctx, "many-foreign@example.test", "synthetic-foreign-password")
	if err != nil {
		t.Fatal(err)
	}
	_, err = owner.Exec(ctx, `INSERT INTO auth_sessions(user_id,token_hash,expires_at)
 SELECT $1,decode(lpad(to_hex(g),64,'0'),'hex'),now()+interval '1 hour' FROM generate_series(1,105) g`, s.User.ID)
	if err != nil {
		t.Fatal(err)
	}
	_, err = owner.Exec(ctx, `INSERT INTO auth_sessions(user_id,token_hash,created_at,expires_at,revoked_at) VALUES
 ($1,decode(repeat('aa',32),'hex'),now()-interval '2 days',now()-interval '1 day',NULL),
 ($1,decode(repeat('bb',32),'hex'),now()-interval '1 hour',now()+interval '1 hour',now())`, s.User.ID)
	if err != nil {
		t.Fatal(err)
	}
	items, more, err := db.Sessions(ctx, s.Token)
	if err != nil || len(items) != 100 || !more || !items[0].Current {
		t.Fatal("unbounded/incorrect active session list", len(items), more, err)
	}
	count, err := db.RevokeOtherSessions(ctx, s.Token)
	if err != nil || count != 105 {
		t.Fatal("revoke-others missed hidden sessions or included inactive rows", count, err)
	}
	items, more, err = db.Sessions(ctx, s.Token)
	if err != nil || len(items) != 1 || more || !items[0].Current {
		t.Fatal("current session did not survive", err)
	}
	if count, err = db.RevokeOtherSessions(ctx, s.Token); err != nil || count != 0 {
		t.Fatal("repeat action must be idempotent", count, err)
	}
	if _, err = db.Authenticate(ctx, foreign.Token); err != nil {
		t.Fatal("revoke-others crossed user boundary", err)
	}
}

func TestConcurrentRefreshCannotEscapeRemoteRevocation(t *testing.T) {
	db, _, _, _ := testDB(t)
	ctx := context.Background()
	s, err := db.Register(ctx, "session-race@example.test", "synthetic-session-password")
	if err != nil {
		t.Fatal(err)
	}
	for range 4 {
		other, err := db.Login(ctx, s.User.Email, "synthetic-session-password")
		if err != nil {
			t.Fatal(err)
		}
		items, _, err := db.Sessions(ctx, s.Token)
		if err != nil || len(items) != 2 {
			t.Fatal("setup:", err)
		}
		target := items[1].ID
		start := make(chan struct{})
		revoked := make(chan error, 1)
		type refreshResult struct {
			s   postgres.Session
			err error
		}
		refreshed := make(chan refreshResult, 1)
		go func() {
			<-start
			found, err := db.RevokeSession(ctx, s.Token, target)
			if err == nil && !found {
				err = errors.New("target session not found")
			}
			revoked <- err
		}()
		go func() { <-start; next, err := db.Refresh(ctx, other.Token); refreshed <- refreshResult{next, err} }()
		close(start)
		if err = <-revoked; err != nil {
			t.Fatal("revocation race:", err)
		}
		next := <-refreshed
		if next.err != nil && !errors.Is(next.err, postgres.ErrUnauthorized) {
			t.Fatal("refresh race:", next.err)
		}
		for _, token := range []string{other.Token, next.s.Token} {
			if _, err = db.Authenticate(ctx, token); !errors.Is(err, postgres.ErrUnauthorized) {
				t.Fatal("refresh escaped revocation", err)
			}
		}
	}
}

func TestConcurrentMutualSessionRevocation(t *testing.T) {
	db, _, _, _ := testDB(t)
	ctx := context.Background()
	one, err := db.Register(ctx, "mutual-revoke@example.test", "synthetic-session-password")
	if err != nil {
		t.Fatal(err)
	}
	two, err := db.Login(ctx, one.User.Email, "synthetic-session-password")
	if err != nil {
		t.Fatal(err)
	}
	items, _, err := db.Sessions(ctx, one.Token)
	if err != nil || len(items) != 2 {
		t.Fatal(err)
	}
	start := make(chan struct{})
	results := make(chan error, 2)
	for _, operation := range []struct{ token, id string }{{one.Token, items[1].ID}, {two.Token, items[0].ID}} {
		go func(token, id string) {
			<-start
			found, err := db.RevokeSession(ctx, token, id)
			if err == nil && !found {
				err = errors.New("missing target")
			}
			results <- err
		}(operation.token, operation.id)
	}
	close(start)
	success := 0
	for range 2 {
		err := <-results
		if err == nil {
			success++
		} else if !errors.Is(err, postgres.ErrUnauthorized) {
			t.Fatal("mutual revocation deadlocked/failed:", err)
		}
	}
	if success != 1 {
		t.Fatal("expected one serialized revocation", success)
	}
}

func TestHTTPSessionManagementIsolation(t *testing.T) {
	db, _, _, _ := testDB(t)
	ctx := context.Background()
	one, err := db.Register(ctx, "http-sessions@example.test", "synthetic-session-password")
	if err != nil {
		t.Fatal(err)
	}
	two, err := db.Login(ctx, one.User.Email, "synthetic-session-password")
	if err != nil {
		t.Fatal(err)
	}
	foreign, err := db.Register(ctx, "http-foreign@example.test", "synthetic-foreign-password")
	if err != nil {
		t.Fatal(err)
	}
	handler := httpapi.NewPostgresRouter(slog.New(slog.NewTextHandler(io.Discard, nil)), db)
	call := func(method, path, token string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, nil)
		if token != "" {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	for _, spec := range [][2]string{{"GET", "/api/v1/auth/sessions"}, {"DELETE", "/api/v1/auth/sessions/00000000-0000-4000-8000-000000000001"}, {"POST", "/api/v1/auth/sessions/revoke-others"}} {
		if w := call(spec[0], spec[1], ""); w.Code != 401 {
			t.Fatal("unprotected session route", w.Code)
		}
	}
	w := call("GET", "/api/v1/auth/sessions", one.Token)
	if w.Code != 200 || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("list:", w.Code)
	}
	var page struct {
		Items   []postgres.SessionInfo `json:"items"`
		Count   int                    `json:"count"`
		HasMore bool                   `json:"hasMore"`
	}
	if err = json.Unmarshal(w.Body.Bytes(), &page); err != nil || page.Count != 2 || page.HasMore {
		t.Fatal("list contract:", err)
	}
	target := page.Items[1].ID
	if w = call("DELETE", "/api/v1/auth/sessions/not-a-uuid", one.Token); w.Code != 400 {
		t.Fatal("malformed session ID", w.Code)
	}
	if w = call("DELETE", "/api/v1/auth/sessions/"+target, foreign.Token); w.Code != 404 {
		t.Fatal("foreign session enumeration/revocation", w.Code)
	}
	if w = call("DELETE", "/api/v1/auth/sessions/00000000-0000-4000-8000-000000000001", foreign.Token); w.Code != 404 {
		t.Fatal("unknown ID behavior differs", w.Code)
	}
	if w = call("POST", "/api/v1/auth/sessions/revoke-others?user_id="+foreign.User.ID, one.Token); w.Code != 200 || !strings.Contains(w.Body.String(), `"revoked":1`) {
		t.Fatal("revoke-others:", w.Code, w.Body.String())
	}
	if w = call("GET", "/api/v1/auth/me", two.Token); w.Code != 401 {
		t.Fatal("revoked session retained HTTP access")
	}
	if w = call("GET", "/api/v1/auth/me", foreign.Token); w.Code != 200 {
		t.Fatal("foreign account changed")
	}
	if w = call("DELETE", "/api/v1/auth/sessions/"+page.Items[0].ID, one.Token); w.Code != 204 {
		t.Fatal("self revoke:", w.Code)
	}
	if w = call("GET", "/api/v1/tracker", one.Token); w.Code != 401 {
		t.Fatal("self-revoked session retained tracker access")
	}
}
