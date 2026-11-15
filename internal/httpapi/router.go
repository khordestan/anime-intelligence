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
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]string{"status": "ok", "storage": "local-file"})
	})
	mux.HandleFunc("GET /api/v1", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]any{"name": "Anime Intelligence API", "version": "v1", "status": "development", "authenticationRequired": false, "modules": []string{"catalog", "tracker"}})
	})
	c, t := catalog.NewHandler(store.Local{Store: data}), tracker.NewHandler(store.Local{Store: data})
	mux.HandleFunc("GET /api/v1/anime", c.List)
	mux.HandleFunc("GET /api/v1/anime/{id}", c.Get)
	mux.HandleFunc("GET /api/v1/tracker", t.List)
	mux.HandleFunc("POST /api/v1/tracker", t.Create)
	mux.HandleFunc("PATCH /api/v1/tracker/{id}", t.Update)
	mux.HandleFunc("PUT /api/v1/ratings/{id}", t.Rate)
	return requestMiddleware(logger, corsMiddleware(mux))
}
func requestMiddleware(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		logger.Info("http request", "method", r.Method, "path", r.URL.Path, "duration_ms", time.Since(start).Milliseconds())
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
