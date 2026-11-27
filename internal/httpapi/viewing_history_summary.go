package httpapi

import (
	"net/http"

	"github.com/khordestan/anime-intelligence/internal/platform/postgres"
)

func viewingHistorySummary(db *postgres.DB) func(http.ResponseWriter, *http.Request, string, postgres.User) {
	return func(w http.ResponseWriter, r *http.Request, token string, user postgres.User) {
		if !user.EmailVerified {
			authError(w, http.StatusForbidden, "email_verification_required")
			return
		}
		months, err := db.Repository(token).ViewingHistorySummary(r.Context())
		if err != nil {
			rewatchError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"months": months})
	}
}
