package postgres_test

import (
 "context"
 "encoding/json"
 "io"
 "log/slog"
 "net/http/httptest"
 "strings"
 "testing"
 "time"

 "github.com/khordestan/anime-intelligence/internal/httpapi"
 "github.com/khordestan/anime-intelligence/internal/platform/postgres"
)

func TestNotificationMaterializationBacklogIsBoundedDeduplicatedAndOwned(t *testing.T){
 db,owner,_,_:=testDB(t);ctx:=context.Background()
 one,err:=db.Register(ctx,"backlog-one@example.test","synthetic-backlog-password");if err!=nil{t.Fatal(err)}
 two,err:=db.Register(ctx,"backlog-two@example.test","synthetic-backlog-password");if err!=nil{t.Fatal(err)}
 verifyAccount(t,db,one.Token);verifyAccount(t,db,two.Token)
 const root="10000000-0000-4000-8000-000000000001"
 const linked="10000000-0000-4000-8000-000000000002"
 const unrelated="10000000-0000-4000-8000-000000000003"
 a,b:=db.Repository(one.Token),db.Repository(two.Token)
 if ok,err:=a.Follow(ctx,root,"SUB");err!=nil||!ok{t.Fatal(err)}
 if ok,err:=a.FollowScope(ctx,linked,"SUB","FRANCHISE");err!=nil||!ok{t.Fatal(err)}
 now:=time.Now().UTC().Truncate(time.Microsecond)
 if _,err=owner.Exec(ctx,`INSERT INTO anime_relations(source_anime_id,target_anime_id,relation_type) VALUES($1,$2,'SEQUEL')`,linked,root);err!=nil{t.Fatal(err)}
 if _,err=owner.Exec(ctx,`UPDATE user_follows SET created_at=$1 WHERE user_id=$2`,now.Add(-3*time.Hour),one.User.ID);err!=nil{t.Fatal(err)}
 // An existing old notice makes a history cursor available during catch-up.
 if _,err=owner.Exec(ctx,`INSERT INTO notifications(user_id,channel,title,body,created_at) VALUES($1,'IN_APP','Synthetic earlier history','Synthetic earlier history',$2)`,one.User.ID,now.Add(-4*time.Hour));err!=nil{t.Fatal(err)}
 for _,fixture:=range []struct{key,id,track string;count int;at time.Time}{
  {"sub",root,"SUB",101,now.Add(-2*time.Hour)},
  {"dub",root,"DUB",120,now.Add(-2*time.Hour)},
  {"old",root,"SUB",5,now.Add(-5*time.Hour)},
  {"unrelated",unrelated,"SUB",125,now.Add(-2*time.Hour)},
 }{
  if _,err=owner.Exec(ctx,`INSERT INTO domain_events(event_key,event_type,aggregate_type,aggregate_id,payload,occurred_at)
 SELECT $1||':'||n,'EPISODE_RELEASED','ANIME',$2::uuid,jsonb_build_object('number',n,'track',$3::text),$4::timestamptz+n*interval '1 second' FROM generate_series(1,$5::int) n`,fixture.key,fixture.id,fixture.track,fixture.at,fixture.count);err!=nil{t.Fatal(err)}
 }
 assertStatus:=func(page postgres.NotificationPage,created int,more bool){
  t.Helper()
  if page.Materialization==nil||page.Materialization.Created!=created||page.Materialization.HasMore!=more{t.Fatal("materialization status",page.Materialization,created,more)}
 }
 count:=func(want int){
  t.Helper();var actual int
  if err:=owner.QueryRow(ctx,`SELECT count(*) FROM notifications WHERE user_id=$1`,one.User.ID).Scan(&actual);err!=nil||actual!=want{t.Fatal("materialized batch/owner boundary",actual,want,err)}
 }
 first,err:=a.NotificationHistory(ctx,"")
 if err!=nil||len(first.Items)!=100||first.NextCursor==""{t.Fatal("initial catch-up",len(first.Items),err)}
 assertStatus(first,100,true);count(101)
 history,err:=a.NotificationHistory(ctx,first.NextCursor)
 if err!=nil||len(history.Items)!=1||history.Materialization!=nil{t.Fatal("history must not check/materialize backlog",history,err)}
 count(101)
 foreign,err:=b.NotificationHistory(ctx,"")
 if err!=nil||len(foreign.Items)!=0{t.Fatal("foreign backlog leaked",foreign,err)}
 assertStatus(foreign,0,false);count(101)
 next,err:=a.NotificationHistory(ctx,"")
 if err!=nil{t.Fatal(err)}
 assertStatus(next,1,false);count(102)
 repeated,err:=a.NotificationHistory(ctx,"");if err!=nil{t.Fatal(err)}
 assertStatus(repeated,0,false);count(102)
 // Changing one track admits the previously ineligible DUB backlog, while
 // overlapping franchise paths and existing notices remain deduplicated.
 if ok,err:=a.Follow(ctx,root,"ALL");err!=nil||!ok{t.Fatal(err)}
 h:=httpapi.NewPostgresRouter(slog.New(slog.NewTextHandler(io.Discard,nil)),db)
 request:=httptest.NewRequest("GET","/api/v1/notifications",nil);request.Header.Set("Authorization","Bearer "+one.Token)
 w:=httptest.NewRecorder();h.ServeHTTP(w,request)
 var body struct{Materialization *postgres.NotificationMaterialization `json:"materialization"`;NextCursor string `json:"nextCursor"`}
 if w.Code!=200||json.Unmarshal(w.Body.Bytes(),&body)!=nil||body.NextCursor==""||body.Materialization==nil||body.Materialization.Created!=100||!body.Materialization.HasMore||w.Header().Get("Cache-Control")!="no-store"{t.Fatal("HTTP backlog contract",w.Code,w.Body.String())}
 count(202)
 // The HTTP history representation omits status rather than claiming a checked
 // empty backlog. It must leave the remaining twenty DUB events untouched.
 request=httptest.NewRequest("GET","/api/v1/notifications?cursor="+body.NextCursor,nil);request.Header.Set("Authorization","Bearer "+one.Token)
 w=httptest.NewRecorder();h.ServeHTTP(w,request)
 if w.Code!=200||strings.Contains(w.Body.String(),`"materialization"`){t.Fatal("history status must be absent",w.Code)}
 count(202)
 last,err:=a.NotificationHistory(ctx,"");if err!=nil{t.Fatal(err)}
 assertStatus(last,20,false);count(222)
 for i,n:=range last.Items{if !strings.Contains(n.Body,"Episode"){t.Fatal("unexpected historical fixture on latest page",i)}}
}
