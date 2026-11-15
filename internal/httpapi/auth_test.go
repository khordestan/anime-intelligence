package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestBearerOnlyAcceptsOneAuthorizationHeader(t *testing.T) {
	for _, value := range []string{"", "Basic credential", "Bearer", "Bearer token extra"} {
		r := httptest.NewRequest("GET", "/?token=ignored", nil)
		r.Header.Set("Authorization", value)
		r.AddCookie(&http.Cookie{Name: "session", Value: "ignored"})
		if bearer(r) != "" {
			t.Fatal("invalid credential source accepted")
		}
	}
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("Authorization", "bEaReR opaque")
	if bearer(r) != "opaque" {
		t.Fatal("case-insensitive bearer scheme rejected")
	}
	r.Header.Add("Authorization", "Bearer second")
	if bearer(r) != "" {
		t.Fatal("ambiguous headers accepted")
	}
}
func TestAuthJSONBounds(t *testing.T) {
	for _, body := range []string{`{"email":"one"} {"email":"two"}`, `{"unknown":true}`, `{"email":"` + strings.Repeat("a", 4096) + `"}`} {
		var q struct {
			Email string `json:"email"`
		}
		r := httptest.NewRequest("POST", "/", strings.NewReader(body))
		if decodeAuth(httptest.NewRecorder(), r, &q) == nil {
			t.Fatal("invalid auth body accepted")
		}
	}
}
func TestAuthGateBoundsConcurrencyAndReplenishes(t *testing.T) {
	g := newAuthGate()
	if !g.acquire() || !g.acquire() || g.acquire() {
		t.Fatal("hashing concurrency limit failed")
	}
	g.release()
	g.release()
	for range 8 {
		if !g.acquire() {
			t.Fatal("burst ended too soon")
		}
		g.release()
	}
	if g.acquire() {
		t.Fatal("rate limit exceeded")
	}
	g.updated = time.Now().Add(-6 * time.Second)
	if !g.acquire() {
		t.Fatal("budget did not replenish")
	}
	g.release()
}
