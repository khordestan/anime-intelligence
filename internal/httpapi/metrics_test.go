package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestRequestMetricsBoundedRoutesAndConcurrency(t *testing.T) {
	metrics := &RequestMetrics{}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/auth/login", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	})
	mux.HandleFunc("GET /api/v1/tracker/{id}", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok"))
	})
	handler := metrics.Wrap(corsMiddleware(mux))
	serve := func(method, path string) {
		request := httptest.NewRequest(method, path, nil)
		request.Header.Set("Authorization", "Bearer private-token")
		handler.ServeHTTP(httptest.NewRecorder(), request)
	}
	var requests sync.WaitGroup
	for range 10 {
		requests.Go(func() { serve("POST", "/api/v1/auth/login?secret=private-query") })
		requests.Go(func() { serve("GET", "/api/v1/tracker/private-path") })
		requests.Go(func() { metrics.RecordPush(PushAccepted) })
	}
	requests.Wait()
	metrics.RecordPush(PushFinishError)
	metrics.RecordPush(PushEvent(100))
	(*RequestMetrics)(nil).RecordPush(PushRetry)
	serve("GET", "/private-unrecognized")
	serve("OPTIONS", "/api/v1/auth/login")
	metrics.observe("GET /readyz", 503, 250*time.Millisecond)
	metrics.observe("GET /api/v1/calendar", 200, time.Second)
	response := httptest.NewRecorder()
	metrics.Handler().ServeHTTP(response, httptest.NewRequest("GET", "/metrics", nil))
	if response.Code != 200 || response.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("metrics status=%d headers=%v", response.Code, response.Header())
	}
	text := response.Body.String()
	for _, expected := range []string{
		`anime_release_push_events_total{outcome="provider_accepted"} 10`,
		`anime_release_push_events_total{outcome="provider_retry"} 0`,
		`anime_release_push_events_total{outcome="finish_error"} 1`,
		`anime_http_requests_total{group="identity",status_class="4xx"} 10`,
		`anime_http_rate_limited_total{group="identity"} 10`,
		`anime_http_requests_total{group="tracker",status_class="2xx"} 10`,
		`anime_http_requests_total{group="other",status_class="4xx"} 1`,
		`anime_http_requests_total{group="other",status_class="2xx"} 1`,
		`anime_http_requests_total{group="probe",status_class="5xx"} 1`,
		`anime_http_requests_total{group="releases",status_class="2xx"} 1`,
		`anime_http_request_duration_seconds_bucket{group="probe",le="0.1"} 0`,
		`anime_http_request_duration_seconds_bucket{group="probe",le="0.25"} 1`,
		`anime_http_request_duration_seconds_bucket{group="probe",le="+Inf"} 1`,
		`anime_http_request_duration_seconds_sum{group="probe"} 0.25`,
	} {
		if !strings.Contains(text, expected+"\n") {
			t.Errorf("missing %s", expected)
		}
	}
	if strings.Contains(text, "private-") || strings.Contains(text, "Authorization") {
		t.Fatal("request-specific labels escaped the aggregate")
	}
	for _, method := range []string{"GET", "POST"} {
		recorder := httptest.NewRecorder()
		metrics.Handler().ServeHTTP(recorder, httptest.NewRequest(method, "/metrics/"+method, nil))
		if recorder.Code != http.StatusNotFound {
			t.Fatalf("unexpected metrics subpath response %d", recorder.Code)
		}
	}
}

func TestPushWorkerConfigurationMetric(t *testing.T) {
	metrics := &RequestMetrics{}
	handler := metrics.Handler()
	scrape := func(expected string) {
		t.Helper()
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest("GET", "/metrics", nil))
		if !strings.Contains(response.Body.String(), "anime_release_push_worker_configured "+expected+"\n") {
			t.Fatalf("worker configuration gauge missing or wrong, want %s", expected)
		}
	}
	scrape("0")
	(*RequestMetrics)(nil).MarkPushWorkerConfigured()
	metrics.MarkPushWorkerConfigured()
	scrape("1")
}
