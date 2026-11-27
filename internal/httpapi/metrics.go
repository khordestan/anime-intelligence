package httpapi

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/khordestan/anime-intelligence/internal/platform/smtptransport"
)

const (
	probeGroup = iota
	identityGroup
	catalogGroup
	trackerGroup
	releasesGroup
	recommendationsGroup
	analysisGroup
	otherGroup
)

var requestGroups = [...]string{"probe", "identity", "catalog", "tracker", "releases", "recommendations", "analysis", "other"}
var requestClasses = [...]string{"2xx", "3xx", "4xx", "5xx", "other"}
var latencyBounds = [...]time.Duration{
	5 * time.Millisecond, 25 * time.Millisecond, 100 * time.Millisecond,
	250 * time.Millisecond, 500 * time.Millisecond, time.Second,
	2500 * time.Millisecond, 5 * time.Second, 15 * time.Second, 30 * time.Second,
}

type requestTotals struct {
	count     uint64
	byClass   [len(requestClasses)]uint64
	limited   uint64
	buckets   [len(latencyBounds)]uint64
	totalTime float64
}

// RequestMetrics records completed API requests using fixed labels. It belongs
// to one process and is exposed by a separate monitoring listener.
type PushEvent int

const (
	PushAccepted PushEvent = iota
	PushRetry
	PushGone
	PushClaimError
	PushRecheckError
	PushSubscriptionError
	PushFinishError
)

var pushEventNames = [...]string{
	PushAccepted: "provider_accepted", PushRetry: "provider_retry", PushGone: "provider_gone",
	PushClaimError: "claim_error", PushRecheckError: "recheck_error",
	PushSubscriptionError: "subscription_error", PushFinishError: "finish_error",
}

type requestMetricsContextKey struct{}

const (
	analysisLocalLimit = iota
	analysisOwnerLimit
	analysisDeploymentLimit
	analysisBudgetError
	analysisProviderError
	analysisProviderReturned
)

var analysisOutcomeNames = [...]string{
	analysisLocalLimit: "local_limit", analysisOwnerLimit: "owner_budget_limit",
	analysisDeploymentLimit: "deployment_budget_limit", analysisBudgetError: "budget_error",
	analysisProviderError: "provider_error", analysisProviderReturned: "provider_returned",
}

type SMTPChannel uint8

const (
	SMTPIdentity SMTPChannel = iota
	SMTPRelease
)

var smtpChannelNames = [...]string{"identity", "release"}
var smtpOutcomeNames = [...]string{
	smtptransport.Accepted: "accepted", smtptransport.ConnectionError: "connection_error",
	smtptransport.GreetingError: "greeting_error", smtptransport.AuthenticationError: "authentication_error",
	smtptransport.EnvelopeError: "envelope_error", smtptransport.DataError: "data_error",
}

type RequestMetrics struct {
	smtpOutcomes     [len(smtpChannelNames)][len(smtpOutcomeNames)]uint64
	pushEvents       [len(pushEventNames)]uint64
	analysisOutcomes [len(analysisOutcomeNames)]uint64
	pushConfigured   int
	mu               sync.Mutex
	totals           [len(requestGroups)]requestTotals
}

func (m *RequestMetrics) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		response := &statusWriter{ResponseWriter: w}
		r = r.WithContext(context.WithValue(r.Context(), requestMetricsContextKey{}, m))
		next.ServeHTTP(response, r)
		status := response.status
		if status == 0 {
			status = http.StatusOK
		}
		m.observe(r.Pattern, status, time.Since(start))
	})
}

func (m *RequestMetrics) observe(pattern string, status int, elapsed time.Duration) {
	// Use only ServeMux's registered template, never URL or request fields.
	_, route, _ := strings.Cut(pattern, " ")
	group := otherGroup
	switch {
	case route == "/readyz" || route == "/healthz":
		group = probeGroup
	case strings.HasPrefix(route, "/api/v1/auth/"):
		group = identityGroup
	case route == "/api/v1/anime" || strings.HasPrefix(route, "/api/v1/anime/"):
		group = catalogGroup
	case routeGroup(route, "/api/v1/tracker") || routeGroup(route, "/api/v1/ratings"):
		group = trackerGroup
	case route == "/api/v1/calendar" || routeGroup(route, "/api/v1/follows") || routeGroup(route, "/api/v1/notifications") ||
		routeGroup(route, "/api/v1/releases"):
		group = releasesGroup
	case routeGroup(route, "/api/v1/recommendations") || routeGroup(route, "/api/v1/preferences"):
		group = recommendationsGroup
	case strings.HasPrefix(route, "/api/v1/analysis/"):
		group = analysisGroup
	}
	class := len(requestClasses) - 1
	if status >= 200 && status < 600 {
		class = status/100 - 2
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	totals := &m.totals[group]
	totals.count++
	totals.byClass[class]++
	if status == http.StatusTooManyRequests {
		totals.limited++
	}
	totals.totalTime += elapsed.Seconds()
	for i, bound := range latencyBounds {
		if elapsed <= bound {
			totals.buckets[i]++
		}
	}
}

func routeGroup(route, prefix string) bool {
	return route == prefix || strings.HasPrefix(route, prefix+"/")
}

// RecordPush counts an event from the browser delivery loop. The listener is optional.
func (m *RequestMetrics) RecordPush(event PushEvent) {
	if m == nil || event < 0 || int(event) >= len(pushEventNames) {
		return
	}
	m.mu.Lock()
	m.pushEvents[event]++
	m.mu.Unlock()
}

// MarkPushWorkerConfigured records that this process enabled browser delivery.
func (m *RequestMetrics) MarkPushWorkerConfigured() {
	if m == nil {
		return
	}
	m.mu.Lock()
	m.pushConfigured = 1
	m.mu.Unlock()
}

func (m *RequestMetrics) recordAnalysis(outcome int) {
	if m == nil {
		return
	}
	m.mu.Lock()
	m.analysisOutcomes[outcome]++
	m.mu.Unlock()
}

// SMTPObserver accepts only a fixed channel and one transport stage per send.
func (m *RequestMetrics) SMTPObserver(channel SMTPChannel) func(smtptransport.Outcome) {
	if m == nil || int(channel) >= len(smtpChannelNames) {
		return nil
	}
	return func(outcome smtptransport.Outcome) {
		if int(outcome) < len(smtpOutcomeNames) {
			m.mu.Lock()
			m.smtpOutcomes[channel][outcome]++
			m.mu.Unlock()
		}
	}
}

func (m *RequestMetrics) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /metrics", func(w http.ResponseWriter, _ *http.Request) {
		var output bytes.Buffer
		m.mu.Lock()
		totals, pushEvents, pushConfigured, analysisOutcomes := m.totals, m.pushEvents, m.pushConfigured, m.analysisOutcomes
		smtpOutcomes := m.smtpOutcomes
		m.mu.Unlock()

		fmt.Fprintln(&output, "# HELP anime_http_requests_total Completed API requests by route group and response class.")
		fmt.Fprintln(&output, "# TYPE anime_http_requests_total counter")
		for i, group := range requestGroups {
			for j, class := range requestClasses {
				fmt.Fprintf(&output, "anime_http_requests_total{group=%q,status_class=%q} %d\n", group, class, totals[i].byClass[j])
			}
		}
		fmt.Fprintln(&output, "# HELP anime_http_rate_limited_total Completed API requests with status 429.")
		fmt.Fprintln(&output, "# TYPE anime_http_rate_limited_total counter")
		for i, group := range requestGroups {
			fmt.Fprintf(&output, "anime_http_rate_limited_total{group=%q} %d\n", group, totals[i].limited)
		}
		fmt.Fprintln(&output, "# HELP anime_http_request_duration_seconds Duration of completed API requests by route group.")
		fmt.Fprintln(&output, "# TYPE anime_http_request_duration_seconds histogram")
		for i, group := range requestGroups {
			for j, bound := range latencyBounds {
				fmt.Fprintf(&output, "anime_http_request_duration_seconds_bucket{group=%q,le=%q} %d\n", group,
					fmt.Sprint(bound.Seconds()), totals[i].buckets[j])
			}
			fmt.Fprintf(&output, "anime_http_request_duration_seconds_bucket{group=%q,le=\"+Inf\"} %d\n", group, totals[i].count)
			fmt.Fprintf(&output, "anime_http_request_duration_seconds_sum{group=%q} %g\n", group, totals[i].totalTime)
			fmt.Fprintf(&output, "anime_http_request_duration_seconds_count{group=%q} %d\n", group, totals[i].count)
		}

		fmt.Fprintln(&output, "# HELP anime_release_push_worker_configured Whether this process enabled the browser delivery worker.")
		fmt.Fprintln(&output, "# TYPE anime_release_push_worker_configured gauge")
		fmt.Fprintf(&output, "anime_release_push_worker_configured %d\n", pushConfigured)

		fmt.Fprintln(&output, "# HELP anime_release_push_events_total Browser delivery loop events by fixed outcome.")
		fmt.Fprintln(&output, "# TYPE anime_release_push_events_total counter")
		for i, name := range pushEventNames {
			fmt.Fprintf(&output, "anime_release_push_events_total{outcome=%q} %d\n", name, pushEvents[i])
		}

		fmt.Fprintln(&output, "# HELP anime_assisted_analysis_outcomes_total Assisted catalog selector outcomes by fixed stage.")
		fmt.Fprintln(&output, "# TYPE anime_assisted_analysis_outcomes_total counter")
		for i, name := range analysisOutcomeNames {
			fmt.Fprintf(&output, "anime_assisted_analysis_outcomes_total{outcome=%q} %d\n", name, analysisOutcomes[i])
		}

		fmt.Fprintln(&output, "# HELP anime_smtp_transport_outcomes_total SMTP send attempts by channel and fixed transport outcome.")
		fmt.Fprintln(&output, "# TYPE anime_smtp_transport_outcomes_total counter")
		for i, channel := range smtpChannelNames {
			for j, outcome := range smtpOutcomeNames {
				fmt.Fprintf(&output, "anime_smtp_transport_outcomes_total{channel=%q,outcome=%q} %d\n", channel, outcome, smtpOutcomes[i][j])
			}
		}

		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		_, _ = w.Write(output.Bytes())
	})
	return mux
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

func (w *statusWriter) WriteHeader(status int) {
	if w.status == 0 && status >= 200 {
		w.status = status
	}
	w.ResponseWriter.WriteHeader(status)
}

func (w *statusWriter) Write(payload []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return w.ResponseWriter.Write(payload)
}
