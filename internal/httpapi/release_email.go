package httpapi

import (
	"net/http"

	"github.com/khordestan/anime-intelligence/internal/platform/postgres"
)

func registerReleaseEmailRoutes(mux *http.ServeMux, db *postgres.DB, protect func(func(http.ResponseWriter, *http.Request, string, postgres.User)) http.HandlerFunc) {
	mux.HandleFunc("GET /api/v1/releases/email-consent", protect(func(w http.ResponseWriter, r *http.Request, token string, _ postgres.User) {
		state, err := db.Repository(token).ReleaseEmailConsent(r.Context())
		if err != nil {
			releaseError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, state)
	}))
	mux.HandleFunc("PUT /api/v1/releases/email-consent", protect(func(w http.ResponseWriter, r *http.Request, token string, _ postgres.User) {
		var body struct {
			Enabled *bool `json:"enabled"`
		}
		if decodeAuth(w, r, &body) != nil || body.Enabled == nil {
			authError(w, http.StatusBadRequest, "invalid_consent")
			return
		}
		state, err := db.Repository(token).SetReleaseEmailConsent(r.Context(), *body.Enabled)
		if err != nil {
			releaseError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, state)
	}))
}
