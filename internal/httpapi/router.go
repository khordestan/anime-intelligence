package httpapi

import (
	"encoding/json"
	"github.com/khordestan/anime-intelligence/internal/modules/catalog"
	"github.com/khordestan/anime-intelligence/internal/modules/tracker"
	"github.com/khordestan/anime-intelligence/internal/platform/store"
	"log/slog"
	"net/http"
	"time"
)

func NewRouter(logger *slog.Logger, data *store.Store) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /readyz", readinessHandler("local-file", nil))
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]string{"status": "ok", "storage": "local-file"})
	})
	mux.HandleFunc("GET /api/v1", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]any{"name": "Anime Intelligence API", "version": "v1", "status": "development", "authenticationRequired": false, "modules": []string{"catalog", "tracker"}})
	})
	c, t := catalog.NewHandler(store.Local{Store: data}), tracker.NewHandler(store.Local{Store: data})
	mux.HandleFunc("GET /api/v1/anime", c.List)
	mux.HandleFunc("GET /api/v1/anime/lookup", c.Lookup)
 mux.HandleFunc("GET /api/v1/anime/facets", c.Facets)
	mux.HandleFunc("GET /api/v1/anime/{id}", c.Get)
	mux.HandleFunc("GET /api/v1/anime/{id}/details", c.Details)
	mux.HandleFunc("GET /api/v1/anime/{id}/continuity", c.Continuity)
	mux.HandleFunc("GET /api/v1/tracker", t.List)
	mux.HandleFunc("POST /api/v1/tracker", t.Create)
	mux.HandleFunc("PATCH /api/v1/tracker/{id}", t.Update)
	mux.HandleFunc("DELETE /api/v1/tracker/{id}", t.Remove)
	mux.HandleFunc("PUT /api/v1/ratings/{id}", t.Rate)
	return requestMiddleware(logger, corsMiddleware(mux))
}
func requestMiddleware(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Both storage modes share this boundary, including errors, redirects
		// and preflights. Account exports and bearer/proof responses must never
		// become reusable browser or intermediary cache entries.
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		start := time.Now()
		next.ServeHTTP(w, r)
		method := r.Method
		switch method {
		case http.MethodGet, http.MethodHead, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodOptions, http.MethodConnect, http.MethodTrace:
		default:
			method = "OTHER"
		}
		route := r.Pattern
		if route == "" {
			route = "unmatched"
		}
		logger.Info("http request", "method", method, "route", route, "duration_ms", time.Since(start).Milliseconds())
	})
}
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin == "http://localhost:3000" || origin == "http://127.0.0.1:3000" {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
		}
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
