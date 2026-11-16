package postgres_test

import (
 "context"
 "encoding/json"
 "errors"
 "io"
 "log/slog"
 "net/http/httptest"
 "net/url"
 "strings"
 "sync"
 "testing"

 "github.com/jackc/pgx/v5"
 "github.com/khordestan/anime-intelligence/internal/httpapi"
 "github.com/khordestan/anime-intelligence/internal/platform/postgres"
 "github.com/khordestan/anime-intelligence/internal/platform/store"
)
func TestRewatchSnapshotsIsolationConcurrencyAndRollback(t *testing.T){
 db,owner,_,dsn:=testDB(t);ctx:=context.Background()
 one,err:=db.Register(ctx,"rewatch-one@example.test","synthetic-rewatch-password");if err!=nil{t.Fatal(err)}
 two,err:=db.Register(ctx,"rewatch-two@example.test","synthetic-rewatch-password");if err!=nil{t.Fatal(err)}
 if _,_,err=db.Repository(one.Token).WatchHistory(ctx,"10000000-0000-4000-8000-000000000001",0);!errors.Is(err,store.ErrEmailUnverified){t.Fatal("unverified history",err)}
 verifyAccount(t,db,one.Token);verifyAccount(t,db,two.Token)
 second,err:=db.Login(ctx,one.User.Email,"synthetic-rewatch-password");if err!=nil{t.Fatal(err)}
 const anime="10000000-0000-4000-8000-000000000001"
 a,b:=db.Repository(one.Token),db.Repository(two.Token)
 entry,err:=a.Add(ctx,anime,"WATCHING",3);if err!=nil{t.Fatal(err)}
 if _,_,err=a.StartRewatch(ctx,entry.ID,1);!errors.Is(err,postgres.ErrRewatchState){t.Fatal("incomplete pass reset",err)}
 if _,_,err=a.SetEpisodeProgress(ctx,entry.ID,[]int32{1,4,7});err!=nil{t.Fatal(err)}
 completed:="COMPLETED"
 if _,_,err=a.Update(ctx,entry.ID,&completed,nil);err!=nil{t.Fatal(err)}
 if _,_,err=a.Rate(ctx,anime,5);err!=nil{t.Fatal(err)}
 var wg sync.WaitGroup;results:=make(chan error,2)
 for _,token:=range []string{one.Token,second.Token}{wg.Add(1);go func(token string){defer wg.Done();_,_,err:=db.Repository(token).StartRewatch(ctx,entry.ID,1);results<-err}(token)}
 wg.Wait();close(results);success,conflicts:=0,0
 for err:=range results{if err==nil{success++}else if errors.Is(err,postgres.ErrRewatchState){conflicts++}else{t.Fatal("concurrent start",err)}}
 if success!=1||conflicts!=1{t.Fatal("retry archived twice",success,conflicts)}
 history,ok,err:=a.WatchHistory(ctx,entry.ID,0)
 if err!=nil||!ok||history.CurrentCycle!=2||len(history.Items)!=1{t.Fatal("archive missing",history,err)}
 old:=history.Items[0]
 if old.Cycle!=1||old.Mode!="EXACT"||old.Count!=3||len(old.Numbers)!=3||old.Numbers[1]!=4||old.Total==nil||*old.Total!=12||old.Rating==nil||*old.Rating!=5{t.Fatal("exact snapshot lost",old)}
 rows,err:=a.Library(ctx)
 if err!=nil||rows[0].WatchCycle!=2||rows[0].WatchStatus!="WATCHING"||rows[0].EpisodesWatched!=0||rows[0].ExactProgress||rows[0].Rating!=5{t.Fatal("rewatch reset semantics",rows,err)}
 if _,ok,err=b.WatchHistory(ctx,entry.ID,0);err!=nil||ok{t.Fatal("foreign history read",err)}
 if _,ok,err=b.StartRewatch(ctx,entry.ID,2);err!=nil||ok{t.Fatal("foreign start",err)}
 if _,_,err=a.Rate(ctx,anime,1);err!=nil{t.Fatal(err)}
 count:=5
 if _,_,err=a.Update(ctx,entry.ID,&completed,&count);err!=nil{t.Fatal(err)}
 // Fail after archive insertion to prove the entire pass transition rolls back.
 if _,err=owner.Exec(ctx,`CREATE FUNCTION synthetic_rewatch_fail() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'synthetic failure'; END $$;
 CREATE TRIGGER synthetic_rewatch_fail BEFORE UPDATE ON watch_progress FOR EACH ROW EXECUTE FUNCTION synthetic_rewatch_fail()`);err!=nil{t.Fatal(err)}
 if _,_,err=a.StartRewatch(ctx,entry.ID,2);err==nil{t.Fatal("injected progress failure accepted")}
 if _,err=owner.Exec(ctx,`DROP TRIGGER synthetic_rewatch_fail ON watch_progress; DROP FUNCTION synthetic_rewatch_fail()`);err!=nil{t.Fatal(err)}
 history,_,err=a.WatchHistory(ctx,entry.ID,0)
 if err!=nil||history.CurrentCycle!=2||len(history.Items)!=1||*history.Items[0].Rating!=5{t.Fatal("rollback or snapshot mutation",history,err)}
 if _,_,err=a.StartRewatch(ctx,entry.ID,2);err!=nil{t.Fatal(err)}
 // A real unscoped runtime connection must see no archived personal rows.
 raw,err:=pgx.Connect(ctx,dsn);if err!=nil{t.Fatal(err)};defer raw.Close(ctx)
 var visible int
 if err=raw.QueryRow(ctx,`SELECT count(*) FROM watch_cycles`).Scan(&visible);err!=nil||visible!=0{t.Fatal("unscoped history visible",visible,err)}
 u,_:=url.Parse(dsn); var canUpdate bool
 if err=owner.QueryRow(ctx,`SELECT has_table_privilege($1,'watch_cycles','UPDATE')`,u.User.Username()).Scan(&canUpdate);err!=nil||canUpdate{t.Fatal("archive update privilege",err)}
 exported,err:=db.ExportAccount(ctx,one.Token);if err!=nil{t.Fatal(err)}
 var snapshots []map[string]any
 if err=json.Unmarshal(exported.Data["watchCycles"],&snapshots);err!=nil||len(snapshots)!=2{t.Fatal("export archives",err)}
 if ok,err:=a.Remove(ctx,entry.ID);err!=nil||!ok{t.Fatal(err)}
 if err=owner.QueryRow(ctx,`SELECT count(*) FROM watch_cycles WHERE user_id=$1`,one.User.ID).Scan(&visible);err!=nil||visible!=0{t.Fatal("removal retained archived private data",err)}
}
func TestRewatchRoutesRequireVerifiedOwnerAndConditionalCycle(t *testing.T){
 db,_,_,_:=testDB(t);ctx:=context.Background()
 s,err:=db.Register(ctx,"rewatch-http@example.test","synthetic-rewatch-password");if err!=nil{t.Fatal(err)}
 h:=httpapi.NewPostgresRouter(slog.New(slog.NewTextHandler(io.Discard,nil)),db)
 call:=func(method,path,token,body string)*httptest.ResponseRecorder{r:=httptest.NewRequest(method,path,strings.NewReader(body));if token!=""{r.Header.Set("Authorization","Bearer "+token)};w:=httptest.NewRecorder();h.ServeHTTP(w,r);return w}
 const fake="10000000-0000-4000-8000-000000000001"
 for _,method:=range []string{"GET","POST"}{
  if w:=call(method,"/api/v1/tracker/"+fake+"/rewatches","",`{"expectedCycle":1}`);w.Code!=401{t.Fatal("unprotected route",w.Code)}
  if w:=call(method,"/api/v1/tracker/"+fake+"/rewatches",s.Token,`{"expectedCycle":1}`);w.Code!=403{t.Fatal("unverified route",w.Code)}
 }
 verifyAccount(t,db,s.Token)
 entry,err:=db.Repository(s.Token).Add(ctx,fake,"COMPLETED",12);if err!=nil{t.Fatal(err)}
 path:="/api/v1/tracker/"+entry.ID+"/rewatches"
 for _,body:=range []string{`{}`,`{"expectedCycle":0}`,`{"expectedCycle":null}`,`{"expectedCycle":1,"userId":"spoofed"}`}{
  if w:=call("POST",path,s.Token,body);w.Code!=400{t.Fatal("invalid conditional request",w.Code,w.Body.String())}
 }
 if w:=call("POST",path,s.Token,`{"expectedCycle":1}`);w.Code!=200||!strings.Contains(w.Body.String(),`"watchCycle":2`){t.Fatal("start response",w.Code,w.Body.String())}
 if w:=call("POST",path,s.Token,`{"expectedCycle":1}`);w.Code!=409{t.Fatal("replayed start",w.Code)}
 for _,query:=range []string{"?before=0","?before=no","?before=1&before=2","?before=1001"}{
  if w:=call("GET",path+query,s.Token,"");w.Code!=400{t.Fatal("invalid history boundary",w.Code)}
 }
 for _,body:=range []string{`{"episodesWatched":9,"expectedCycle":1}`,`{"watchStatus":"COMPLETED","expectedCycle":1}`}{
  if w:=call("PATCH","/api/v1/tracker/"+entry.ID,s.Token,body);w.Code!=409{t.Fatal("stale pass changed tracker",w.Code,w.Body.String())}
 }
 if w:=call("PUT","/api/v1/tracker/"+entry.ID+"/episodes",s.Token,`{"numbers":[1,4],"expectedCycle":1}`);w.Code!=409{t.Fatal("stale exact edit changed rewatch",w.Code,w.Body.String())}
 if w:=call("GET","/api/v1/tracker/"+entry.ID+"/episodes",s.Token,"");w.Code!=200||!strings.Contains(w.Body.String(),`"count":0`)||!strings.Contains(w.Body.String(),`"watchCycle":2`){t.Fatal("stale edit mutated new pass",w.Body.String())}
 if w:=call("PATCH","/api/v1/tracker/"+entry.ID,s.Token,`{"episodesWatched":2,"expectedCycle":2}`);w.Code!=200{t.Fatal("current conditional update rejected",w.Code,w.Body.String())}
 if w:=call("PUT","/api/v1/tracker/"+entry.ID+"/episodes",s.Token,`{"numbers":[1,4],"expectedCycle":2}`);w.Code!=200{t.Fatal("current exact update rejected",w.Code,w.Body.String())}
 if w:=call("GET",path,s.Token,"");w.Code!=200||!strings.Contains(w.Body.String(),`"currentCycle":2`)||w.Header().Get("Cache-Control")!="no-store"{t.Fatal("history contract",w.Code,w.Body.String())}
}

func TestRewatchHistoryPaginationAndLimits(t *testing.T){
 db,owner,_,_:=testDB(t);ctx:=context.Background()
 s,err:=db.Register(ctx,"rewatch-pages@example.test","synthetic-rewatch-password");if err!=nil{t.Fatal(err)}
 verifyAccount(t,db,s.Token)
 const anime="10000000-0000-4000-8000-000000000001"
 entry,err:=db.Repository(s.Token).Add(ctx,anime,"COMPLETED",12);if err!=nil{t.Fatal(err)}
 if _,err=owner.Exec(ctx,`INSERT INTO watch_cycles(user_id,anime_id,cycle_number,episodes_watched,total_episodes) SELECT $1,$2,n,12,12 FROM generate_series(1,105) n`,s.User.ID,anime);err!=nil{t.Fatal(err)}
 if _,err=owner.Exec(ctx,`UPDATE user_anime SET watch_cycle=106 WHERE user_id=$1 AND anime_id=$2`,s.User.ID,anime);err!=nil{t.Fatal(err)}
 first,ok,err:=db.Repository(s.Token).WatchHistory(ctx,entry.ID,0)
 if err!=nil||!ok||len(first.Items)!=100||first.Items[0].Cycle!=105||first.NextBefore!=6{t.Fatal("first bounded history page",first,err)}
 second,ok,err:=db.Repository(s.Token).WatchHistory(ctx,entry.ID,first.NextBefore)
 if err!=nil||!ok||len(second.Items)!=5||second.Items[0].Cycle!=5||second.NextBefore!=0{t.Fatal("older cycle history",second,err)}
 if _,err=owner.Exec(ctx,`UPDATE user_anime SET watch_cycle=1000 WHERE user_id=$1 AND anime_id=$2`,s.User.ID,anime);err!=nil{t.Fatal(err)}
 if _,_,err=db.Repository(s.Token).StartRewatch(ctx,entry.ID,1000);!errors.Is(err,postgres.ErrRewatchState){t.Fatal("unbounded cycle allocation",err)}
}
