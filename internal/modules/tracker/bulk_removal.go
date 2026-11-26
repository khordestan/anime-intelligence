package tracker

import (
	"errors"
	"net/http"

	"github.com/khordestan/anime-intelligence/internal/platform/store"
)

func (h *Handler) BulkRemove(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Entries []store.BulkRemovalTarget `json:"entries"`
	}
	invalid := func() {
		writeError(w, http.StatusBadRequest, "invalid_bulk_tracker_removal", "Select up to 25 distinct entries with their viewing passes and modification times")
	}
	if decode(w, r, &request) != nil || len(request.Entries) < 1 || len(request.Entries) > store.MaxBulkStatusEntries {
		invalid()
		return
	}
	for _, target := range request.Entries {
		if !validUUID(target.ID) {
			invalid()
			return
		}
	}
	data, ok := h.data.(store.BulkRemovalTracker)
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "tracker_unavailable", "Bulk removal is unavailable")
		return
	}
	found, err := data.RemoveEntries(r.Context(), request.Entries)
	switch {
	case errors.Is(err, store.ErrBulkStatusInvalid):
		invalid()
	case errors.Is(err, store.ErrBulkRemovalChanged):
		writeError(w, http.StatusConflict, "bulk_tracker_removal_changed", "A selected entry changed; refresh your library")
	case err != nil:
		if !authorizationError(w, err) {
			writeError(w, http.StatusInternalServerError, "tracker_remove_failed", "Could not remove selected entries")
		}
	case !found:
		writeError(w, http.StatusNotFound, "tracker_entry_not_found", "A library entry was not found")
	default:
		w.WriteHeader(http.StatusNoContent)
	}
}
