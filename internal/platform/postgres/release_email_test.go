package postgres_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/khordestan/anime-intelligence/internal/httpapi"
	"github.com/khordestan/anime-intelligence/internal/platform/postgres"
)

func TestReleaseEmailConsentIsExplicitAndOwnerScoped(t *testing.T) {
	db, _, _, _ := testDB(t)
	ctx := context.Background()
	const password = "synthetic-consent-test-password"
	first, err := db.Register(ctx, "consent-one@example.test", password)
	if err != nil {
		t.Fatal(err)
	}
	second, err := db.Register(ctx, "consent-two@example.test", password)
	if err != nil {
		t.Fatal(err)
	}
	handler := httpapi.NewPostgresRouter(slog.New(slog.NewTextHandler(io.Discard, nil)), db)
	request := func(token, method, body string, want int) postgres.ReleaseEmailConsent {
		t.Helper()
		req := httptest.NewRequest(method, "/api/v1/releases/email-consent", strings.NewReader(body))
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		res := httptest.NewRecorder()
		handler.ServeHTTP(res, req)
		if res.Code != want {
			t.Fatalf("got %d, want %d: %s", res.Code, want, res.Body)
		}
		var consent postgres.ReleaseEmailConsent
		if want == 200 {
			if err := json.Unmarshal(res.Body.Bytes(), &consent); err != nil {
				t.Fatal(err)
			}
		}
		return consent
	}
	request("", http.MethodGet, "", 401)
	request(first.Token, http.MethodPut, `{"enabled":true}`, 403)
	verifyAccount(t, db, first.Token)
	verifyAccount(t, db, second.Token)
	if request(first.Token, http.MethodGet, "", 200).EnabledSince != nil {
		t.Fatal("new accounts must start opted out")
	}
	request(first.Token, http.MethodPut, `{}`, 400)
	enabled := request(first.Token, http.MethodPut, `{"enabled":true}`, 200).EnabledSince
	repeated := request(first.Token, http.MethodPut, `{"enabled":true}`, 200).EnabledSince
	if enabled == nil || repeated == nil || !enabled.Equal(*repeated) {
		t.Fatal("enable must be idempotent")
	}
	if request(second.Token, http.MethodGet, "", 200).EnabledSince != nil {
		t.Fatal("consent crossed accounts")
	}
	exported, err := db.ExportAccount(ctx, first.Token)
	if err != nil || exported.ReleaseEmailSince == nil || !exported.ReleaseEmailSince.Equal(*enabled) {
		t.Fatalf("missing exported consent: %v", err)
	}
	if request(first.Token, http.MethodPut, `{"enabled":false}`, 200).EnabledSince != nil {
		t.Fatal("disable must clear consent")
	}
}
