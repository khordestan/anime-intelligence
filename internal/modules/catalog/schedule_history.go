package catalog

import (
	"net/http"
	"strconv"

	"github.com/khordestan/anime-intelligence/internal/platform/store"
)

func (h *Handler) ScheduleHistory(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !validUUID(id) {
		writeError(w, 400, "invalid_id", "Anime ID must be a UUID")
		return
	}
	number, err := strconv.Atoi(r.PathValue("number"))
	if err != nil || number < 1 || number > 100000 {
		writeError(w, 400, "invalid_episode_number", "Episode number must be between 1 and 100000")
		return
	}
	query := r.URL.Query()
	track := query.Get("track")
	if len(query["track"]) != 1 || (track != "RAW" && track != "SUB" && track != "DUB" && track != "UNKNOWN") {
		writeError(w, 400, "invalid_release_track", "Select one release track")
		return
	}
	var before int64
	if values, ok := query["beforeRevision"]; ok {
		if len(values) == 1 {
			before, err = strconv.ParseInt(values[0], 10, 32)
		}
		if err != nil || before < 1 {
			writeError(w, 400, "invalid_schedule_cursor", "Select a positive beforeRevision")
			return
		}
	}
	data, ok := h.data.(store.ScheduleHistoryCatalog)
	if !ok {
		writeError(w, 503, "catalog_unavailable", "Schedule history is unavailable")
		return
	}
	page, err := data.CatalogScheduleHistory(r.Context(), id, number, track, int(before))
	if err != nil {
		writeError(w, 500, "catalog_read_failed", "Could not read schedule history")
		return
	}
	writeJSON(w, 200, page)
}
