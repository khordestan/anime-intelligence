package httpapi

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/khordestan/anime-intelligence/internal/platform/postgres"
	"github.com/khordestan/anime-intelligence/internal/platform/store"
)
func matchError(w http.ResponseWriter,err error){
	if errors.Is(err,store.ErrEmailUnverified){authError(w,403,"email_verification_required");return}
	if errors.Is(err,postgres.ErrUnauthorized){w.Header().Set("WWW-Authenticate","Bearer");authError(w,401,"unauthorized");return}
	authError(w,500,"recommendation_operation_failed")
}
func registerRecommendationRoutes(mux *http.ServeMux,db *postgres.DB,protect func(func(http.ResponseWriter,*http.Request,string,postgres.User))http.HandlerFunc){
	mux.HandleFunc("GET /api/v1/preferences",protect(func(w http.ResponseWriter,r *http.Request,token string,_ postgres.User){
		items,err:=db.Repository(token).FeaturePreferences(r.Context())
		if err!=nil{matchError(w,err);return}
		writeJSON(w,200,map[string]any{"items":items,"limit":256})
	}))
	change:=func(remove bool)http.HandlerFunc{return protect(func(w http.ResponseWriter,r *http.Request,token string,_ postgres.User){
		key:=r.PathValue("key")
		if len(key)==0||len(key)>128{authError(w,400,"invalid_feature");return}
		var weight *float64
		if !remove{
			var body struct{Weight *float64 `json:"weight"`}
			if decodeAuth(w,r,&body)!=nil||body.Weight==nil{authError(w,400,"invalid_preference");return}
			weight=body.Weight
		}
		found,err:=db.Repository(token).SetFeaturePreference(r.Context(),key,weight)
		if errors.Is(err,postgres.ErrPreference){authError(w,400,"invalid_preference");return}
		if err!=nil{matchError(w,err);return}
		if !found{authError(w,404,"feature_not_found");return}
		w.WriteHeader(204)
	})}
	mux.HandleFunc("PUT /api/v1/preferences/{key}",change(false))
	mux.HandleFunc("DELETE /api/v1/preferences/{key}",change(true))
	mux.HandleFunc("GET /api/v1/recommendations/dismissals", protect(func(w http.ResponseWriter, r *http.Request, token string, _ postgres.User) {
		after := r.URL.Query().Get("after")
		if after != "" && !validSessionID(after) {
			authError(w, 400, "invalid_dismissal_cursor")
			return
		}
		page, err := db.Repository(token).PersonalMatchDismissals(r.Context(), after)
		if err != nil {
			matchError(w, err)
			return
		}
		writeJSON(w, 200, page)
	}))
	setDismissed := func(dismissed bool) http.HandlerFunc {
		return protect(func(w http.ResponseWriter, r *http.Request, token string, _ postgres.User) {
			id := r.PathValue("id")
			if !validSessionID(id) {
				authError(w, 400, "invalid_anime_id")
				return
			}
			found, err := db.Repository(token).SetPersonalMatchDismissed(r.Context(), id, dismissed)
			if err != nil {
				matchError(w, err)
				return
			}
			if !found {
				authError(w, 404, "anime_not_found")
				return
			}
			w.WriteHeader(204)
		})
	}
	mux.HandleFunc("PUT /api/v1/recommendations/dismissals/{id}", setDismissed(true))
	mux.HandleFunc("DELETE /api/v1/recommendations/dismissals/{id}", setDismissed(false))
	feedback := func(remove bool) http.HandlerFunc {
		return protect(func(w http.ResponseWriter, r *http.Request, token string, _ postgres.User) {
			id := r.PathValue("id")
			if !validSessionID(id) {
				authError(w, 400, "invalid_anime_id")
				return
			}
			direction := 0
			if !remove {
				var body struct { Direction int `json:"direction"` }
				if decodeAuth(w, r, &body) != nil || (body.Direction != 1 && body.Direction != -1) {
					authError(w, 400, "invalid_feedback_direction")
					return
				}
				direction = body.Direction
			}
			ok, err := db.Repository(token).SetDiscoveryTitleFeedback(r.Context(), id, direction)
			if err != nil {
				matchError(w, err)
				return
			}
			if !ok {
				authError(w, 404, "discovery_title_unavailable")
				return
			}
			w.WriteHeader(204)
		})
	}
	mux.HandleFunc("GET /api/v1/recommendations/discovery/feedback", protect(func(w http.ResponseWriter, r *http.Request, token string, _ postgres.User) {
		after := r.URL.Query().Get("after")
		if after != "" && !validSessionID(after) {
			authError(w, 400, "invalid_feedback_cursor")
			return
		}
		page, err := db.Repository(token).DiscoveryTitleFeedback(r.Context(), after)
		if err != nil {
			matchError(w, err)
			return
		}
		writeJSON(w, 200, page)
	}))
	mux.HandleFunc("PUT /api/v1/recommendations/discovery/feedback/{id}", feedback(false))
	mux.HandleFunc("DELETE /api/v1/recommendations/discovery/feedback/{id}", feedback(true))
	mux.HandleFunc("GET /api/v1/recommendations/discovery", protect(func(w http.ResponseWriter, r *http.Request, token string, _ postgres.User) {
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		query := r.URL.Query()
		featureKey := query.Get("featureKey")
		if len(featureKey) > 128 || len(query["featureKey"]) > 1 {
			authError(w, 400, "invalid_feature")
			return
		}
		for _, flag := range []string{"unfamiliar", "afterUnfamiliar"} {
			if values, present := query[flag]; present && (len(values) != 1 || (values[0] != "true" && values[0] != "false")) {
				authError(w, 400, "invalid_discovery_order")
				return
			}
		}
		unfamiliar := query.Get("unfamiliar") == "true"
		var cursor *postgres.DiscoveryCursor
		if query.Has("after") || query.Has("afterCount") || query.Has("afterFeature") || query.Has("afterUnfamiliar") {
			id := query.Get("after")
			count, err := strconv.Atoi(query.Get("afterCount"))
			if err != nil || count < 1 || !validSessionID(id) || len(query["after"]) != 1 || len(query["afterCount"]) != 1 || len(query["afterFeature"]) > 1 || query.Get("afterFeature") != featureKey || (query.Get("afterUnfamiliar") == "true") != unfamiliar {
				authError(w, 400, "invalid_discovery_cursor")
				return
			}
			cursor = &postgres.DiscoveryCursor{AnimeID: id, EvidenceCount: count, FeatureKey: featureKey, Unfamiliar: unfamiliar}
		}
		page, err := db.Repository(token).DiscoverWithOptions(r.Context(), limit, cursor, featureKey, unfamiliar)
		if err != nil {
			matchError(w, err)
			return
		}
		writeJSON(w, 200, page)
	}))
	mux.HandleFunc("GET /api/v1/recommendations",protect(func(w http.ResponseWriter,r *http.Request,token string,_ postgres.User){
		limit,err:=strconv.Atoi(r.URL.Query().Get("limit"))
		if err!=nil{limit=20}
		page,err:=db.Repository(token).PersonalMatches(r.Context(),limit)
		if err!=nil{matchError(w,err);return}
		writeJSON(w,200,page)
	}))
}
