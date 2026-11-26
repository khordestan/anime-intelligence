package releasepush

import (
	"bytes"
	"context"
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	"io"
	"net/http"
	"net/netip"
	"strings"
	"testing"
	"time"
)

type testTransport func(*http.Request) (*http.Response, error)

func (t testTransport) Do(request *http.Request) (*http.Response, error) { return t(request) }

func TestBrowserPushEnvelopeAndProviderResponse(t *testing.T) {
	key, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	encode := base64.RawURLEncoding.EncodeToString
	sender, err := New(Config{
		PublicKey: encode(key.PublicKey().Bytes()), PrivateKey: encode(key.Bytes()),
		Contact: "https://example.test/contact", StorageKey: base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{8}, 32)),
	})
	if err != nil {
		t.Fatal(err)
	}
	subscription := Subscription{
		Endpoint: "https://fcm.googleapis.com/fcm/send/synthetic-endpoint",
		Keys:     Keys{Auth: encode(bytes.Repeat([]byte{9}, 16)), P256DH: encode(key.PublicKey().Bytes())},
	}
	for _, endpoint := range []string{
		"http://fcm.googleapis.com/fcm/send/test",
		"https://fcm.googleapis.com.example.test/fcm/send/test",
		"https://fcm.googleapis.com@internal.example.test/fcm/send/test",
		"https://fcm.googleapis.com:443/fcm/send/test",
		"https://fcm.googleapis.com/fcm/send/test?x=1",
		"https://fcm.googleapis.com/other/test",
		"https://fcm.googleapis.com/fcm/send/../internal",
	} {
		bad := subscription
		bad.Endpoint = endpoint
		if _, err := sender.Seal("owner-one", bad); err != ErrSubscription {
			t.Fatalf("invalid destination accepted: %s", endpoint)
		}
	}
	for _, ip := range []string{"127.0.0.1", "::1", "192.168.1.1", "100.64.1.2", "169.254.169.254", "fd00::1"} {
		if publicProviderIP(netip.MustParseAddr(ip)) {
			t.Fatalf("non-public address accepted: %s", ip)
		}
	}
	encrypted, err := sender.Seal("owner-one", subscription)
	if err != nil || bytes.Contains(encrypted, []byte(subscription.Endpoint)) {
		t.Fatalf("subscription storage: %v", err)
	}
	if _, err := sender.Open("owner-two", encrypted); err != ErrSubscription {
		t.Fatal("encrypted subscription opened under another owner")
	}
	if replay, err := sender.Open("owner-one", encrypted); err != nil || replay != subscription {
		t.Fatalf("cannot open stored subscription: %v", err)
	}

	for status, outcome := range map[int]Delivery{201: Delivered, 410: Gone, 302: Retry, 429: Retry} {
		sender.templates.HTTPClient = testTransport(func(r *http.Request) (*http.Response, error) {
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatal(err)
			}
			if r.Method != http.MethodPost || r.URL.String() != subscription.Endpoint ||
				!strings.HasPrefix(r.Header.Get("Authorization"), "vapid t=") || r.Header.Get("Topic") != "anime-release" ||
				r.Header.Get("Content-Encoding") != "aes128gcm" || bytes.Contains(body, []byte("release")) {
				t.Fatal("outbound push envelope differs from protocol")
			}
			return &http.Response{StatusCode: status, Header: http.Header{"Retry-After": {"300"}}, Body: io.NopCloser(strings.NewReader(""))}, nil
		})
		got, delay := sender.Send(context.Background(), subscription)
		if got != outcome || (status == 429 && delay != 5*time.Minute) || (status != 429 && delay != 0) {
			t.Fatalf("status %d produced %v %v", status, got, delay)
		}
	}
}

func TestRetryDelayBoundsBeforeConvertingToDuration(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for value, want := range map[string]time.Duration{
		"0":                        0,
		"-99999999999999999999999": 0,
		"9223372036854775807":      24 * time.Hour,
		"99999999999999999999999":  24 * time.Hour,
		now.Add(time.Minute).Format(http.TimeFormat): time.Minute,
	} {
		if got := retryDelay(value, now); got != want {
			t.Errorf("%q: got %v, want %v", value, got, want)
		}
	}
}
