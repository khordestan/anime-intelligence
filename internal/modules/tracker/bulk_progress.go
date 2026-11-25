package tracker

import (
	"errors"
	"net/http"

	"github.com/khordestan/anime-intelligence/internal/platform/store"
)

func (h *Handler) BulkProgress(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Entries []store.BulkProgressTarget `json:"entries"`
	}
	invalid := func() {
		writeError(w, 400, "invalid_bulk_tracker_progress", "Provide up to 25 entry IDs with their viewing passes and current episode counts")
	}
	if decode(w, r, &body) != nil || len(body.Entries) == 0 || len(body.Entries) > store.MaxBulkStatusEntries {
		invalid()
		return
	}
	for _, entry := range body.Entries {
		if !validUUID(entry.ID) {
			invalid()
			return
		}
	}
	data, ok := h.data.(store.BulkProgressTracker)
	if !ok {
		writeError(w, 503, "tracker_unavailable", "Bulk progress is unavailable")
		return
	}
	items, found, err := data.AdvanceProgress(r.Context(), body.Entries)
	if errors.Is(err, store.ErrBulkProgressConflict) {
		writeError(w, 409, "progress_state_changed", "Progress changed or cannot advance for a selected entry")
		return
	}
	writeBulkEditResult(w, items, found, err, invalid)
}
