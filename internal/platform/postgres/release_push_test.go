package postgres_test

import (
	"bytes"
	"context"
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"io"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/khordestan/anime-intelligence/internal/httpapi"
	"github.com/khordestan/anime-intelligence/internal/platform/postgres"
	"github.com/khordestan/anime-intelligence/internal/platform/releasepush"
)

func TestBrowserPushConsentLeasesAndOwnerIsolation(t *testing.T) {
	db, admin, _, runtimeDSN := testDB(t)
	ctx := context.Background()
	first, err := db.Register(ctx, "browser-push@example.test", "synthetic-browser-push-password")
	if err != nil {
		t.Fatal(err)
	}
	other, err := db.Register(ctx, "other-browser-push@example.test", "synthetic-browser-push-password")
	if err != nil {
		t.Fatal(err)
	}
	verifyAccount(t, db, first.Token)
	repo := db.Repository(first.Token)
	const anime = "10000000-0000-4000-8000-000000000001"
	if found, err := repo.Follow(ctx, anime, "SUB"); err != nil || !found {
		t.Fatal("fixture follow", err)
	}
	key, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	encode := base64.RawURLEncoding.EncodeToString
	sender, err := releasepush.New(releasepush.Config{
		PublicKey: encode(key.PublicKey().Bytes()), PrivateKey: encode(key.Bytes()),
		Contact: "https://example.test/contact", StorageKey: base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{22}, 32)),
	})
	if err != nil {
		t.Fatal(err)
	}
	starter := releasepush.Subscription{
		Endpoint: "https://fcm.googleapis.com/fcm/send/synthetic-registration",
		Keys:     releasepush.Keys{Auth: encode(bytes.Repeat([]byte{9}, 16)), P256DH: encode(key.PublicKey().Bytes())},
	}
	router := httpapi.NewPostgresRouterWithPush(slog.New(slog.NewTextHandler(io.Discard, nil)), db, nil, nil, nil, sender)
	request := func(method, suffix, token string, body any, want int) string {
		t.Helper()
		data, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		req := httptest.NewRequest(method, "/api/v1/releases/browser-push"+suffix, bytes.NewReader(data))
		req.Header.Set("Authorization", "Bearer "+token)
		out := httptest.NewRecorder()
		router.ServeHTTP(out, req)
		if out.Code != want {
			t.Fatalf("%s status=%d want=%d: %s", method, out.Code, want, out.Body)
		}
		if strings.Contains(out.Body.String(), starter.Endpoint) || strings.Contains(out.Body.String(), starter.Keys.Auth) {
			t.Fatal("private subscription details in response")
		}
		return out.Body.String()
	}
	event := func(name, track string) {
		t.Helper()
		if _, err := admin.Exec(ctx, `INSERT INTO domain_events(event_key,event_type,aggregate_type,aggregate_id,payload)
			VALUES($1,'EPISODE_RELEASED','ANIME',$2,jsonb_build_object('track',$3::text,'number',1))`, name, anime, track); err != nil {
			t.Fatal(err)
		}
	}
	event("before-push-consent", "SUB")
	request("PUT", "", other.Token, starter, 403)
	verifyAccount(t, db, other.Token)
	create := func(subscription releasepush.Subscription) postgres.PushRegistration {
		t.Helper()
		body := request("PUT", "", first.Token, subscription, 200)
		var registration postgres.PushRegistration
		if err := json.Unmarshal([]byte(body), &registration); err != nil || registration.ID == "" {
			t.Fatalf("invalid registration: %s", body)
		}
		return registration
	}
	original := create(starter)
	if repeated := create(starter); repeated != original {
		t.Fatal("repeat registration changed consent time or ID")
	}
	request("PUT", "", other.Token, starter, 409)
	request("DELETE", "/"+original.ID, other.Token, nil, 204)
	export, err := db.ExportAccount(ctx, first.Token)
	if err != nil || export.ReleasePushSince == nil || !export.ReleasePushSince.Equal(original.CreatedAt) {
		t.Fatalf("account export missed consent: %v", err)
	}
	var encrypted []byte
	if err := admin.QueryRow(ctx, "SELECT payload FROM release_push_subscriptions WHERE user_id=$1", first.User.ID).Scan(&encrypted); err != nil ||
		bytes.Contains(encrypted, []byte(starter.Endpoint)) || bytes.Contains(encrypted, []byte(starter.Keys.Auth)) {
		t.Fatalf("invalid stored subscription envelope: %v", err)
	}

	event("wrong-push-track", "DUB")
	event("after-push-consent", "SUB")
	claim, err := db.ClaimPush(ctx)
	if err != nil || claim == nil {
		t.Fatalf("missing first delivery: %v", err)
	}
	if actual, err := claim.Subscription(sender); err != nil || actual != starter {
		t.Fatalf("wrong delivery subscription: %v", err)
	}
	replica, err := postgres.Open(ctx, runtimeDSN)
	if err != nil {
		t.Fatal(err)
	}
	defer replica.Close()
	if duplicate, err := replica.ClaimPush(ctx); err != nil || duplicate != nil {
		t.Fatalf("same delivery claimed on two replicas: %v", err)
	}
	if err = replica.FinishPush(ctx, claim, releasepush.Retry, 5*time.Minute); err != nil {
		t.Fatal(err)
	}
	var future bool
	if err = admin.QueryRow(ctx, "SELECT available_at>now()+interval '4 minutes' FROM release_push_subscriptions WHERE user_id=$1", first.User.ID).Scan(&future); err != nil || !future {
		t.Fatalf("retry ignored provider delay: %v", err)
	}
	due := func() {
		t.Helper()
		if _, err := admin.Exec(ctx, "UPDATE release_push_subscriptions SET available_at=now() WHERE user_id=$1", first.User.ID); err != nil {
			t.Fatal(err)
		}
	}
	due()
	retry, err := replica.ClaimPush(ctx)
	if err != nil || retry == nil {
		t.Fatalf("missing retry: %v", err)
	}
	if err = db.FinishPush(ctx, claim, releasepush.Gone, 0); err != nil {
		t.Fatal(err)
	}
	if current, err := db.PushClaimCurrent(ctx, retry); err != nil || !current {
		t.Fatalf("stale completion affected new lease: %v", err)
	}
	if err = replica.FinishPush(ctx, retry, releasepush.Delivered, 0); err != nil {
		t.Fatal(err)
	}
	var deliveredEvent string
	if err = admin.QueryRow(ctx, `SELECT e.event_key FROM notifications n JOIN domain_events e ON e.id=n.event_id
		WHERE n.channel='PUSH' AND n.user_id=$1`, first.User.ID).Scan(&deliveredEvent); err != nil || deliveredEvent != "after-push-consent" {
		t.Fatalf("wrong delivery event %s: %v", deliveredEvent, err)
	}
	due()
	if duplicate, err := db.ClaimPush(ctx); err != nil || duplicate != nil {
		t.Fatalf("delivered event was replayed: %v", err)
	}
	event("revoked-push-event", "SUB")
	due()
	pending, err := db.ClaimPush(ctx)
	if err != nil || pending == nil {
		t.Fatalf("missing pending event: %v", err)
	}
	replaced := starter
	replaced.Endpoint += "-replacement"
	newRegistration := create(replaced)
	request("DELETE", "/"+original.ID, first.Token, nil, 204)
	if current, err := db.PushClaimCurrent(ctx, pending); err != nil || current {
		t.Fatalf("replaced device remained eligible: %v", err)
	}
	if err = db.FinishPush(ctx, pending, releasepush.Gone, 0); err != nil {
		t.Fatal(err)
	}
	event("replacement-push-event", "SUB")
	newClaim, err := db.ClaimPush(ctx)
	if err != nil || newClaim == nil {
		t.Fatalf("replacement device blocked by stale removal: %v", err)
	}
	if err := db.FinishPush(ctx, newClaim, releasepush.Gone, 0); err != nil {
		t.Fatal(err)
	}
	if newRegistration.ID == original.ID || !strings.Contains(request("GET", "", first.Token, nil, 200), `"registration":null`) {
		t.Fatal("gone provider endpoint remained registered")
	}
}
