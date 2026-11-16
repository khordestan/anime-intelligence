package postgres_test

import (
 "context"
 "encoding/json"
 "errors"
 "io"
 "log/slog"
 "net/http/httptest"
 "strings"
 "testing"
 "time"

 "github.com/khordestan/anime-intelligence/internal/httpapi"
 "github.com/khordestan/anime-intelligence/internal/platform/postgres"
 "github.com/khordestan/anime-intelligence/internal/platform/store"
)
func TestPrivateNotesVersionIsolationExportAndRewatch(t *testing.T){
 db,_,_,_:=testDB(t);ctx:=context.Background()
 a,err:=db.Register(ctx,"notes-one@example.test","synthetic-notes-password");if err!=nil{t.Fatal(err)}
 b,err:=db.Register(ctx,"notes-two@example.test","synthetic-notes-password");if err!=nil{t.Fatal(err)}
 if _,_,err=db.Repository(a.Token).TrackerNotes(ctx,"10000000-0000-4000-8000-000000000001");!errors.Is(err,store.ErrEmailUnverified){t.Fatal("unverified notes",err)}
 verifyAccount(t,db,a.Token);verifyAccount(t,db,b.Token)
 const anime="10000000-0000-4000-8000-000000000001"
 first,err:=db.Repository(a.Token).Add(ctx,anime,"COMPLETED",12);if err!=nil{t.Fatal(err)}
 second,err:=db.Repository(b.Token).Add(ctx,anime,"WATCHING",1);if err!=nil{t.Fatal(err)}
 before,ok,err:=db.Repository(a.Token).TrackerNotes(ctx,first.ID)
 if err!=nil||!ok||before.Notes!=""||before.UpdatedAt.IsZero(){t.Fatal("initial notes",before,err)}
 value:="Synthetic private note\n<script>plain text</script> · نمونه"
 saved,ok,err:=db.Repository(a.Token).SetTrackerNotes(ctx,first.ID,value,before.UpdatedAt)
 if err!=nil||!ok||saved.Notes!=value||!saved.UpdatedAt.After(before.UpdatedAt){t.Fatal("save notes",saved,err)}
 if _,_,err=db.Repository(a.Token).SetTrackerNotes(ctx,first.ID,"stale overwrite",before.UpdatedAt);!errors.Is(err,postgres.ErrNotesConflict){t.Fatal("lost update",err)}
 if _,ok,err=db.Repository(b.Token).TrackerNotes(ctx,first.ID);err!=nil||ok{t.Fatal("foreign read",err)}
 if _,ok,err=db.Repository(b.Token).SetTrackerNotes(ctx,first.ID,"foreign overwrite",saved.UpdatedAt);err!=nil||ok{t.Fatal("foreign write",err)}
 own,_,err:=db.Repository(b.Token).TrackerNotes(ctx,second.ID);if err!=nil||own.Notes!=""{t.Fatal("shared title note leaked",own,err)}
 if _,_,err=db.Repository(a.Token).SetTrackerNotes(ctx,first.ID,strings.Repeat("界",2001),saved.UpdatedAt);!errors.Is(err,postgres.ErrNotes){t.Fatal("oversized Unicode notes",err)}
 if _,_,err=db.Repository(a.Token).SetTrackerNotes(ctx,first.ID,"",time.Time{});!errors.Is(err,postgres.ErrNotes){t.Fatal("missing version",err)}
 if _,_,err=db.Repository(a.Token).StartRewatch(ctx,first.ID,1);err!=nil{t.Fatal(err)}
 current,_,err:=db.Repository(a.Token).TrackerNotes(ctx,first.ID)
 if err!=nil||current.Notes!=value{t.Fatal("rewatch erased notes",current,err)}
 if _,_,err=db.Repository(a.Token).SetTrackerNotes(ctx,first.ID,"stale after rewatch",saved.UpdatedAt);!errors.Is(err,postgres.ErrNotesConflict){t.Fatal("tracker change did not invalidate note revision",err)}
 exported,err:=db.ExportAccount(ctx,a.Token);if err!=nil{t.Fatal(err)}
 var rows []struct{Notes string `json:"notes"`}
 if err=json.Unmarshal(exported.Data["tracker"],&rows);err!=nil||len(rows)!=1||rows[0].Notes!=value{t.Fatal("notes export",err)}
 cleared,ok,err:=db.Repository(a.Token).SetTrackerNotes(ctx,first.ID,"",current.UpdatedAt)
 if err!=nil||!ok||cleared.Notes!=""{t.Fatal("clear notes",cleared,err)}
 if ok,err:=db.Repository(a.Token).Remove(ctx,first.ID);err!=nil||!ok{t.Fatal(err)}
 if _,ok,err=db.Repository(a.Token).TrackerNotes(ctx,first.ID);err!=nil||ok{t.Fatal("removed tracker retained notes",err)}
}
func TestNotesHTTPValidationAndAuthorization(t *testing.T){
 db,_,_,_:=testDB(t);ctx:=context.Background()
 s,err:=db.Register(ctx,"notes-http@example.test","synthetic-notes-password");if err!=nil{t.Fatal(err)}
 h:=httpapi.NewPostgresRouter(slog.New(slog.NewTextHandler(io.Discard,nil)),db)
 call:=func(method,path,token,body string)*httptest.ResponseRecorder{r:=httptest.NewRequest(method,path,strings.NewReader(body));if token!=""{r.Header.Set("Authorization","Bearer "+token)};w:=httptest.NewRecorder();h.ServeHTTP(w,r);return w}
 const anime="10000000-0000-4000-8000-000000000001"
 for _,method:=range []string{"GET","PUT"}{
  path:="/api/v1/tracker/"+anime+"/notes"
  if w:=call(method,path,"","{}");w.Code!=401{t.Fatal("unprotected notes",w.Code)}
  if w:=call(method,path,s.Token,"{}");w.Code!=403{t.Fatal("unverified notes",w.Code)}
 }
 verifyAccount(t,db,s.Token)
 entry,err:=db.Repository(s.Token).Add(ctx,anime,"WATCHING",1);if err!=nil{t.Fatal(err)}
 path:="/api/v1/tracker/"+entry.ID+"/notes"
 for _,body:=range []string{`{}`,`{"notes":null}`,`{"notes":"x","expectedUpdatedAt":"bad"}`,`{"notes":"x","expectedUpdatedAt":"2026-01-01T00:00:00Z","userId":"spoofed"}`}{
  if w:=call("PUT",path,s.Token,body);w.Code!=400{t.Fatal("invalid note body",w.Code)}
 }
 read:=call("GET",path,s.Token,"")
 var notes postgres.TrackerNotes
 if read.Code!=200||json.Unmarshal(read.Body.Bytes(),&notes)!=nil||read.Header().Get("Cache-Control")!="no-store"{t.Fatal("notes response",read.Code)}
 body,_:=json.Marshal(map[string]any{"notes":"Synthetic note","expectedUpdatedAt":notes.UpdatedAt})
 if w:=call("PUT",path,s.Token,string(body));w.Code!=200{t.Fatal("notes save",w.Code,w.Body.String())}
 if w:=call("PUT",path,s.Token,string(body));w.Code!=409{t.Fatal("notes replay",w.Code)}
}
