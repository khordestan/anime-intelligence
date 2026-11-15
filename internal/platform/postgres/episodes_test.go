package postgres_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/khordestan/anime-intelligence/internal/httpapi"
)
func TestExactEpisodeProgressAndAggregateBoundary(t *testing.T) {
	db,owner,_,_:=testDB(t)
	ctx:=context.Background()
	one,err:=db.Register(ctx,"episodes-one@example.test","synthetic-episodes-password")
	if err!=nil {t.Fatal(err)}
	two,err:=db.Register(ctx,"episodes-two@example.test","synthetic-episodes-password")
	if err!=nil {t.Fatal(err)}
	verifyAccount(t,db,one.Token);verifyAccount(t,db,two.Token)
	a,b:=db.Repository(one.Token),db.Repository(two.Token)
	const anime="10000000-0000-4000-8000-000000000001"
	entry,err:=a.Add(ctx,anime,"WATCHING",3)
	if err!=nil {t.Fatal(err)}
	p,ok,err:=a.EpisodeProgress(ctx,entry.ID)
	if err!=nil || !ok || p.Mode!="COUNT" || p.Numbers!=nil || p.Count!=3 {t.Fatal("legacy aggregate inferred exact episodes",p,err)}
	p,ok,err=a.SetEpisodeProgress(ctx,entry.ID,[]int32{4,1,2})
	if err!=nil || !ok || p.Mode!="EXACT" || p.Count!=3 || !slices.Equal(p.Numbers,[]int32{1,2,4}) {t.Fatal("exact replacement",p,err)}
	library,err:=a.Library(ctx)
	if err!=nil || len(library)!=1 || !library[0].ExactProgress || library[0].EpisodesWatched!=3 {t.Fatal("library exact mode",library,err)}
	if _,ok,err:=b.EpisodeProgress(ctx,entry.ID);err!=nil || ok {t.Fatal("foreign progress leaked",err)}
	if _,ok,err:=b.SetEpisodeProgress(ctx,entry.ID,[]int32{1});err!=nil || ok {t.Fatal("foreign progress changed",err)}
	for _,numbers:=range [][]int32{nil,{0},{-1},{1,1},{13},{100001}} {
		if _,_,err:=a.SetEpisodeProgress(ctx,entry.ID,numbers);err==nil {t.Fatal("invalid episode set accepted",numbers)}
	}
	status:="ON_HOLD"
	if _,ok,err:=a.Update(ctx,entry.ID,&status,nil);err!=nil || !ok {t.Fatal(err)}
	p,_,err=a.EpisodeProgress(ctx,entry.ID)
	if err!=nil || p.Mode!="EXACT" || !slices.Equal(p.Numbers,[]int32{1,2,4}) {t.Fatal("status update erased exact episodes",err)}
	count:=5
	if _,ok,err:=a.Update(ctx,entry.ID,nil,&count);err!=nil || !ok {t.Fatal(err)}
	p,_,err=a.EpisodeProgress(ctx,entry.ID)
	if err!=nil || p.Mode!="COUNT" || p.Count!=5 || p.Numbers!=nil {t.Fatal("explicit count update did not reset exact mode",p,err)}
	if _,_,err=a.SetEpisodeProgress(ctx,entry.ID,[]int32{});err!=nil {t.Fatal(err)}
	p,_,err=a.EpisodeProgress(ctx,entry.ID)
	if err!=nil || p.Mode!="EXACT" || p.Numbers==nil || len(p.Numbers)!=0 || p.Count!=0 {t.Fatal("empty exact set",p,err)}
	if _,_,err=a.SetEpisodeProgress(ctx,entry.ID,[]int32{1,3});err!=nil {t.Fatal(err)}
	exported,err:=db.ExportAccount(ctx,one.Token)
	if err!=nil {t.Fatal(err)}
	var progress []struct{Numbers []int32 `json:"watched_episode_numbers"`}
	if json.Unmarshal(exported.Data["progress"],&progress)!=nil || len(progress)!=1 || !slices.Equal(progress[0].Numbers,[]int32{1,3}) {t.Fatal("exact episodes absent from export")}
	for _,sql:=range []string{
		`UPDATE watch_progress SET watched_episode_numbers=ARRAY[1,1],episodes_watched=2 WHERE user_id=$1`,
		`UPDATE watch_progress SET watched_episode_numbers=ARRAY[1,2],episodes_watched=3 WHERE user_id=$1`,
		`UPDATE watch_progress SET watched_episode_numbers=ARRAY[0],episodes_watched=1 WHERE user_id=$1`,
	} {if _,err=owner.Exec(ctx,sql,one.User.ID);err==nil {t.Fatal("database accepted malformed exact progress")}}
	if ok,err:=a.Remove(ctx,entry.ID);err!=nil || !ok {t.Fatal(err)}
	if _,ok,err:=a.EpisodeProgress(ctx,entry.ID);err!=nil || ok {t.Fatal("removed progress survived",err)}
}
func TestExactEpisodeHTTPAuthorization(t *testing.T) {
	db,_,_,_:=testDB(t)
	ctx:=context.Background()
	s,err:=db.Register(ctx,"episodes-http@example.test","synthetic-episodes-password")
	if err!=nil {t.Fatal(err)}
	h:=httpapi.NewPostgresRouter(slog.New(slog.NewTextHandler(io.Discard,nil)),db)
	call:=func(method,path,token,body string)*httptest.ResponseRecorder{
		r:=httptest.NewRequest(method,path,strings.NewReader(body))
		if token!="" {r.Header.Set("Authorization","Bearer "+token)}
		w:=httptest.NewRecorder();h.ServeHTTP(w,r);return w
	}
	const fake="/api/v1/tracker/20000000-0000-4000-8000-000000000001/episodes"
	for _,method:=range []string{"GET","PUT"} {
		if w:=call(method,fake,"",`{"numbers":[]}`);w.Code!=401 {t.Fatal("unprotected exact progress",w.Code)}
		if w:=call(method,fake,s.Token,`{"numbers":[]}`);w.Code!=403 {t.Fatal("unverified exact progress",w.Code)}
	}
	verifyAccount(t,db,s.Token)
	entry,err:=db.Repository(s.Token).Add(ctx,"10000000-0000-4000-8000-000000000001","WATCHING",0)
	if err!=nil {t.Fatal(err)}
	path:="/api/v1/tracker/"+entry.ID+"/episodes"
	if w:=call("PUT",path,s.Token,`{"numbers":[3,1]}`);w.Code!=200 || !strings.Contains(w.Body.String(),`"numbers":[1,3]`) {t.Fatal("exact response",w.Code,w.Body.String())}
	for _,body:=range []string{`{"numbers":[1],"userId":"spoofed"}`,`{"numbers":null}`,`{}`,`{"numbers":[1]} {}`,`{"numbers":[1,1]}`} {
		if w:=call("PUT",path,s.Token,body);w.Code!=400 {t.Fatal("invalid exact body",body,w.Code)}
	}
	if w:=call("GET",fake,s.Token,"");w.Code!=404 {t.Fatal("missing progress response",w.Code)}
}
