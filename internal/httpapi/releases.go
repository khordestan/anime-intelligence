package httpapi

import (
	"errors"
	"net/http"

	"github.com/khordestan/anime-intelligence/internal/platform/postgres"
	"github.com/khordestan/anime-intelligence/internal/platform/store"
)

func releaseError(w http.ResponseWriter,err error) {
	switch {
	case errors.Is(err,store.ErrEmailUnverified): authError(w,403,"email_verification_required")
	case errors.Is(err,postgres.ErrUnauthorized): authError(w,401,"unauthorized")
	case errors.Is(err,postgres.ErrReleaseTrack): authError(w,400,"invalid_release_track")
	default: authError(w,500,"release_operation_failed")
	}
}
func registerReleaseRoutes(mux *http.ServeMux,db *postgres.DB,protect func(func(http.ResponseWriter,*http.Request,string,postgres.User)) http.HandlerFunc) {
	mux.HandleFunc("GET /api/v1/follows",protect(func(w http.ResponseWriter,r *http.Request,token string,_ postgres.User){
		items,err:=db.Repository(token).Follows(r.Context())
		if err!=nil {releaseError(w,err);return}
		writeJSON(w,200,map[string]any{"items":items,"count":len(items),"limit":1000})
	}))
	mux.HandleFunc("PUT /api/v1/follows/{id}",protect(func(w http.ResponseWriter,r *http.Request,token string,_ postgres.User){
		id:=r.PathValue("id")
		if !validSessionID(id) {authError(w,400,"invalid_anime_id");return}
		var body struct {Track string `json:"track"`}
		if decodeAuth(w,r,&body)!=nil {authError(w,400,"invalid_json");return}
		if body.Track=="" {body.Track="SUB"}
		found,err:=db.Repository(token).Follow(r.Context(),id,body.Track)
		if err!=nil {releaseError(w,err);return}
		if !found {authError(w,404,"anime_not_found");return}
		w.WriteHeader(204)
	}))
	mux.HandleFunc("DELETE /api/v1/follows/{id}",protect(func(w http.ResponseWriter,r *http.Request,token string,_ postgres.User){
		id:=r.PathValue("id")
		if !validSessionID(id) {authError(w,400,"invalid_anime_id");return}
		found,err:=db.Repository(token).Unfollow(r.Context(),id)
		if err!=nil {releaseError(w,err);return}
		if !found {authError(w,404,"follow_not_found");return}
		w.WriteHeader(204)
	}))
	mux.HandleFunc("GET /api/v1/notifications",protect(func(w http.ResponseWriter,r *http.Request,token string,_ postgres.User){
		items,err:=db.Repository(token).Notifications(r.Context())
		if err!=nil {releaseError(w,err);return}
		writeJSON(w,200,map[string]any{"items":items,"count":len(items),"limit":100})
	}))
	mux.HandleFunc("PATCH /api/v1/notifications/{id}/read",protect(func(w http.ResponseWriter,r *http.Request,token string,_ postgres.User){
		id:=r.PathValue("id")
		if !validSessionID(id) {authError(w,400,"invalid_notification_id");return}
		found,err:=db.Repository(token).ReadNotification(r.Context(),id)
		if err!=nil {releaseError(w,err);return}
		if !found {authError(w,404,"notification_not_found");return}
		w.WriteHeader(204)
	}))
}
