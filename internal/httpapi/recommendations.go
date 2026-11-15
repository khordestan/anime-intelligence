package httpapi

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/khordestan/anime-intelligence/internal/platform/postgres"
)
func registerRecommendationRoutes(mux *http.ServeMux,db *postgres.DB,protect func(func(http.ResponseWriter,*http.Request,string,postgres.User))http.HandlerFunc){
	mux.HandleFunc("GET /api/v1/preferences",protect(func(w http.ResponseWriter,r *http.Request,token string,_ postgres.User){
		items,err:=db.Repository(token).FeaturePreferences(r.Context())
		if err!=nil{releaseError(w,err);return}
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
		if err!=nil{releaseError(w,err);return}
		if !found{authError(w,404,"feature_not_found");return}
		w.WriteHeader(204)
	})}
	mux.HandleFunc("PUT /api/v1/preferences/{key}",change(false))
	mux.HandleFunc("DELETE /api/v1/preferences/{key}",change(true))
	mux.HandleFunc("GET /api/v1/recommendations",protect(func(w http.ResponseWriter,r *http.Request,token string,_ postgres.User){
		limit,err:=strconv.Atoi(r.URL.Query().Get("limit"))
		if err!=nil{limit=20}
		page,err:=db.Repository(token).PersonalMatches(r.Context(),limit)
		if err!=nil{releaseError(w,err);return}
		writeJSON(w,200,page)
	}))
}
