package catalog

import (
	"encoding/json"
	"github.com/khordestan/anime-intelligence/internal/platform/store"
	"net/http"
	"strconv"
	"strings"
)

type Handler struct{ data store.Repository }

func NewHandler(data store.Repository) *Handler { return &Handler{data: data} }
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	limit := queryInt(r, "limit", 24, 1, 100)
	offset := queryInt(r, "offset", 0, 0, 100000)
	items, err := h.data.Catalog(r.Context(), strings.TrimSpace(r.URL.Query().Get("q")), limit, offset)
	if err != nil {
		writeError(w, 500, "catalog_read_failed", "Could not read catalog")
		return
	}
	writeJSON(w, 200, map[string]any{"items": items, "limit": limit, "offset": offset, "count": len(items)})
}
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !validUUID(id) {
		writeError(w, 400, "invalid_id", "Anime ID must be a UUID")
		return
	}
	a, ok, err := h.data.GetAnime(r.Context(), id)
	if err != nil {
		writeError(w, 500, "catalog_read_failed", "Could not read catalog")
		return
	}
	if !ok {
		writeError(w, 404, "anime_not_found", "Anime was not found")
		return
	}
	writeJSON(w, 200, a)
}
func queryInt(r *http.Request, key string, fallback, min, max int) int {
	v, e := strconv.Atoi(r.URL.Query().Get(key))
	if e != nil {
		return fallback
	}
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}
func validUUID(s string) bool {
	if len(s) != 36 {
		return false
	}
	for i, c := range s {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			if c != '-' {
				return false
			}
			continue
		}
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]string{"error": code, "message": message})
}
