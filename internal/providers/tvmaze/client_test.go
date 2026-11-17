package tvmaze

import (
 "context"
 "encoding/json"
 "errors"
 "io"
 "math"
 "net/http"
 "strings"
 "testing"
 "time"

 "github.com/khordestan/anime-intelligence/internal/modules/catalog/feed"
)

type transportFunc func(*http.Request)(*http.Response,error)
func(f transportFunc)RoundTrip(r *http.Request)(*http.Response,error){return f(r)}
func response(r *http.Request,status int,body string)*http.Response{
 return &http.Response{StatusCode:status,Header:make(http.Header),Body:io.NopCloser(strings.NewReader(body)),Request:r}
}
const workJSON=`{"id":101,"name":"Synthetic Dawn","type":"Animation","genres":["Action"],"status":"Ended","summary":"discard this synopsis","image":{"original":"https://example.test/image.jpg"}}`
const seasonsJSON=`[{"id":501,"number":2,"premiereDate":"2026-01-01","endDate":"2026-02-01","episodeOrder":12}]`
const episodesJSON=`[{"id":1,"season":2,"number":1,"type":"regular","airstamp":"2020-01-01T01:00:00+01:00"},
{"id":2,"season":2,"number":3,"type":"regular","airstamp":"2030-01-01T00:00:00Z"},
{"id":3,"season":2,"number":null,"type":"significant_special","airstamp":null},
{"id":4,"season":2,"number":2,"type":"insignificant_special","airstamp":"2026-01-01T00:00:00Z"},
{"id":5,"season":2,"number":4,"type":"regular","airstamp":null}]`
func fixture(t *testing.T,fn transportFunc)*Client{
 t.Helper()
 cooldownMu.Lock();nextRequest=time.Time{};cooldownMu.Unlock()
 clock:=time.Date(2026,1,2,0,0,0,0,time.UTC)
 c:=newClient(fn,true)
 c.now=func()time.Time{return clock}
 c.wait=func(ctx context.Context,d time.Duration)error{
  if ctx.Err()!=nil{return ctx.Err()}
  clock=clock.Add(d);return nil
 }
 return c
}
func bodyFor(path string)string{
 switch path{case "/shows/101":return workJSON;case "/shows/101/seasons":return seasonsJSON;case "/seasons/501/episodes":return episodesJSON}
 return "{}"
}
func TestReviewedSeasonAndScheduleNormalization(t *testing.T){
 var paths []string
 c:=fixture(t,func(r *http.Request)(*http.Response,error){
  paths=append(paths,r.URL.Path)
  if r.URL.Scheme!="https"||r.URL.Host!="api.tvmaze.com"||r.URL.RawQuery!=""{t.Fatal("unapproved endpoint",r.URL)}
  if !strings.Contains(r.Header.Get("User-Agent"),"AnimeIntelligence")||r.Header.Get("Accept")!="application/json"{t.Fatal("missing identification")}
  if _,ok:=r.Context().Deadline();!ok{t.Fatal("request timeout missing")}
  return response(r,200,bodyFor(r.URL.Path)),nil
 })
 s,err:=c.Fetch(context.Background(),[]string{"show-101-season-501"})
 if err!=nil{t.Fatal(err)}
 if strings.Join(paths,",")!="/shows/101,/shows/101/seasons,/seasons/501/episodes"{t.Fatal("unexpected requests",paths)}
 title:=s.Titles[0]
 if s.Provider!="tvmaze"||title.ExternalID!="show-101-season-501"||title.Title!="Synthetic Dawn — Season 2"||title.ReleaseYear!=2026||
 title.Status!="UNKNOWN"||title.MediaType!="TV"||title.EpisodeCount!=nil||title.Synopsis!=""||len(title.Genres)!=0||len(title.Relations)!=0{t.Fatal("unsupported inference",title)}
 if len(title.Episodes)!=3||title.Episodes[0].Number!=1||title.Episodes[1].Number!=3||title.Episodes[2].Number!=4||title.Episodes[2].ScheduledAt!=nil{t.Fatal("specials/gaps",title.Episodes)}
 if !title.Episodes[0].ScheduledAt.Equal(time.Date(2020,1,1,0,0,0,0,time.UTC)){t.Fatal("timestamp normalization")}
 for _,ep:=range title.Episodes{if ep.Track!="UNKNOWN"||ep.ReleasedAt!=nil{t.Fatal("availability inferred",ep)}}
 releases,err:=feed.Releases(nil,s,c.now())
 if err!=nil||len(releases)!=0{t.Fatal("historical alerts",releases,err)}
 s2:=s;s2.ObservedAt=s.ObservedAt.Add(time.Hour)
 releases,err=feed.Releases(&s,s2,s2.ObservedAt)
 if err!=nil||len(releases)!=0{t.Fatal("airdate became release confirmation",releases,err)}
}
func TestReviewAndInputBoundsPreventNetwork(t *testing.T){
 calls:=0
 c:=fixture(t,func(r *http.Request)(*http.Response,error){calls++;return response(r,200,"{}"),nil})
 c.reviewed=false
 if _,err:=c.Fetch(context.Background(),[]string{"show-101-season-501"});!errors.Is(err,ErrReview){t.Fatal(err)}
 c.reviewed=true
 for _,ids:=range [][]string{nil,{"Q1"},{"show-0-season-1"},{"show-01-season-1"},{"show-1-season-0"},{"show-1000000000-season-1"},{"https://example.test"},{"show-1-season-1","show-1-season-2"}}{
  if _,err:=c.Fetch(context.Background(),ids);err==nil{t.Fatal("invalid IDs accepted",ids)}
 }
 ctx,cancel:=context.WithCancel(context.Background());cancel()
 if _,err:=c.Fetch(ctx,[]string{"show-101-season-501"});err==nil{t.Fatal("cancelled fetch accepted")}
 if calls!=0{t.Fatal("network before validation",calls)}
}
func TestMalformedMembershipAndEpisodeBoundaries(t *testing.T){
 cases:=[]struct{name,path,body string}{
  {"wrong show","/shows/101",strings.Replace(workJSON,`"id":101`,`"id":102`,1)},
  {"not animation","/shows/101",strings.Replace(workJSON,`"Animation"`,`"Scripted"`,1)},
  {"missing season","/shows/101/seasons",`[{"id":502,"number":2}]`},
  {"ambiguous season","/shows/101/seasons",`[{"id":501,"number":2},{"id":501,"number":3}]`},
  {"invalid premiere","/shows/101/seasons",strings.Replace(seasonsJSON,"2026-01-01","2026-02-30",1)},
  {"foreign season episode","/seasons/501/episodes",`[{"id":1,"season":3,"number":1,"type":"regular"}]`},
  {"duplicate episode IDs","/seasons/501/episodes",`[{"id":1,"season":2,"number":1,"type":"regular"},{"id":1,"season":2,"number":2,"type":"regular"}]`},
  {"duplicate episode numbers","/seasons/501/episodes",`[{"id":1,"season":2,"number":1,"type":"regular"},{"id":2,"season":2,"number":1,"type":"regular"}]`},
  {"invalid airstamp","/seasons/501/episodes",`[{"id":1,"season":2,"number":1,"type":"regular","airstamp":"tomorrow"}]`},
  {"zero episode","/seasons/501/episodes",`[{"id":1,"season":2,"number":0,"type":"regular"}]`},
  {"null episodes","/seasons/501/episodes","null"},
  {"typed corruption","/shows/101",`{"id":"101","type":"Animation"}`},
  {"trailing document","/shows/101",workJSON+" {}"},
  {"oversized response","/shows/101",strings.Repeat("x",feed.MaxBytes+1)},
 }
 for _,tc:=range cases{t.Run(tc.name,func(t *testing.T){
  c:=fixture(t,func(r *http.Request)(*http.Response,error){body:=bodyFor(r.URL.Path);if r.URL.Path==tc.path{body=tc.body};return response(r,200,body),nil})
  if _,err:=c.Fetch(context.Background(),[]string{"show-101-season-501"});err==nil{t.Fatal("corrupt response accepted")}
 })}
}
func TestRateLimitsAreRetainedAndErrorsAreRedacted(t *testing.T){
 for _,status:=range []int{429,503}{t.Run(http.StatusText(status),func(t *testing.T){
  calls:=0
  c:=fixture(t,func(r *http.Request)(*http.Response,error){
   calls++;res:=response(r,status,"synthetic secret response")
   res.Header.Set("Retry-After","86400");return res,nil
  })
  _,err:=c.Fetch(context.Background(),[]string{"show-101-season-501"})
  var limited *RateLimitError
  if !errors.As(err,&limited)||limited.Wait!=24*time.Hour||limited.Until.Before(c.now().Add(24*time.Hour)){t.Fatal("rate limit",err)}
  other:=newClient(c.http.Transport,true);other.now=c.now
  var waited time.Duration
  other.wait=func(_ context.Context,d time.Duration)error{waited=d;return context.DeadlineExceeded}
  if _,err=other.Fetch(context.Background(),[]string{"show-101-season-501"});err==nil||calls!=1||waited!=24*time.Hour{t.Fatal("client restart bypassed cooldown",calls,waited,err)}
 })}
 if got:=retryAfter(strings.Repeat("9",100),time.Now());got!=time.Duration(math.MaxInt64){t.Fatal("overflow did not saturate",got)}
 now:=time.Now().UTC().Truncate(time.Second)
 if got:=retryAfter(now.Add(2*time.Hour).Format(http.TimeFormat),now);got!=2*time.Hour{t.Fatal("HTTP date",got)}
 for _,status:=range []int{301,302,404,500}{
  c:=fixture(t,func(r *http.Request)(*http.Response,error){res:=response(r,status,"synthetic secret response");res.Header.Set("Location","https://example.test/private");return res,nil})
  _,err:=c.Fetch(context.Background(),[]string{"show-101-season-501"})
  if !errors.Is(err,ErrProvider)||strings.Contains(err.Error(),"secret"){t.Fatal("unsafe response error",err)}
 }
 c:=fixture(t,func(r *http.Request)(*http.Response,error){return nil,errors.New("synthetic transport credential")})
 if _,err:=c.Fetch(context.Background(),[]string{"show-101-season-501"});!errors.Is(err,ErrProvider){t.Fatal("transport detail leaked",err)}
}
func TestEmptyEpisodeListAndUnknownDatesRemainUnknown(t *testing.T){
 c:=fixture(t,func(r *http.Request)(*http.Response,error){
  body:=bodyFor(r.URL.Path)
  if strings.HasSuffix(r.URL.Path,"/episodes"){body="[]"}
  if strings.HasSuffix(r.URL.Path,"/seasons"){body=`[{"id":501,"number":2,"premiereDate":null}]`}
  return response(r,200,body),nil
 })
 s,err:=c.Fetch(context.Background(),[]string{"show-101-season-501"})
 if err!=nil||s.Titles[0].ReleaseYear!=0||s.Titles[0].EpisodeCount!=nil||len(s.Titles[0].Episodes)!=0{t.Fatal("unknown facts invented",s,err)}
 data,_:=json.Marshal(s)
 if strings.Contains(string(data),"releasedAt"){t.Fatal("invented release",string(data))}
}
