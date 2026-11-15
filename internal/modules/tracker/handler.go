package tracker

import (
 "encoding/json"
 "net/http"
 "strings"
 "github.com/example/anime-intelligence/internal/platform/store"
)
type Handler struct{data *store.Store}
func NewHandler(data *store.Store)*Handler{return &Handler{data:data}}
type createRequest struct{AnimeID string `json:"animeId"`;WatchStatus string `json:"watchStatus"`;EpisodesWatched int `json:"episodesWatched"`}
type updateRequest struct{WatchStatus *string `json:"watchStatus"`;EpisodesWatched *int `json:"episodesWatched"`}
type ratingRequest struct{Score int `json:"score"`}
func(h *Handler)List(w http.ResponseWriter,r *http.Request){items:=h.data.Library();writeJSON(w,200,map[string]any{"items":items,"count":len(items)})}
func(h *Handler)Create(w http.ResponseWriter,r *http.Request){var q createRequest;if decode(w,r,&q)!=nil{writeError(w,400,"invalid_json","Request body must be valid JSON");return};q.WatchStatus=strings.ToUpper(q.WatchStatus);if !validStatus(q.WatchStatus){writeError(w,400,"invalid_watch_status","Invalid watchStatus");return};if !validUUID(q.AnimeID){writeError(w,400,"invalid_anime_id","animeId must be a UUID");return};if q.EpisodesWatched<0||q.EpisodesWatched>10000{writeError(w,400,"invalid_progress","episodesWatched must be between 0 and 10000");return};e,err:=h.data.Add(q.AnimeID,q.WatchStatus,q.EpisodesWatched);if err!=nil{if err.Error()=="anime_not_found"{writeError(w,404,"anime_not_found","Anime was not found")}else{writeError(w,500,"tracker_write_failed","Could not save library entry")};return};writeJSON(w,201,e)}
func(h *Handler)Update(w http.ResponseWriter,r *http.Request){id:=r.PathValue("id");if !validUUID(id){writeError(w,400,"invalid_id","Tracker ID must be a UUID");return};var q updateRequest;if decode(w,r,&q)!=nil{writeError(w,400,"invalid_json","Request body must be valid JSON");return};if q.WatchStatus==nil&&q.EpisodesWatched==nil{writeError(w,400,"empty_update","At least one field is required");return};if q.WatchStatus!=nil{v:=strings.ToUpper(*q.WatchStatus);if !validStatus(v){writeError(w,400,"invalid_watch_status","Invalid watchStatus");return};q.WatchStatus=&v};if q.EpisodesWatched!=nil&&(*q.EpisodesWatched<0||*q.EpisodesWatched>10000){writeError(w,400,"invalid_progress","episodesWatched must be between 0 and 10000");return};e,ok,err:=h.data.Update(id,q.WatchStatus,q.EpisodesWatched);if err!=nil{writeError(w,500,"tracker_update_failed","Could not update library entry");return};if !ok{writeError(w,404,"tracker_entry_not_found","Library entry was not found");return};writeJSON(w,200,e)}
func(h *Handler)Rate(w http.ResponseWriter,r *http.Request){id:=r.PathValue("id");if !validUUID(id){writeError(w,400,"invalid_id","Anime ID must be a UUID");return};var q ratingRequest;if decode(w,r,&q)!=nil{writeError(w,400,"invalid_json","Request body must be valid JSON");return};if q.Score<1||q.Score>5{writeError(w,400,"invalid_score","score must be between 1 and 5");return};e,ok,err:=h.data.Rate(id,q.Score);if err!=nil{writeError(w,500,"rating_write_failed","Could not save rating");return};if !ok{writeError(w,404,"tracker_entry_not_found","Add this anime to your library before rating it");return};writeJSON(w,200,e)}
func decode(w http.ResponseWriter,r *http.Request,v any)error{d:=json.NewDecoder(http.MaxBytesReader(w,r.Body,1<<20));d.DisallowUnknownFields();return d.Decode(v)}
func validStatus(s string)bool{switch strings.ToUpper(s){case "PLANNING","WATCHING","COMPLETED","ON_HOLD","DROPPED":return true};return false}
func validUUID(s string)bool{return len(s)==36&&s[8]=='-'&&s[13]=='-'&&s[18]=='-'&&s[23]=='-'}
func writeJSON(w http.ResponseWriter,status int,v any){w.Header().Set("Content-Type","application/json; charset=utf-8");w.WriteHeader(status);_=json.NewEncoder(w).Encode(v)}
func writeError(w http.ResponseWriter,status int,code,message string){writeJSON(w,status,map[string]string{"error":code,"message":message})}
