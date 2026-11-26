package catalog

import (
	"context"
	"net/http"
	"time"

	"github.com/khordestan/anime-intelligence/internal/platform/store"
)

func (h *Handler) CharacterCredits(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !validUUID(id) {
		writeError(w, http.StatusBadRequest, "invalid_id", "Anime ID must be a UUID")
		return
	}
	data, ok := h.data.(store.CharacterCreditCatalog)
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "catalog_unavailable", "Character credits are unavailable")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	credits, found, err := data.CharacterCredits(ctx, id)
	switch {
	case err != nil:
		writeError(w, http.StatusInternalServerError, "catalog_read_failed", "Could not read character credits")
	case !found:
		writeError(w, http.StatusNotFound, "anime_not_found", "Anime was not found")
	default:
		writeJSON(w, http.StatusOK, credits)
	}
}
