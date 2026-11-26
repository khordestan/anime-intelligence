package tracker

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/khordestan/anime-intelligence/internal/platform/store"
)

func (h *Handler) SetSeasonRating(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !validUUID(id) {
		writeError(w, http.StatusBadRequest, "invalid_id", "Library entry ID must be a UUID")
		return
	}
	var request struct {
		ExpectedCycle int             `json:"expectedCycle"`
		Score         json.RawMessage `json:"score"`
	}
	invalid := func() {
		writeError(w, http.StatusBadRequest, "invalid_season_rating", "Provide a viewing pass and a score from 1 to 5 or null to clear")
	}
	if decode(w, r, &request) != nil || request.ExpectedCycle < 1 || request.ExpectedCycle > 1000 || len(request.Score) == 0 {
		invalid()
		return
	}
	var score *int
	if json.Unmarshal(request.Score, &score) != nil || (score != nil && (*score < 1 || *score > 5)) {
		invalid()
		return
	}
	data, ok := h.data.(store.SeasonRatingTracker)
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "tracker_unavailable", "Season ratings are unavailable")
		return
	}
	entry, found, err := data.SetSeasonRating(r.Context(), id, request.ExpectedCycle, score)
	switch {
	case errors.Is(err, store.ErrWatchCycleConflict):
		writeError(w, http.StatusConflict, "rewatch_state_changed", "Viewing pass changed; refresh your library")
	case errors.Is(err, store.ErrSeasonRatingUnavailable):
		writeError(w, http.StatusConflict, "season_rating_unavailable", "This title has no sourced season number")
	case err != nil:
		if !authorizationError(w, err) {
			writeError(w, http.StatusInternalServerError, "rating_write_failed", "Could not save season rating")
		}
	case !found:
		writeError(w, http.StatusNotFound, "tracker_entry_not_found", "Library entry was not found")
	default:
		writeJSON(w, http.StatusOK, entry)
	}
}
