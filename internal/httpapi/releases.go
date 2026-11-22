package httpapi

import (
	"errors"
	"net/http"

	"github.com/khordestan/anime-intelligence/internal/platform/postgres"
	"github.com/khordestan/anime-intelligence/internal/platform/store"
)

func releaseError(w http.ResponseWriter,err error) {
	switch {
	case errors.Is(err,postgres.ErrNotificationCursor): authError(w,400,"invalid_notification_cursor")
	case errors.Is(err,store.ErrEmailUnverified): authError(w,403,"email_verification_required")
	case errors.Is(err,postgres.ErrUnauthorized): authError(w,401,"unauthorized")
	case errors.Is(err,postgres.ErrFollowLimit): authError(w,409,"follow_limit_reached")
	case errors.Is(err,postgres.ErrFollowScope): authError(w,400,"invalid_follow_scope")
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
		var body struct {Track string `json:"track"`; Scope string `json:"scope"`}
		if decodeAuth(w,r,&body)!=nil {authError(w,400,"invalid_json");return}
		if body.Track=="" {body.Track="SUB"}
		if body.Scope=="" {body.Scope="ANIME"}
		found,err:=db.Repository(token).FollowScope(r.Context(),id,body.Track,body.Scope)
		if err!=nil {releaseError(w,err);return}
		if !found {authError(w,404,"anime_not_found");return}
		w.WriteHeader(204)
	}))
	mux.HandleFunc("DELETE /api/v1/follows/{id}",protect(func(w http.ResponseWriter,r *http.Request,token string,_ postgres.User){
		id:=r.PathValue("id")
		if !validSessionID(id) {authError(w,400,"invalid_anime_id");return}
		scope:=r.URL.Query().Get("scope")
		if scope=="" {scope="ANIME"}
		found,err:=db.Repository(token).UnfollowScope(r.Context(),id,scope)
		if err!=nil {releaseError(w,err);return}
		if !found {authError(w,404,"follow_not_found");return}
		w.WriteHeader(204)
	}))
	mux.HandleFunc("GET /api/v1/notifications",protect(func(w http.ResponseWriter,r *http.Request,token string,_ postgres.User){
		query := r.URL.Query()
		var unreadOnly bool
		if values, ok := query["unreadOnly"]; ok {
			if len(values) != 1 || (values[0] != "true" && values[0] != "false") {
				authError(w, 400, "invalid_notification_filter")
				return
			}
			unreadOnly = values[0] == "true"
		}
		page,err:=db.Repository(token).FilterNotificationHistory(r.Context(),query.Get("cursor"),unreadOnly)
		if err!=nil {releaseError(w,err);return}
		response:=map[string]any{"items":page.Items,"count":len(page.Items),"limit":100,"nextCursor":page.NextCursor}
		if page.Materialization!=nil{response["materialization"]=page.Materialization}
		writeJSON(w,200,response)
	}))
	mux.HandleFunc("PATCH /api/v1/notifications/read-all", protect(func(w http.ResponseWriter, r *http.Request, token string, _ postgres.User) {
		updated, err := db.Repository(token).ReadAllNotifications(r.Context())
		if err != nil {
			releaseError(w, err)
			return
		}
		writeJSON(w, 200, map[string]int64{"updated": updated})
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
