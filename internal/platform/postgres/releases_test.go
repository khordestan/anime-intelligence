package postgres_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/khordestan/anime-intelligence/internal/httpapi"
	"github.com/khordestan/anime-intelligence/internal/modules/catalog/feed"
	"github.com/khordestan/anime-intelligence/internal/platform/catalogsync"
	"github.com/khordestan/anime-intelligence/internal/platform/store"
)
func TestReleaseAlertsOwnerIsolationAndDeduplication(t *testing.T) {
	db,owner,_,_:=testDB(t)
	ctx:=context.Background()
	one,err:=db.Register(ctx,"alerts-one@example.test","synthetic-alerts-password")
	if err!=nil {t.Fatal(err)}
	two,err:=db.Register(ctx,"alerts-two@example.test","synthetic-alerts-password")
	if err!=nil {t.Fatal(err)}
	if _,err=db.Repository(one.Token).Follows(ctx);!errors.Is(err,store.ErrEmailUnverified) {t.Fatal("unverified follows accepted",err)}
	verifyAccount(t,db,one.Token);verifyAccount(t,db,two.Token)
	now:=time.Now().UTC().Truncate(time.Microsecond)
	if _,err=owner.Exec(ctx,`INSERT INTO data_sources(provider_key,display_name,enabled,policy_url) VALUES('alerts-fixture','Synthetic alerts',true,'https://example.test/policy')`);err!=nil {t.Fatal(err)}
	s:=feed.Snapshot{Version:1,Provider:"alerts-fixture",ObservedAt:now.Add(-2*time.Hour),Titles:[]feed.Title{{ExternalID:"sky",Title:"Synthetic Alert Sky",MediaType:"TV",Status:"RELEASING",Episodes:[]feed.Episode{{Number:1,Track:"SUB"},{Number:1,Track:"DUB"},{Number:1,Track:"RAW"}}}}}
	if _,err=catalogsync.Import(ctx,owner,s,now);err!=nil {t.Fatal(err)}
	var id string
	if err=owner.QueryRow(ctx,`SELECT anime_id::text FROM anime_external_ids WHERE provider='alerts-fixture'`).Scan(&id);err!=nil {t.Fatal(err)}
	a,b:=db.Repository(one.Token),db.Repository(two.Token)
	if ok,err:=a.Follow(ctx,id,"SUB");err!=nil || !ok {t.Fatal(err)}
	if ok,err:=b.Follow(ctx,id,"DUB");err!=nil || !ok {t.Fatal(err)}
	if _,err=a.Follow(ctx,id,"INVALID");err==nil {t.Fatal("invalid track accepted")}
	if _,err=owner.Exec(ctx,`UPDATE user_follows SET created_at=$1 WHERE anime_id=$2`,now.Add(-90*time.Minute),id);err!=nil {t.Fatal(err)}
	released:=now.Add(-30*time.Minute)
	s.ObservedAt=now.Add(-20*time.Minute)
	for i:=range s.Titles[0].Episodes {s.Titles[0].Episodes[i].ReleasedAt=&released}
	result,err:=catalogsync.Import(ctx,owner,s,now)
	if err!=nil || result.ReleaseEvents!=3 {t.Fatalf("release events: %+v %v",result,err)}
	na,err:=a.Notifications(ctx)
	if err!=nil || len(na)!=1 || !strings.Contains(na[0].Body,"SUB") {t.Fatal("SUB preference",na,err)}
	nb,err:=b.Notifications(ctx)
	if err!=nil || len(nb)!=1 || !strings.Contains(nb[0].Body,"DUB") || nb[0].ID==na[0].ID {t.Fatal("DUB preference/account isolation",nb,err)}
	if ok,err:=b.ReadNotification(ctx,na[0].ID);err!=nil || ok {t.Fatal("foreign notification changed",err)}
	for range 2 {if ok,err:=a.ReadNotification(ctx,na[0].ID);err!=nil || !ok {t.Fatal("read must be idempotent",err)}}
	if ok,err:=a.Follow(ctx,id,"ALL");err!=nil || !ok {t.Fatal(err)}
	login,err:=db.Login(ctx,one.User.Email,"synthetic-alerts-password")
	if err!=nil {t.Fatal(err)}
	var wg sync.WaitGroup
	errs:=make(chan error,2)
	for _,token:=range []string{one.Token,login.Token} {
		wg.Add(1)
		go func(token string){defer wg.Done();_,err:=db.Repository(token).Notifications(ctx);errs<-err}(token)
	}
	wg.Wait();close(errs)
	for err:=range errs {if err!=nil {t.Fatal("concurrent polling",err)}}
	na,err=a.Notifications(ctx)
	if err!=nil || len(na)!=3 {t.Fatal("duplicate materialization or missing ALL tracks",na,err)}
	var count int
	if err=owner.QueryRow(ctx,`SELECT count(*) FROM notifications WHERE user_id=$1`,one.User.ID).Scan(&count);err!=nil || count!=3 {t.Fatal("durable duplicate suppression",count,err)}
	if ok,err:=b.Unfollow(ctx,id);err!=nil || !ok {t.Fatal(err)}
	follows,err:=a.Follows(ctx)
	if err!=nil || len(follows)!=1 || follows[0].Track!="ALL" {t.Fatal("unfollow changed other owner",err)}
	// A new follow must not replay releases from before subscription.
	if ok,err:=b.Follow(ctx,id,"ALL");err!=nil || !ok {t.Fatal(err)}
	nb,err=b.Notifications(ctx)
	if err!=nil || len(nb)!=1 {t.Fatal("new subscription replayed old RAW/SUB events",nb,err)}
	exported,err:=db.ExportAccount(ctx,one.Token)
	if err!=nil {t.Fatal(err)}
	encoded,_:=json.Marshal(exported)
	if strings.Contains(string(encoded),two.User.ID) || strings.Contains(string(encoded),two.User.Email) {t.Fatal("alerts export leaked account")}
}
func TestReleaseRoutesAuthorization(t *testing.T) {
	db,_,_,_:=testDB(t)
	ctx:=context.Background()
	s,err:=db.Register(ctx,"alerts-http@example.test","synthetic-alerts-password")
	if err!=nil {t.Fatal(err)}
	h:=httpapi.NewPostgresRouter(slog.New(slog.NewTextHandler(io.Discard,nil)),db)
	call:=func(method,path,token,body string)*httptest.ResponseRecorder{
		r:=httptest.NewRequest(method,path,strings.NewReader(body))
		if token!="" {r.Header.Set("Authorization","Bearer "+token)}
		w:=httptest.NewRecorder();h.ServeHTTP(w,r);return w
	}
	const id="10000000-0000-4000-8000-000000000001"
	for _,spec:=range [][2]string{{"GET","/api/v1/follows"},{"PUT","/api/v1/follows/"+id},{"DELETE","/api/v1/follows/"+id},{"GET","/api/v1/notifications"},{"PATCH","/api/v1/notifications/"+id+"/read"},{"PATCH","/api/v1/notifications/read-all"}} {
		if w:=call(spec[0],spec[1],"","{}");w.Code!=401 {t.Fatal("unprotected alert route",spec,w.Code)}
		if w:=call(spec[0],spec[1],s.Token,"{}");w.Code!=403 {t.Fatal("unverified alert route",spec,w.Code,w.Body.String())}
	}
	verifyAccount(t,db,s.Token)
	if w:=call("PUT","/api/v1/follows/"+id,s.Token,`{"track":"SUB","userId":"spoofed"}`);w.Code!=400 {t.Fatal("owner override accepted",w.Code)}
	if w:=call("PUT","/api/v1/follows/"+id,s.Token,`{"track":"SUB"}`);w.Code!=204 {t.Fatal("follow failed",w.Code,w.Body.String())}
	if w:=call("GET","/api/v1/follows",s.Token,"");w.Code!=200 || !strings.Contains(w.Body.String(),id) || w.Header().Get("Cache-Control")!="no-store" {t.Fatal("follow response",w.Code,w.Body.String())}
	if w:=call("GET","/api/v1/notifications",s.Token,"");w.Code!=200 || !strings.Contains(w.Body.String(),`"items":[]`) {t.Fatal("notification response",w.Code,w.Body.String())}
}

func TestNotificationHistoryStableBoundariesAndOwnerIsolation(t *testing.T){
 db,owner,_,_:=testDB(t)
 ctx:=context.Background()
 one,err:=db.Register(ctx,"history-one@example.test","synthetic-history-password")
 if err!=nil{t.Fatal(err)}
 two,err:=db.Register(ctx,"history-two@example.test","synthetic-history-password")
 if err!=nil{t.Fatal(err)}
 verifyAccount(t,db,one.Token);verifyAccount(t,db,two.Token)
 // Equal timestamps force the UUID tie-breaker across every page.
 at:=time.Now().UTC().Add(-time.Hour).Truncate(time.Microsecond)
 if _,err=owner.Exec(ctx,`INSERT INTO notifications(user_id,channel,title,body,created_at)
 SELECT $1,'IN_APP','Synthetic history','Synthetic notice '||n,$2 FROM generate_series(1,225) n`,one.User.ID,at);err!=nil{t.Fatal(err)}
 if _,err=owner.Exec(ctx,`INSERT INTO notifications(user_id,channel,title,body,created_at) VALUES($1,'IN_APP','Foreign history','Foreign synthetic notice',$2)`,two.User.ID,at);err!=nil{t.Fatal(err)}
 a,b:=db.Repository(one.Token),db.Repository(two.Token)
 page,err:=a.NotificationHistory(ctx,"")
 if err!=nil||len(page.Items)!=100||page.NextCursor==""{t.Fatal("first page",len(page.Items),err)}
 firstCursor:=page.NextCursor
 seen:=map[string]bool{}
 for _,n:=range page.Items{seen[n.ID]=true}
 // A concurrent newer notice must not shift an older page as OFFSET would.
 if _,err=owner.Exec(ctx,`INSERT INTO notifications(user_id,channel,title,body) VALUES($1,'IN_APP','Newer synthetic','Newer synthetic notice')`,one.User.ID);err!=nil{t.Fatal(err)}
 for i,want:=range []int{100,25}{
  page,err=a.NotificationHistory(ctx,page.NextCursor)
  if err!=nil||len(page.Items)!=want{t.Fatal("history page",i,len(page.Items),err)}
  for _,n:=range page.Items{
   if seen[n.ID]||n.Title=="Foreign history"||n.Title=="Newer synthetic"{t.Fatal("duplicate/foreign/shifted history")}
   seen[n.ID]=true
  }
  if (i==0)!=(page.NextCursor!=""){t.Fatal("end cursor",page.NextCursor)}
 }
 if len(seen)!=225{t.Fatal("history lost",len(seen))}
 other,err:=b.NotificationHistory(ctx,firstCursor)
 if err!=nil{t.Fatal(err)}
 for _,n:=range other.Items{if n.Title!="Foreign history"{t.Fatal("cursor bypassed ownership")}}
 // Marking an older page read does not move its sort position.
 second,err:=a.NotificationHistory(ctx,firstCursor)
 if err!=nil{t.Fatal(err)}
 id:=second.Items[0].ID
 if ok,err:=b.ReadNotification(ctx,id);err!=nil||ok{t.Fatal("foreign history read",err)}
 if ok,err:=a.ReadNotification(ctx,id);err!=nil||!ok{t.Fatal(err)}
 repeated,err:=a.NotificationHistory(ctx,firstCursor)
 if err!=nil||repeated.Items[0].ID!=id||repeated.Items[0].ReadAt==nil{t.Fatal("read moved boundary",err)}
 if updated, err := a.ReadAllNotifications(ctx); err != nil || updated != 224 {
  t.Fatalf("mark all read: updated=%d err=%v", updated, err)
 }
 if updated, err := a.ReadAllNotifications(ctx); err != nil || updated != 0 {
  t.Fatalf("mark all replay: updated=%d err=%v", updated, err)
 }
 history, err := a.NotificationHistory(ctx, firstCursor)
 if err != nil || len(history.Items) == 0 {
  t.Fatalf("history after bulk read: %+v err=%v", history, err)
 }
 for _, item := range history.Items {
  if item.ReadAt == nil {
   t.Fatalf("older notice remained unread: %s", item.ID)
  }
 }
 foreign, err := b.NotificationHistory(ctx, "")
 if err != nil || len(foreign.Items) != 1 {
  t.Fatalf("foreign notification history: %+v err=%v", foreign, err)
 }
 for _, item := range foreign.Items {
  if item.ReadAt != nil {
   t.Fatalf("foreign notice marked read: %s", item.ID)
  }
 }
 h:=httpapi.NewPostgresRouter(slog.New(slog.NewTextHandler(io.Discard,nil)),db)
 for _,spec:=range []struct{cursor string;status int}{{"?",400},{firstCursor,200}}{
  r:=httptest.NewRequest("GET","/api/v1/notifications?cursor="+spec.cursor,nil)
  r.Header.Set("Authorization","Bearer "+one.Token)
  w:=httptest.NewRecorder();h.ServeHTTP(w,r)
  if w.Code!=spec.status{t.Fatal("history route",w.Code,w.Body.String())}
  if spec.status==200&&(!strings.Contains(w.Body.String(),`"nextCursor"`)||w.Header().Get("Cache-Control")!="no-store"){t.Fatal("history response contract")}
 }
}
