package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/khordestan/anime-intelligence/internal/modules/catalog"
	"github.com/khordestan/anime-intelligence/internal/modules/intelligence"
	"github.com/khordestan/anime-intelligence/internal/modules/tracker"
	"github.com/khordestan/anime-intelligence/internal/platform/identitymail"
	"github.com/khordestan/anime-intelligence/internal/platform/password"
	"github.com/khordestan/anime-intelligence/internal/platform/postgres"
	"github.com/khordestan/anime-intelligence/internal/platform/releasepush"
	"github.com/khordestan/anime-intelligence/internal/platform/store"
)

// NewPostgresRouter never registers the unauthenticated local-store routes.
// Browser cookie authentication is deliberately unsupported: only explicit
// Authorization bearer headers are accepted, avoiding ambient-cookie CSRF.
func NewPostgresRouter(logger *slog.Logger, db *postgres.DB) http.Handler {
	return NewPostgresRouterWithIdentity(logger, db, nil)
}
func NewPostgresRouterWithIdentity(logger *slog.Logger, db *postgres.DB, delivery *identitymail.Service) http.Handler {
	return NewPostgresRouterWithEssentials(logger, db, delivery, nil)
}

func NewPostgresRouterWithEssentials(logger *slog.Logger, db *postgres.DB, delivery *identitymail.Service, essentials []intelligence.EssentialsMatch) http.Handler {
	return NewPostgresRouterWithAnalyst(logger, db, delivery, essentials, nil)
}

func NewPostgresRouterWithAnalyst(logger *slog.Logger, db *postgres.DB, delivery *identitymail.Service, essentials []intelligence.EssentialsMatch, selector catalog.CatalogFieldSelector) http.Handler {
	return NewPostgresRouterWithPush(logger, db, delivery, essentials, selector, nil)
}

func NewPostgresRouterWithPush(logger *slog.Logger, db *postgres.DB, delivery *identitymail.Service, essentials []intelligence.EssentialsMatch, selector catalog.CatalogFieldSelector, push *releasepush.Sender) http.Handler {
	mux := http.NewServeMux()
	registerGlobalEssentials(mux, essentials)
	mux.HandleFunc("GET /readyz", readinessHandler("postgresql", db.Ready))
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]string{"status": "ok", "storage": "postgresql"})
	})
	mux.HandleFunc("GET /api/v1", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]any{"name": "Anime Intelligence API", "version": "v1", "status": "development", "authenticationRequired": true, "identityDeliveryAvailable": delivery != nil, "globalEssentialsAvailable": len(essentials) > 0, "assistedCatalogAnalysisAvailable": selector != nil, "emailVerificationRequired": true})
	})
	c := catalog.NewHandler(db.Repository(""))
	mux.HandleFunc("GET /api/v1/anime", c.List)
	mux.HandleFunc("POST /api/v1/analysis/catalog", c.Analyze)
	mux.HandleFunc("GET /api/v1/anime/lookup", c.Lookup)
	mux.HandleFunc("GET /api/v1/anime/facets", c.Facets)
	mux.HandleFunc("GET /api/v1/anime/{id}", c.Get)
	mux.HandleFunc("GET /api/v1/anime/{id}/details", c.Details)
	mux.HandleFunc("GET /api/v1/anime/{id}/character-credits", c.CharacterCredits)
	mux.HandleFunc("GET /api/v1/anime/{id}/episodes/{number}/schedule-history", c.ScheduleHistory)
	mux.HandleFunc("GET /api/v1/anime/{id}/continuity", c.Continuity)
	gate := newAuthGate()
	credential := func(register bool) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Cache-Control", "no-store")
			if !gate.acquire() {
				w.Header().Set("Retry-After", "6")
				authError(w, 429, "too_many_requests")
				return
			}
			defer gate.release()
			var q struct {
				Email    string `json:"email"`
				Password string `json:"password"`
			}
			if decodeAuth(w, r, &q) != nil {
				authError(w, 400, "invalid_json")
				return
			}
			var s postgres.Session
			var err error
			if register {
				s, err = db.Register(r.Context(), q.Email, q.Password)
			} else {
				s, err = db.Login(r.Context(), q.Email, q.Password)
			}
			if err != nil {
				switch {
				case errors.Is(err, postgres.ErrUnauthorized):
					authError(w, 401, "invalid_credentials")
				case errors.Is(err, postgres.ErrConflict):
					authError(w, 409, "account_unavailable")
				case errors.Is(err, postgres.ErrEmail), errors.Is(err, password.ErrInvalid):
					authError(w, 400, "invalid_credentials")
				default:
					authError(w, 500, "authentication_failed")
				}
				return
			}
			status := 200
			if register {
				status = 201
			}
			writeJSON(w, status, s)
		}
	}
	mux.HandleFunc("POST /api/v1/auth/register", credential(true))
	mux.HandleFunc("POST /api/v1/auth/login", credential(false))
	protect := func(fn func(http.ResponseWriter, *http.Request, string, postgres.User)) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Cache-Control", "no-store")
			token := bearer(r)
			u, err := db.Authenticate(r.Context(), token)
			if err != nil {
				if errors.Is(err, postgres.ErrUnauthorized) {
					w.Header().Set("WWW-Authenticate", "Bearer")
					authError(w, 401, "unauthorized")
				} else {
					authError(w, 503, "authentication_unavailable")
				}
				return
			}
			fn(w, r, token, u)
		}
	}
	if selector != nil {
		slots := make(chan struct{}, 2)
		mux.HandleFunc("POST /api/v1/analysis/catalog/assisted", protect(func(w http.ResponseWriter, r *http.Request, token string, u postgres.User) {
			if !u.EmailVerified {
				authError(w, http.StatusForbidden, "email_verification_required")
				return
			}
			c.AnalyzeWithSelector(w, r, boundedAnalysisSelector(slots, db.Repository(token).ReserveAnalysisCall, selector))
		}))
	}
	registerIdentityRoutes(mux, db, delivery, gate, protect)
	registerReleaseRoutes(mux, db, protect)
	registerReleaseEmailRoutes(mux, db, protect)
	registerEpisodeRoutes(mux, db, protect)
	registerRewatchRoutes(mux, db, protect)
	registerNotesRoutes(mux, db, protect)
	registerCalendarRoutes(mux, db, protect)
	registerRecommendationRoutes(mux, db, protect)
	mux.HandleFunc("GET /api/v1/auth/sessions", protect(func(w http.ResponseWriter, r *http.Request, token string, _ postgres.User) {
		items, more, err := db.Sessions(r.Context(), token)
		if err != nil {
			sessionError(w, err)
			return
		}
		writeJSON(w, 200, map[string]any{"items": items, "count": len(items), "hasMore": more})
	}))
	mux.HandleFunc("DELETE /api/v1/auth/sessions/{id}", protect(func(w http.ResponseWriter, r *http.Request, token string, _ postgres.User) {
		id := r.PathValue("id")
		if !validSessionID(id) {
			authError(w, 400, "invalid_session_id")
			return
		}
		found, err := db.RevokeSession(r.Context(), token, id)
		if err != nil {
			sessionError(w, err)
			return
		}
		if !found {
			authError(w, 404, "session_not_found")
			return
		}
		w.WriteHeader(204)
	}))
	mux.HandleFunc("POST /api/v1/auth/sessions/revoke-others", protect(func(w http.ResponseWriter, r *http.Request, token string, _ postgres.User) {
		count, err := db.RevokeOtherSessions(r.Context(), token)
		if err != nil {
			sessionError(w, err)
			return
		}
		writeJSON(w, 200, map[string]int64{"revoked": count})
	}))
	mux.HandleFunc("POST /api/v1/auth/password", protect(func(w http.ResponseWriter, r *http.Request, token string, _ postgres.User) {
		if !gate.acquire() {
			w.Header().Set("Retry-After", "6")
			authError(w, 429, "too_many_requests")
			return
		}
		defer gate.release()
		var q struct {
			CurrentPassword string `json:"currentPassword"`
			NewPassword     string `json:"newPassword"`
		}
		if decodeAuth(w, r, &q) != nil {
			authError(w, 400, "invalid_json")
			return
		}
		if err := db.ChangePassword(r.Context(), token, q.CurrentPassword, q.NewPassword); err != nil {
			if errors.Is(err, postgres.ErrReauthentication) {
				authError(w, 401, "invalid_current_password")
			} else if errors.Is(err, password.ErrInvalid) {
				authError(w, 400, "invalid_password")
			} else {
				sessionError(w, err)
			}
			return
		}
		w.WriteHeader(204)
	}))
	mux.HandleFunc("GET /api/v1/auth/export", protect(func(w http.ResponseWriter, r *http.Request, token string, _ postgres.User) {
		data, err := db.ExportAccount(r.Context(), token)
		if err != nil {
			sessionError(w, err)
			return
		}
		w.Header().Set("Content-Disposition", `attachment; filename="anime-intelligence-account.json"`)
		writeJSON(w, 200, data)
	}))
	mux.HandleFunc("DELETE /api/v1/auth/account", protect(func(w http.ResponseWriter, r *http.Request, token string, _ postgres.User) {
		if !gate.acquire() {
			w.Header().Set("Retry-After", "6")
			authError(w, 429, "too_many_requests")
			return
		}
		defer gate.release()
		var q struct {
			Password string `json:"password"`
		}
		if decodeAuth(w, r, &q) != nil {
			authError(w, 400, "invalid_json")
			return
		}
		if err := db.DeleteAccount(r.Context(), token, q.Password); err != nil {
			if errors.Is(err, postgres.ErrReauthentication) {
				authError(w, 401, "invalid_current_password")
			} else {
				sessionError(w, err)
			}
			return
		}
		w.WriteHeader(204)
	}))
	mux.HandleFunc("GET /api/v1/auth/me", protect(func(w http.ResponseWriter, r *http.Request, _ string, u postgres.User) { writeJSON(w, 200, u) }))
	mux.HandleFunc("POST /api/v1/auth/logout", protect(func(w http.ResponseWriter, r *http.Request, token string, _ postgres.User) {
		if err := db.Logout(r.Context(), token); err != nil {
			sessionError(w, err)
			return
		}
		w.WriteHeader(204)
	}))
	mux.HandleFunc("POST /api/v1/auth/refresh", protect(func(w http.ResponseWriter, r *http.Request, token string, _ postgres.User) {
		s, err := db.Refresh(r.Context(), token)
		if err != nil {
			sessionError(w, err)
			return
		}
		writeJSON(w, 200, s)
	}))
	product := func(fn func(http.ResponseWriter, *http.Request, string, postgres.User)) http.HandlerFunc {
		return protect(func(w http.ResponseWriter, r *http.Request, token string, user postgres.User) {
			if !user.EmailVerified {
				authError(w, 403, "email_verification_required")
				return
			}
			fn(w, r, token, user)
		})
	}
	registerReleasePush(mux, db, product, push)
	mux.HandleFunc("POST /api/v1/tracker/import/preview", product(func(w http.ResponseWriter, r *http.Request, token string, _ postgres.User) {
		importWatchList(w, r, db.Repository(token), true)
	}))
	mux.HandleFunc("POST /api/v1/tracker/import", product(func(w http.ResponseWriter, r *http.Request, token string, _ postgres.User) {
		importWatchList(w, r, db.Repository(token), false)
	}))
	mux.HandleFunc("GET /api/v1/tracker/export", product(func(w http.ResponseWriter, r *http.Request, token string, _ postgres.User) {
		export, err := db.Repository(token).ExportWatchList(r.Context())
		switch {
		case errors.Is(err, store.ErrUnauthorized):
			authError(w, 401, "unauthorized")
			return
		case errors.Is(err, store.ErrEmailUnverified):
			authError(w, 403, "email_verification_required")
			return
		case err != nil:
			authError(w, 500, "watch_list_export_failed")
			return
		}
		w.Header().Set("Content-Disposition", `attachment; filename="anime-intelligence-watch-list.json"`)
		writeJSON(w, 200, export)
	}))
	mux.HandleFunc("GET /api/v1/tracker", product(func(w http.ResponseWriter, r *http.Request, token string, _ postgres.User) {
		tracker.NewHandler(db.Repository(token)).List(w, r)
	}))
	mux.HandleFunc("POST /api/v1/tracker", product(func(w http.ResponseWriter, r *http.Request, token string, _ postgres.User) {
		tracker.NewHandler(db.Repository(token)).Create(w, r)
	}))
	mux.HandleFunc("PATCH /api/v1/tracker/bulk-progress", product(func(w http.ResponseWriter, r *http.Request, token string, _ postgres.User) {
		tracker.NewHandler(db.Repository(token)).BulkProgress(w, r)
	}))
	mux.HandleFunc("PATCH /api/v1/tracker/bulk-rating", product(func(w http.ResponseWriter, r *http.Request, token string, _ postgres.User) {
		tracker.NewHandler(db.Repository(token)).BulkRating(w, r)
	}))
	mux.HandleFunc("POST /api/v1/tracker/bulk-remove", product(func(w http.ResponseWriter, r *http.Request, token string, _ postgres.User) {
		tracker.NewHandler(db.Repository(token)).BulkRemove(w, r)
	}))
	mux.HandleFunc("PATCH /api/v1/tracker/bulk-status", product(func(w http.ResponseWriter, r *http.Request, token string, _ postgres.User) {
		tracker.NewHandler(db.Repository(token)).BulkStatus(w, r)
	}))
	mux.HandleFunc("PATCH /api/v1/tracker/{id}", product(func(w http.ResponseWriter, r *http.Request, token string, _ postgres.User) {
		tracker.NewHandler(db.Repository(token)).Update(w, r)
	}))
	mux.HandleFunc("DELETE /api/v1/tracker/{id}", product(func(w http.ResponseWriter, r *http.Request, token string, _ postgres.User) {
		tracker.NewHandler(db.Repository(token)).Remove(w, r)
	}))
	mux.HandleFunc("PUT /api/v1/tracker/{id}/season-rating", product(func(w http.ResponseWriter, r *http.Request, token string, _ postgres.User) {
		tracker.NewHandler(db.Repository(token)).SetSeasonRating(w, r)
	}))
	mux.HandleFunc("PUT /api/v1/ratings/{id}", product(func(w http.ResponseWriter, r *http.Request, token string, _ postgres.User) {
		tracker.NewHandler(db.Repository(token)).Rate(w, r)
	}))
	return requestMiddleware(logger, corsMiddleware(mux))
}
func bearer(r *http.Request) string {
	values := r.Header.Values("Authorization")
	if len(values) != 1 {
		return ""
	}
	parts := strings.Fields(values[0])
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}
	return parts[1]
}
func decodeAuth(w http.ResponseWriter, r *http.Request, v any) error {
	d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
	d.DisallowUnknownFields()
	if err := d.Decode(v); err != nil {
		return err
	}
	if err := d.Decode(new(any)); err != io.EOF {
		return errors.New("expected one JSON object")
	}
	return nil
}
func authError(w http.ResponseWriter, status int, code string) {
	writeJSON(w, status, map[string]string{"error": code})
}
func sessionError(w http.ResponseWriter, err error) {
	if errors.Is(err, postgres.ErrUnauthorized) {
		authError(w, 401, "unauthorized")
	} else {
		authError(w, 500, "session_update_failed")
	}
}

// A bounded, process-wide gate limits expensive password hashing without an
// unbounded per-IP map or reliance on spoofable forwarding headers. Production
// still needs ingress/per-account abuse controls and distributed limits.
type authGate struct {
	mu      sync.Mutex
	tokens  float64
	updated time.Time
	active  int
}

func newAuthGate() *authGate { return &authGate{tokens: 10, updated: time.Now()} }
func (g *authGate) acquire() bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	now := time.Now()
	g.tokens += now.Sub(g.updated).Seconds() / 6
	if g.tokens > 10 {
		g.tokens = 10
	}
	g.updated = now
	if g.tokens < 1 || g.active >= 2 {
		return false
	}
	g.tokens--
	g.active++
	return true
}
func (g *authGate) release() { g.mu.Lock(); g.active--; g.mu.Unlock() }

func validSessionID(id string) bool {
	if len(id) != 36 {
		return false
	}
	for i, c := range id {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			if c != '-' {
				return false
			}
			continue
		}
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}
