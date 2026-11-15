package httpapi

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/khordestan/anime-intelligence/internal/platform/store"
)

func TestResponseSecurityAcrossRouters(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	local, err := store.New("")
	if err != nil {
		t.Fatal(err)
	}
	// Nil database is intentional: these PostgreSQL paths must reject malformed
	// requests or unsupported routes before touching storage.
	routers := map[string]http.Handler{"local": NewRouter(logger, local), "postgres": NewPostgresRouter(logger, nil)}
	for name, router := range routers {
		t.Run(name, func(t *testing.T) {
			for _, tc := range []struct {
				method, path, body string
				status             int
			}{
				{"GET", "/healthz", "", 200},
				{"HEAD", "/api/v1", "", 200},
				{"GET", "/api/v1/anime/not-a-uuid", "", 400},
				{"GET", "/api/v1/missing", "", 404},
				{"DELETE", "/api/v1/anime", "", 405},
				{"OPTIONS", "/api/v1/auth/login", "", 204},
				{"GET", "/api/v1//anime", "", 307},
			} {
				t.Run(tc.method+tc.path, func(t *testing.T) {
					req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
					req.Header.Set("Origin", "http://localhost:3000")
					res := httptest.NewRecorder()
					router.ServeHTTP(res, req)
					if res.Code != tc.status {
						t.Fatalf("status: got %d want %d", res.Code, tc.status)
					}
					assertResponseSecurity(t, res.Result())
				})
			}
		})
	}
}

func TestCredentialErrorsRetainSecurityHeaders(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := NewPostgresRouter(logger, nil)
	for _, path := range []string{"/api/v1/auth/login", "/api/v1/auth/register"} {
		req := httptest.NewRequest("POST", path, strings.NewReader("{"))
		res := httptest.NewRecorder()
		handler.ServeHTTP(res, req)
		if res.Code != 400 {
			t.Fatalf("malformed credential status: %d", res.Code)
		}
		assertResponseSecurity(t, res.Result())
	}
}

func assertResponseSecurity(t *testing.T, res *http.Response) {
	t.Helper()
	defer res.Body.Close()
	for key, want := range map[string]string{"Cache-Control": "no-store", "X-Content-Type-Options": "nosniff", "Referrer-Policy": "no-referrer"} {
		if got := res.Header.Get(key); got != want {
			t.Errorf("%s: got %q want %q", key, got, want)
		}
	}
}
