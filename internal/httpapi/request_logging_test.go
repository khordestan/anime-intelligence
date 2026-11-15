package httpapi

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRequestLogsOnlyTrustedRouteTemplatesAndBoundedMethods(t *testing.T) {
	for _, test := range []struct{ method, path, route, wantMethod string }{
		{"GET", "/items/synthetic-secret-path?token=synthetic-secret-query", "GET /items/{id}", "GET"},
		{"POST", "/synthetic-secret-unknown", "unmatched", "POST"},
		{"SYNTHETIC-SECRET-METHOD", "/items/synthetic-secret-path", "unmatched", "OTHER"},
		{"GET", "/items/synthetic-secret-redirect/../other", "GET /items/{id}", "GET"},
	} {
		t.Run(test.method+test.path, func(t *testing.T) {
			var output bytes.Buffer
			mux := http.NewServeMux()
			mux.HandleFunc("GET /items/{id}", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })
			logger := slog.New(slog.NewJSONHandler(&output, nil))
			handler := requestMiddleware(logger, mux)
			request := httptest.NewRequest(test.method, test.path, strings.NewReader("synthetic-secret-body"))
			request.Header.Set("Authorization", "Bearer synthetic-secret-bearer")
			request.Header.Set("Cookie", "synthetic-secret-cookie")
			handler.ServeHTTP(httptest.NewRecorder(), request)
			if strings.Contains(strings.ToLower(output.String()), "synthetic-secret") {
				t.Fatal("request logging disclosed supplied values")
			}
			var entry map[string]any
			if err := json.Unmarshal(output.Bytes(), &entry); err != nil {
				t.Fatal(err)
			}
			if entry["method"] != test.wantMethod || entry["route"] != test.route || entry["path"] != nil || entry["duration_ms"] == nil {
				t.Fatal("unexpected request log fields", entry)
			}
		})
	}
}
