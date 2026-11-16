package httpapi

import (
 "errors"
 "net/http"
 "strconv"
 "time"

 "github.com/khordestan/anime-intelligence/internal/platform/postgres"
 "github.com/khordestan/anime-intelligence/internal/platform/store"
)
func registerCalendarRoutes(mux *http.ServeMux,db *postgres.DB,protect func(func(http.ResponseWriter,*http.Request,string,postgres.User))http.HandlerFunc){
 mux.HandleFunc("GET /api/v1/calendar",protect(func(w http.ResponseWriter,r *http.Request,token string,user postgres.User){
  if !user.EmailVerified{authError(w,403,"email_verification_required");return}
  today:=time.Now().UTC();from:=time.Date(today.Year(),today.Month(),today.Day(),0,0,0,0,time.UTC);days:=14
  if values,ok:=r.URL.Query()["from"];ok{
   if len(values)!=1{authError(w,400,"invalid_calendar_window");return}
   var err error;from,err=time.Parse("2006-01-02",values[0]);if err!=nil{authError(w,400,"invalid_calendar_window");return}
  }
  if values,ok:=r.URL.Query()["days"];ok{
   if len(values)!=1{authError(w,400,"invalid_calendar_window");return}
   var err error;days,err=strconv.Atoi(values[0]);if err!=nil{authError(w,400,"invalid_calendar_window");return}
  }
  calendar,err:=db.Repository(token).ReleaseCalendar(r.Context(),from,days)
  if err!=nil{
   switch{
   case errors.Is(err,postgres.ErrCalendarRange):authError(w,400,"invalid_calendar_window")
   case errors.Is(err,store.ErrEmailUnverified):authError(w,403,"email_verification_required")
   case errors.Is(err,postgres.ErrUnauthorized):w.Header().Set("WWW-Authenticate","Bearer");authError(w,401,"unauthorized")
   default:authError(w,500,"calendar_read_failed")
   };return
  }
  writeJSON(w,200,calendar)
 }))
}
