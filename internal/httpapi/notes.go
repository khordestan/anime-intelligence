package httpapi

import (
 "encoding/json"
 "errors"
 "io"
 "net/http"
 "time"

 "github.com/khordestan/anime-intelligence/internal/platform/postgres"
 "github.com/khordestan/anime-intelligence/internal/platform/store"
)
func notesError(w http.ResponseWriter,err error){
 switch{
 case errors.Is(err,store.ErrEmailUnverified):authError(w,403,"email_verification_required")
 case errors.Is(err,postgres.ErrUnauthorized):w.Header().Set("WWW-Authenticate","Bearer");authError(w,401,"unauthorized")
 case errors.Is(err,postgres.ErrNotes):authError(w,400,"invalid_tracker_notes")
 case errors.Is(err,postgres.ErrNotesConflict):authError(w,409,"tracker_notes_conflict")
 default:authError(w,500,"tracker_notes_operation_failed")
 }
}
func registerNotesRoutes(mux *http.ServeMux,db *postgres.DB,protect func(func(http.ResponseWriter,*http.Request,string,postgres.User))http.HandlerFunc){
 mux.HandleFunc("GET /api/v1/tracker/{id}/notes",protect(func(w http.ResponseWriter,r *http.Request,token string,user postgres.User){
  if !user.EmailVerified{authError(w,403,"email_verification_required");return}
  if !validSessionID(r.PathValue("id")){authError(w,400,"invalid_tracker_id");return}
  notes,ok,err:=db.Repository(token).TrackerNotes(r.Context(),r.PathValue("id"))
  if err!=nil{notesError(w,err);return}
  if !ok{authError(w,404,"tracker_not_found");return}
  writeJSON(w,200,notes)
 }))
 mux.HandleFunc("PUT /api/v1/tracker/{id}/notes",protect(func(w http.ResponseWriter,r *http.Request,token string,user postgres.User){
  if !user.EmailVerified{authError(w,403,"email_verification_required");return}
  if !validSessionID(r.PathValue("id")){authError(w,400,"invalid_tracker_id");return}
  var body struct{Notes *string `json:"notes"`;ExpectedUpdatedAt *time.Time `json:"expectedUpdatedAt"`}
  d:=json.NewDecoder(http.MaxBytesReader(w,r.Body,64<<10));d.DisallowUnknownFields()
  var extra any
  if d.Decode(&body)!=nil||d.Decode(&extra)!=io.EOF||body.Notes==nil||body.ExpectedUpdatedAt==nil{authError(w,400,"invalid_json");return}
  notes,ok,err:=db.Repository(token).SetTrackerNotes(r.Context(),r.PathValue("id"),*body.Notes,*body.ExpectedUpdatedAt)
  if err!=nil{notesError(w,err);return}
  if !ok{authError(w,404,"tracker_not_found");return}
  writeJSON(w,200,notes)
 }))
}
