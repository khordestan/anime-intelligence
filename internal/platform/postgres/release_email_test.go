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

func TestReleaseEmailLeaseRetryAndOptOut(t *testing.T) {
	db, admin, _, _ := testDB(t)
	ctx := context.Background()
	account, err := db.Register(ctx, "release-worker@example.test", "synthetic-release-worker-password")
	if err != nil {
		t.Fatal(err)
	}
	verifyAccount(t, db, account.Token)
	r := db.Repository(account.Token)
	const anime = "10000000-0000-4000-8000-000000000001"
	if ok, err := r.Follow(ctx, anime, "SUB"); err != nil || !ok {
		t.Fatalf("seed follow: %t %v", ok, err)
	}
	addEvent := func(key, track string) {
		t.Helper()
		if _, err := admin.Exec(ctx, `INSERT INTO domain_events(event_key,event_type,aggregate_type,aggregate_id,payload)
			VALUES($1,'EPISODE_RELEASED','ANIME',$2,jsonb_build_object('track',$3::text,'number',1))`, key, anime, track); err != nil {
			t.Fatal(err)
		}
	}
	addEvent("before-consent", "SUB")
	if _, err := r.SetReleaseEmailConsent(ctx, true); err != nil {
		t.Fatal(err)
	}
	addEvent("wrong-track", "DUB")
	addEvent("after-consent", "SUB")
	due := func() {
		t.Helper()
		if _, err := admin.Exec(ctx, `UPDATE release_email_jobs SET available_at=now() WHERE user_id=$1`, account.User.ID); err != nil {
			t.Fatal(err)
		}
	}
	claim, err := db.ClaimReleaseEmail(ctx)
	if err != nil || claim == nil || claim.Recipient != account.User.Email {
		t.Fatalf("initial claim failed: %v", err)
	}
	state, err := db.ReleaseEmailDiagnostics(ctx)
	if err != nil || state.Subscribers != 1 || state.Leased != 1 || state.Due != 0 || state.Attempted != 1 {
		t.Fatalf("leased diagnostics: %+v %v", state, err)
	}
	if duplicate, err := db.ClaimReleaseEmail(ctx); err != nil || duplicate != nil {
		t.Fatalf("lease allowed another claim: %v", err)
	}
	if err := db.FinishReleaseEmail(ctx, claim, false); err != nil {
		t.Fatal(err)
	}
	state, err = db.ReleaseEmailDiagnostics(ctx)
	if err != nil || state.Delayed != 1 || state.Attempted != 1 || state.Leased != 0 {
		t.Fatalf("retry diagnostics: %+v %v", state, err)
	}
	due()
	state, err = db.ReleaseEmailDiagnostics(ctx)
	if err != nil || state.Due != 1 || state.OldestDueAt == nil {
		t.Fatalf("due diagnostics: %+v %v", state, err)
	}
	retry, err := db.ClaimReleaseEmail(ctx)
	if err != nil || retry == nil {
		t.Fatalf("retry missing: %v", err)
	}
	if err := db.FinishReleaseEmail(ctx, claim, true); err != nil {
		t.Fatal(err)
	}
	if err := db.FinishReleaseEmail(ctx, retry, true); err != nil {
		t.Fatal(err)
	}
	var key string
	if err := admin.QueryRow(ctx, `SELECT e.event_key FROM notifications n JOIN domain_events e ON e.id=n.event_id
		WHERE n.user_id=$1 AND n.channel='EMAIL'`, account.User.ID).Scan(&key); err != nil || key != "after-consent" {
		t.Fatalf("wrong event recorded: %q %v", key, err)
	}
	due()
	if claim, err := db.ClaimReleaseEmail(ctx); err != nil || claim != nil {
		t.Fatalf("old event was replayed: %v", err)
	}
	addEvent("new-event", "SUB")
	due()
	claim, err = db.ClaimReleaseEmail(ctx)
	if err != nil || claim == nil {
		t.Fatalf("new event missing: %v", err)
	}
	if _, err := r.SetReleaseEmailConsent(ctx, false); err != nil {
		t.Fatal(err)
	}
	if err := db.FinishReleaseEmail(ctx, claim, true); err != nil {
		t.Fatal(err)
	}
	if _, err := r.SetReleaseEmailConsent(ctx, true); err != nil {
		t.Fatal(err)
	}
	if claim, err := db.ClaimReleaseEmail(ctx); err != nil || claim != nil {
		t.Fatalf("re-opt-in replayed history: %v", err)
	}
	var completed int
	if err := admin.QueryRow(ctx, `SELECT count(*) FROM notifications WHERE user_id=$1 AND channel='EMAIL'`, account.User.ID).Scan(&completed); err != nil || completed != 1 {
		t.Fatalf("unexpected delivery history: %d %v", completed, err)
	}
}
