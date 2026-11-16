package postgres_test

import (
 "context"
 "errors"
 "io"
 "log/slog"
 "net/http/httptest"
 "testing"
 "time"

 "github.com/khordestan/anime-intelligence/internal/httpapi"
 "github.com/khordestan/anime-intelligence/internal/modules/catalog/feed"
 "github.com/khordestan/anime-intelligence/internal/platform/catalogsync"
 "github.com/khordestan/anime-intelligence/internal/platform/store"
)
func TestFollowCalendarCurrentRevisionsTracksFranchiseAndBounds(t *testing.T){
 db,owner,_,_:=testDB(t);ctx:=context.Background()
 a,err:=db.Register(ctx,"calendar-one@example.test","synthetic-calendar-password");if err!=nil{t.Fatal(err)}
 b,err:=db.Register(ctx,"calendar-two@example.test","synthetic-calendar-password");if err!=nil{t.Fatal(err)}
 now:=time.Now().UTC().Truncate(time.Microsecond)
 from:=time.Date(now.Year(),now.Month(),now.Day()+1,0,0,0,0,time.UTC)
 if _,err=db.Repository(a.Token).ReleaseCalendar(ctx,from,7);!errors.Is(err,store.ErrEmailUnverified){t.Fatal("unverified calendar",err)}
 verifyAccount(t,db,a.Token);verifyAccount(t,db,b.Token)
 if _,err=owner.Exec(ctx,`INSERT INTO data_sources(provider_key,display_name,enabled,policy_url) VALUES('calendar-fixture','Synthetic calendar',true,'https://example.test/policy')`);err!=nil{t.Fatal(err)}
 slot:=func(hours int)*time.Time{v:=from.Add(time.Duration(hours)*time.Hour);return &v}
 released:=now.Add(-3*time.Hour)
 snapshot:=feed.Snapshot{Version:1,Provider:"calendar-fixture",ObservedAt:now.Add(-2*time.Hour),Titles:[]feed.Title{
 {ExternalID:"one",Title:"Synthetic Calendar One",MediaType:"TV",Status:"RELEASING",Relations:[]feed.Relation{{ExternalID:"two",Type:"SEQUEL"}},Episodes:[]feed.Episode{
  {Number:1,Track:"SUB",ScheduledAt:&from},{Number:2,Track:"SUB",ScheduledAt:slot(7*24)},
  {Number:3,Track:"DUB",ScheduledAt:slot(3)},{Number:4,Track:"UNKNOWN",ScheduledAt:slot(4)},
  {Number:5,Track:"SUB",ScheduledAt:slot(5),ReleasedAt:&released},{Number:6,Track:"SUB",ScheduledAt:slot(2)},
 }},
 {ExternalID:"two",Title:"Synthetic Calendar Two",MediaType:"TV",Status:"ANNOUNCED",Relations:[]feed.Relation{{ExternalID:"three",Type:"SEQUEL"}},Episodes:[]feed.Episode{{Number:1,Track:"SUB",ScheduledAt:slot(1)}}},
 {ExternalID:"three",Title:"Synthetic Calendar Three",MediaType:"TV",Status:"ANNOUNCED",Episodes:[]feed.Episode{{Number:1,Track:"SUB",ScheduledAt:slot(2)}}},
 }}
 if _,err=catalogsync.Import(ctx,owner,snapshot,now);err!=nil{t.Fatal(err)}
 var one,two string
 if err=owner.QueryRow(ctx,`SELECT anime_id::text FROM anime_external_ids WHERE provider='calendar-fixture' AND external_id='one'`).Scan(&one);err!=nil{t.Fatal(err)}
 if err=owner.QueryRow(ctx,`SELECT anime_id::text FROM anime_external_ids WHERE provider='calendar-fixture' AND external_id='two'`).Scan(&two);err!=nil{t.Fatal(err)}
 // Move a previously in-window schedule out of the window. An implementation
 // filtering before latest-revision selection would incorrectly resurrect it.
 snapshot.ObservedAt=now.Add(-time.Hour);snapshot.Titles=snapshot.Titles[:1];snapshot.Titles[0].Episodes=[]feed.Episode{{Number:6,Track:"SUB",ScheduledAt:slot(8*24)}}
 if _,err=catalogsync.Import(ctx,owner,snapshot,now);err!=nil{t.Fatal(err)}
 first,second:=db.Repository(a.Token),db.Repository(b.Token)
 if _,err=first.FollowScope(ctx,one,"SUB","FRANCHISE");err!=nil{t.Fatal(err)}
 if _,err=first.Follow(ctx,two,"SUB");err!=nil{t.Fatal(err)}
 if _,err=second.Follow(ctx,one,"DUB");err!=nil{t.Fatal(err)}
 calendar,err:=first.ReleaseCalendar(ctx,from,7)
 if err!=nil||len(calendar.Items)!=3||calendar.Truncated||calendar.Items[0].AnimeID!=one||calendar.Items[0].Number!=1||calendar.Items[1].AnimeID!=two||calendar.Items[2].Title!="Synthetic Calendar Three"||!calendar.From.Equal(from)||!calendar.To.Equal(from.AddDate(0,0,7)){t.Fatal("calendar revision/window/franchise/dedup",calendar,err)}
 for _,item:=range calendar.Items{if item.Track!="SUB"||item.Provider!="calendar-fixture"||item.VerifiedAt==nil{t.Fatal("calendar provenance",item)}}
 other,err:=second.ReleaseCalendar(ctx,from,7)
 if err!=nil||len(other.Items)!=1||other.Items[0].Track!="DUB"||other.Items[0].Number!=3{t.Fatal("foreign follow/track applied",other,err)}
 if _,err=first.FollowScope(ctx,one,"ALL","FRANCHISE");err!=nil{t.Fatal(err)}
 all,err:=first.ReleaseCalendar(ctx,from,7)
 if err!=nil||len(all.Items)!=5{t.Fatal("ALL tracks or released schedule filtering",all,err)}
 if _,err=owner.Exec(ctx,`INSERT INTO airing_schedule(anime_id,episode_number,release_track,scheduled_at,schedule_status,source_id,revision)
 SELECT $1,n,'SUB',$2,'SCHEDULED',s.id,1 FROM generate_series(10,119) n CROSS JOIN data_sources s WHERE s.provider_key='calendar-fixture'`,one,from.Add(6*time.Hour));err!=nil{t.Fatal(err)}
 capped,err:=first.ReleaseCalendar(ctx,from,7)
 if err!=nil||len(capped.Items)!=100||!capped.Truncated||capped.Limit!=100{t.Fatal("bounded calendar",capped,err)}
 for _,window:=range []struct{start time.Time;days int}{{from,0},{from,32},{from.Add(time.Hour),7},{time.Time{},7}}{
  if _,err=first.ReleaseCalendar(ctx,window.start,window.days);err==nil{t.Fatal("invalid calendar window")}
 }
}
func TestCalendarHTTPVerifiedOwnerAndWindowValidation(t *testing.T){
 db,_,_,_:=testDB(t);ctx:=context.Background()
 s,err:=db.Register(ctx,"calendar-http@example.test","synthetic-calendar-password");if err!=nil{t.Fatal(err)}
 h:=httpapi.NewPostgresRouter(slog.New(slog.NewTextHandler(io.Discard,nil)),db)
 call:=func(path,token string)*httptest.ResponseRecorder{r:=httptest.NewRequest("GET",path,nil);if token!=""{r.Header.Set("Authorization","Bearer "+token)};w:=httptest.NewRecorder();h.ServeHTTP(w,r);return w}
 if w:=call("/api/v1/calendar","");w.Code!=401{t.Fatal("unprotected calendar",w.Code)}
 if w:=call("/api/v1/calendar",s.Token);w.Code!=403{t.Fatal("unverified calendar",w.Code)}
 verifyAccount(t,db,s.Token)
 for _,query:=range []string{"?days=0","?days=32","?days=x","?from=bad","?from=2026-02-30","?days=1&days=2","?from=2026-01-01&from=2026-01-02"}{
  if w:=call("/api/v1/calendar"+query,s.Token);w.Code!=400{t.Fatal("invalid calendar query",query,w.Code)}
 }
 if w:=call("/api/v1/calendar",s.Token);w.Code!=200||w.Header().Get("Cache-Control")!="no-store"{t.Fatal("calendar response",w.Code,w.Body.String())}
}
