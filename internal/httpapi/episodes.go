package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/khordestan/anime-intelligence/internal/platform/postgres"
)
func registerEpisodeRoutes(mux *http.ServeMux,db *postgres.DB,protect func(func(http.ResponseWriter,*http.Request,string,postgres.User)) http.HandlerFunc) {
	mux.HandleFunc("GET /api/v1/tracker/{id}/episodes",protect(func(w http.ResponseWriter,r *http.Request,token string,_ postgres.User){
		id:=r.PathValue("id")
		if !validSessionID(id) {authError(w,400,"invalid_tracker_id");return}
		progress,found,err:=db.Repository(token).EpisodeProgress(r.Context(),id)
		if err!=nil {releaseError(w,err);return}
		if !found {authError(w,404,"tracker_not_found");return}
		writeJSON(w,200,progress)
	}))
	mux.HandleFunc("PUT /api/v1/tracker/{id}/episodes",protect(func(w http.ResponseWriter,r *http.Request,token string,_ postgres.User){
		id:=r.PathValue("id")
		if !validSessionID(id) {authError(w,400,"invalid_tracker_id");return}
		var body struct {Numbers []int32 `json:"numbers"`}
		d:=json.NewDecoder(http.MaxBytesReader(w,r.Body,128<<10))
		d.DisallowUnknownFields()
		var extra any
		if d.Decode(&body)!=nil || d.Decode(&extra)!=io.EOF {authError(w,400,"invalid_json");return}
		progress,found,err:=db.Repository(token).SetEpisodeProgress(r.Context(),id,body.Numbers)
		if errors.Is(err,postgres.ErrEpisodes) {authError(w,400,"invalid_episode_selection");return}
		if err!=nil {releaseError(w,err);return}
		if !found {authError(w,404,"tracker_not_found");return}
		writeJSON(w,200,progress)
	}))
}
