package httpapi

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/khordestan/anime-intelligence/internal/platform/postgres"
)

func relinkPlanning(db *postgres.DB) func(http.ResponseWriter, *http.Request, string, postgres.User) {
	return func(w http.ResponseWriter, r *http.Request, token string, user postgres.User) {
		if !user.EmailVerified {
			authError(w, http.StatusForbidden, "email_verification_required")
			return
		}
		var body struct {
			TargetAnimeID     string    `json:"targetAnimeId"`
			ExpectedUpdatedAt time.Time `json:"expectedUpdatedAt"`
		}
		entryID := r.PathValue("id")
		if !validSessionID(entryID) || decodeAuth(w, r, &body) != nil || !validSessionID(body.TargetAnimeID) || body.ExpectedUpdatedAt.IsZero() {
			authError(w, http.StatusBadRequest, "invalid_planning_relink")
			return
		}
		entry, found, err := db.Repository(token).RelinkPlanning(r.Context(), entryID, strings.ToLower(body.TargetAnimeID), body.ExpectedUpdatedAt)
		switch {
		case errors.Is(err, postgres.ErrPlanningRelinkConflict):
			authError(w, http.StatusConflict, "planning_entry_changed")
		case errors.Is(err, postgres.ErrPlanningRelinkTarget):
			authError(w, http.StatusNotFound, "anime_not_found")
		case err != nil:
			rewatchError(w, err)
		case !found:
			authError(w, http.StatusNotFound, "tracker_not_found")
		default:
			writeJSON(w, http.StatusOK, entry)
		}
	}
}
