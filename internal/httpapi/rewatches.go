package httpapi

import (
 "errors"
 "net/http"
 "strconv"

 "github.com/khordestan/anime-intelligence/internal/platform/postgres"
 "github.com/khordestan/anime-intelligence/internal/platform/store"
)
func rewatchError(w http.ResponseWriter,err error){
 switch{
 case errors.Is(err,store.ErrEmailUnverified):authError(w,403,"email_verification_required")
 case errors.Is(err,postgres.ErrUnauthorized):w.Header().Set("WWW-Authenticate","Bearer");authError(w,401,"unauthorized")
 case errors.Is(err,postgres.ErrRewatchRequest):authError(w,400,"invalid_rewatch_request")
 case errors.Is(err,postgres.ErrRewatchState):authError(w,409,"rewatch_state_changed")
 default:authError(w,500,"rewatch_operation_failed")
 }
}
func registerRewatchRoutes(mux *http.ServeMux,db *postgres.DB,protect func(func(http.ResponseWriter,*http.Request,string,postgres.User))http.HandlerFunc){
 mux.HandleFunc("POST /api/v1/tracker/{id}/rewatches",protect(func(w http.ResponseWriter,r *http.Request,token string,user postgres.User){
  if !user.EmailVerified{authError(w,403,"email_verification_required");return}
  if !validSessionID(r.PathValue("id")){authError(w,400,"invalid_tracker_id");return}
  var body struct{ExpectedCycle int `json:"expectedCycle"`}
  if decodeAuth(w,r,&body)!=nil{authError(w,400,"invalid_json");return}
  entry,ok,err:=db.Repository(token).StartRewatch(r.Context(),r.PathValue("id"),body.ExpectedCycle)
  if err!=nil{rewatchError(w,err);return}
  if !ok{authError(w,404,"tracker_not_found");return}
  writeJSON(w,200,entry)
 }))
 mux.HandleFunc("GET /api/v1/tracker/{id}/rewatches",protect(func(w http.ResponseWriter,r *http.Request,token string,user postgres.User){
  if !user.EmailVerified{authError(w,403,"email_verification_required");return}
  if !validSessionID(r.PathValue("id")){authError(w,400,"invalid_tracker_id");return}
  before:=0
  if raw,exists:=r.URL.Query()["before"];exists{
   var err error
   if len(raw)!=1{authError(w,400,"invalid_rewatch_request");return}
   before,err=strconv.Atoi(raw[0])
   if err!=nil||before<1||before>1000{authError(w,400,"invalid_rewatch_request");return}
  }
  history,ok,err:=db.Repository(token).WatchHistory(r.Context(),r.PathValue("id"),before)
  if err!=nil{rewatchError(w,err);return}
  if !ok{authError(w,404,"tracker_not_found");return}
  writeJSON(w,200,history)
 }))
}
