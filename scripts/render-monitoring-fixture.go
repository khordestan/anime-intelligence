//go:build ignore

// Render synthetic exporter output for promtool without database credentials.
package main

import (
	"fmt"
	"net/http/httptest"
	"os"
	"time"

	"github.com/khordestan/anime-intelligence/internal/httpapi"
	"github.com/khordestan/anime-intelligence/internal/platform/metrics"
	"github.com/khordestan/anime-intelligence/internal/platform/postgres"
)

func main() {
	now := time.Unix(2000, 0).UTC()
	created := now.Add(-time.Minute)
	snapshot := postgres.QueueDiagnostics{Version: 1, AsOf: now, RequestCapacity: 256, Requests: postgres.QueueCounts{Active: 3, Ready: 2, OldestActiveAt: &created, Outcomes: postgres.QueueOutcomes{Issued: 2, NoAction: 1}}, Deliveries: postgres.QueueCounts{Outcomes: postgres.QueueOutcomes{Accepted: 1, Exhausted: 1}}}
	if err := metrics.WriteIdentity(os.Stdout, snapshot); err != nil {
		fmt.Fprintln(os.Stderr, "synthetic monitoring fixture rendering failed")
		os.Exit(1)
	}
	requestMetrics := &httpapi.RequestMetrics{}
	scrape := httptest.NewRecorder()
	requestMetrics.Handler().ServeHTTP(scrape, httptest.NewRequest("GET", "/metrics", nil))
	if scrape.Code != 200 {
		fmt.Fprintln(os.Stderr, "synthetic HTTP metrics fixture rendering failed")
		os.Exit(1)
	}
	if _, err := os.Stdout.Write(scrape.Body.Bytes()); err != nil {
		os.Exit(1)
	}

}
