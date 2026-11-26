package httpapi

import (
	"errors"
	"net/http"

	"github.com/khordestan/anime-intelligence/internal/platform/postgres"
	"github.com/khordestan/anime-intelligence/internal/platform/releasepush"
)

func registerReleasePush(mux *http.ServeMux, db *postgres.DB, product func(func(http.ResponseWriter, *http.Request, string, postgres.User)) http.HandlerFunc, sender *releasepush.Sender) {
	mux.HandleFunc("GET /api/v1/releases/browser-push", product(func(w http.ResponseWriter, r *http.Request, token string, _ postgres.User) {
		registration, err := db.Repository(token).PushRegistration(r.Context())
		if err != nil {
			releaseError(w, err)
			return
		}
		var key string
		if sender != nil {
			key = sender.PublicKey()
		}
		writeJSON(w, http.StatusOK, struct {
			PublicKey    string                     `json:"publicKey,omitempty"`
			Registration *postgres.PushRegistration `json:"registration"`
		}{PublicKey: key, Registration: registration})
	}))
	mux.HandleFunc("PUT /api/v1/releases/browser-push", product(func(w http.ResponseWriter, r *http.Request, token string, _ postgres.User) {
		if sender == nil {
			authError(w, http.StatusServiceUnavailable, "push_unavailable")
			return
		}
		var subscription releasepush.Subscription
		if decodeAuth(w, r, &subscription) != nil {
			authError(w, http.StatusBadRequest, "invalid_push_subscription")
			return
		}
		registration, err := db.Repository(token).RegisterPush(r.Context(), sender, subscription)
		switch {
		case errors.Is(err, releasepush.ErrSubscription):
			authError(w, http.StatusBadRequest, "invalid_push_subscription")
		case errors.Is(err, postgres.ErrPushRegistrationConflict):
			authError(w, http.StatusConflict, "push_subscription_unavailable")
		case err != nil:
			releaseError(w, err)
		default:
			writeJSON(w, http.StatusOK, registration)
		}
	}))
	mux.HandleFunc("DELETE /api/v1/releases/browser-push/{id}", product(func(w http.ResponseWriter, r *http.Request, token string, _ postgres.User) {
		id := r.PathValue("id")
		if !validSessionID(id) {
			authError(w, http.StatusBadRequest, "invalid_push_registration")
			return
		}
		if err := db.Repository(token).RemovePush(r.Context(), id); err != nil {
			releaseError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
}
