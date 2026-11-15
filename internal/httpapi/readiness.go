package httpapi

import (
	"context"
	"net/http"
	"time"
)

// Keep liveness independent of database availability. This readiness boundary
// only reports generic status and never credentials, query details or errors.
func readinessHandler(storage string, check func(context.Context) error) http.HandlerFunc {
	gate := make(chan struct{}, 1)
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if check != nil {
			select {
			case gate <- struct{}{}:
				defer func() { <-gate }()
			default:
				writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "unavailable", "storage": storage})
				return
			}
			if err := check(ctx); err != nil {
				writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "unavailable", "storage": storage})
				return
			}
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "storage": storage})
	}
}
