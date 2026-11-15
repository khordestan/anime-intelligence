package httpapi

import (
	"errors"
	"net/http"

	"github.com/khordestan/anime-intelligence/internal/platform/identitymail"
	"github.com/khordestan/anime-intelligence/internal/platform/password"
	"github.com/khordestan/anime-intelligence/internal/platform/postgres"
)

func registerIdentityRoutes(mux *http.ServeMux, db *postgres.DB, delivery *identitymail.Service, hashGate *authGate, protect func(func(http.ResponseWriter, *http.Request, string, postgres.User)) http.HandlerFunc) {
	requestGate := newAuthGate()
	limited := func(gate *authGate, next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Cache-Control", "no-store")
			if !gate.acquire() {
				w.Header().Set("Retry-After", "6")
				authError(w, 429, "too_many_requests")
				return
			}
			defer gate.release()
			next(w, r)
		}
	}
	mux.HandleFunc("POST /api/v1/auth/password-reset/request", limited(requestGate, func(w http.ResponseWriter, r *http.Request) {
		if delivery == nil {
			authError(w, 503, "identity_delivery_unavailable")
			return
		}
		var q struct {
			Email string `json:"email"`
		}
		if decodeAuth(w, r, &q) != nil || len(q.Email) > 254 {
			authError(w, 400, "invalid_request")
			return
		}
		if !delivery.RequestPasswordReset(q.Email) {
			authError(w, 503, "identity_delivery_unavailable")
			return
		}
		writeJSON(w, 202, map[string]string{"status": "accepted"})
	}))
	mux.HandleFunc("POST /api/v1/auth/email-verification/request", protect(func(w http.ResponseWriter, r *http.Request, _ string, _ postgres.User) {
		limited(requestGate, func(w http.ResponseWriter, r *http.Request) {
			if delivery == nil {
				authError(w, 503, "identity_delivery_unavailable")
				return
			}
			if r.ContentLength != 0 {
				var q struct{}
				if decodeAuth(w, r, &q) != nil {
					authError(w, 400, "invalid_request")
					return
				}
			}
			if !delivery.RequestVerification(bearer(r)) {
				authError(w, 503, "identity_delivery_unavailable")
				return
			}
			writeJSON(w, 202, map[string]string{"status": "accepted"})
		})(w, r)
	}))
	mux.HandleFunc("POST /api/v1/auth/password-reset/confirm", limited(hashGate, func(w http.ResponseWriter, r *http.Request) {
		var q struct {
			Token       string `json:"token"`
			NewPassword string `json:"newPassword"`
		}
		if decodeAuth(w, r, &q) != nil {
			authError(w, 400, "invalid_json")
			return
		}
		if err := db.ResetPassword(r.Context(), q.Token, q.NewPassword); err != nil {
			challengeError(w, err)
			return
		}
		w.WriteHeader(204)
	}))
	mux.HandleFunc("POST /api/v1/auth/email-verification/confirm", limited(requestGate, func(w http.ResponseWriter, r *http.Request) {
		var q struct {
			Token string `json:"token"`
		}
		if decodeAuth(w, r, &q) != nil {
			authError(w, 400, "invalid_json")
			return
		}
		if err := db.VerifyEmail(r.Context(), q.Token); err != nil {
			challengeError(w, err)
			return
		}
		w.WriteHeader(204)
	}))
}
func challengeError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, postgres.ErrInvalidChallenge):
		authError(w, 400, "invalid_or_expired_token")
	case errors.Is(err, password.ErrInvalid):
		authError(w, 400, "invalid_password")
	default:
		authError(w, 500, "identity_operation_failed")
	}
}
