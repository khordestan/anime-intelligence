package tracker

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/khordestan/anime-intelligence/internal/platform/store"
	"io"
	"net/http"
	"strings"
)

type Handler struct{ data store.Repository }

func NewHandler(data store.Repository) *Handler { return &Handler{data: data} }

type createRequest struct {
	AnimeID         string `json:"animeId"`
	WatchStatus     string `json:"watchStatus"`
	EpisodesWatched int    `json:"episodesWatched"`
}
type updateRequest struct {
	ExpectedCycle *int `json:"expectedCycle"`
	WatchStatus     *string `json:"watchStatus"`
	EpisodesWatched *int    `json:"episodesWatched"`
}
type ratingRequest struct {
	Score int `json:"score"`
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	items, err := h.data.Library(r.Context())
	if err != nil {
		if authorizationError(w, err) {
			return
		}
		writeError(w, 500, "tracker_read_failed", "Could not read library")
		return
	}
	writeJSON(w, 200, map[string]any{"items": items, "count": len(items)})
}
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var q createRequest
	if decode(w, r, &q) != nil {
		writeError(w, 400, "invalid_json", "Request body must be valid JSON")
		return
	}
	q.WatchStatus = strings.ToUpper(q.WatchStatus)
	if !validStatus(q.WatchStatus) {
		writeError(w, 400, "invalid_watch_status", "Invalid watchStatus")
		return
	}
	if !validUUID(q.AnimeID) {
		writeError(w, 400, "invalid_anime_id", "animeId must be a UUID")
		return
	}
	if q.EpisodesWatched < 0 || q.EpisodesWatched > 10000 {
		writeError(w, 400, "invalid_progress", "episodesWatched must be between 0 and 10000")
		return
	}
	e, err := h.data.Add(r.Context(), q.AnimeID, q.WatchStatus, q.EpisodesWatched)
	if err != nil {
		if authorizationError(w, err) {
			return
		}
		if err.Error() == "anime_not_found" {
			writeError(w, 404, "anime_not_found", "Anime was not found")
		} else {
			writeError(w, 500, "tracker_write_failed", "Could not save library entry")
		}
		return
	}
	writeJSON(w, 201, e)
}
func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !validUUID(id) {
		writeError(w, 400, "invalid_id", "Tracker ID must be a UUID")
		return
	}
	var q updateRequest
	if decode(w, r, &q) != nil {
		writeError(w, 400, "invalid_json", "Request body must be valid JSON")
		return
	}
	if q.WatchStatus == nil && q.EpisodesWatched == nil {
		writeError(w, 400, "empty_update", "At least one field is required")
		return
	}
	if q.WatchStatus != nil {
		v := strings.ToUpper(*q.WatchStatus)
		if !validStatus(v) {
			writeError(w, 400, "invalid_watch_status", "Invalid watchStatus")
			return
		}
		q.WatchStatus = &v
	}
	if q.EpisodesWatched != nil && (*q.EpisodesWatched < 0 || *q.EpisodesWatched > 10000) {
		writeError(w, 400, "invalid_progress", "episodesWatched must be between 0 and 10000")
		return
	}
	var e store.Entry
 var ok bool
 var err error
 if q.ExpectedCycle!=nil{
  conditional,supported:=h.data.(store.CycleUpdater)
  if !supported||*q.ExpectedCycle<1||*q.ExpectedCycle>1000{writeError(w,400,"invalid_watch_cycle","Use a valid viewing pass in PostgreSQL mode");return}
  e,ok,err=conditional.UpdateCycle(r.Context(),id,q.WatchStatus,q.EpisodesWatched,*q.ExpectedCycle)
 }else{e,ok,err=h.data.Update(r.Context(),id,q.WatchStatus,q.EpisodesWatched)}
	if err != nil {
		if authorizationError(w, err) {
			return
		}
		if errors.Is(err,store.ErrWatchCycleConflict){writeError(w,409,"rewatch_state_changed","Viewing pass changed; refresh your library");return}
		writeError(w, 500, "tracker_update_failed", "Could not update library entry")
		return
	}
	if !ok {
		writeError(w, 404, "tracker_entry_not_found", "Library entry was not found")
		return
	}
	writeJSON(w, 200, e)
}
func (h *Handler) Rate(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !validUUID(id) {
		writeError(w, 400, "invalid_id", "Anime ID must be a UUID")
		return
	}
	var q ratingRequest
	if decode(w, r, &q) != nil {
		writeError(w, 400, "invalid_json", "Request body must be valid JSON")
		return
	}
	if q.Score < 1 || q.Score > 5 {
		writeError(w, 400, "invalid_score", "score must be between 1 and 5")
		return
	}
	e, ok, err := h.data.Rate(r.Context(), id, q.Score)
	if err != nil {
		if authorizationError(w, err) {
			return
		}
		writeError(w, 500, "rating_write_failed", "Could not save rating")
		return
	}
	if !ok {
		writeError(w, 404, "tracker_entry_not_found", "Add this anime to your library before rating it")
		return
	}
	writeJSON(w, 200, e)
}
func decode(w http.ResponseWriter, r *http.Request, v any) error {
	d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	d.DisallowUnknownFields()
	if err := d.Decode(v); err != nil {
		return err
	}
	var extra any
	if err := d.Decode(&extra); err != io.EOF {
		if err == nil {
			return fmt.Errorf("request body must contain exactly one JSON value")
		}
		return err
	}
	return nil
}
func validStatus(s string) bool {
	switch strings.ToUpper(s) {
	case "PLANNING", "WATCHING", "COMPLETED", "ON_HOLD", "DROPPED":
		return true
	}
	return false
}
func validUUID(s string) bool {
	if len(s) != 36 {
		return false
	}
	for i, c := range s {
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
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]string{"error": code, "message": message})
}

func authorizationError(w http.ResponseWriter, err error) bool {
	switch {
	case errors.Is(err, store.ErrUnauthorized):
		writeError(w, 401, "unauthorized", "A valid session is required")
	case errors.Is(err, store.ErrEmailUnverified):
		writeError(w, 403, "email_verification_required", "Verify your email before using your library")
	default:
		return false
	}
	return true
}

func (h *Handler) Remove(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !validUUID(id) {
		writeError(w, 400, "invalid_id", "Tracker ID must be a UUID")
		return
	}
	ok, err := h.data.Remove(r.Context(), id)
	if err != nil {
		if authorizationError(w, err) {
			return
		}
		writeError(w, 500, "tracker_delete_failed", "Could not remove library entry")
		return
	}
	if !ok {
		writeError(w, 404, "tracker_entry_not_found", "Library entry was not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
