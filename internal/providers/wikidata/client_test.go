package wikidata

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/khordestan/anime-intelligence/internal/modules/catalog/feed"
)
type transportFunc func(*http.Request)(*http.Response,error)
func(f transportFunc)RoundTrip(r *http.Request)(*http.Response,error){return f(r)}
func response(r *http.Request,status int,body string)*http.Response{return &http.Response{StatusCode:status,Header:make(http.Header),Body:io.NopCloser(strings.NewReader(body)),Request:r}}
func property(rank string,value any) claim {
	raw,_:=json.Marshal(value)
	var c claim
	c.Rank=rank;c.Snak.Type="value";c.Snak.Value.Value=raw
	return c
}
func item(id,title string) entity {
	return entity{ID:id,Type:"item",Labels:map[string]text{"en":{Value:title},"ja":{Value:"Synthetic Native"}},Aliases:map[string][]text{"en":{{Value:"Synthetic Alias"},{Value:"synthetic alias"}}},Claims:map[string][]claim{
		"P31":{property("normal",map[string]string{"id":"Q63952888"})},
		"P136":{property("normal",map[string]string{"id":"Q9001"})},
		"P1113":{property("normal",map[string]string{"amount":"+12","unit":"1"})},
		"P577":{property("normal",map[string]any{"time":"+2026-01-00T00:00:00Z","precision":10})},
	}}
}
func envelope(entities map[string]entity) string {b,_:=json.Marshal(map[string]any{"entities":entities});return string(b)}
func fixtureClient(fn transportFunc)*Client{
	gate.Lock();nextRequest=time.Time{};gate.Unlock()
	c:=newClient(fn)
	clock:=time.Date(2026,1,2,0,0,0,0,time.UTC)
	c.now=func()time.Time{return clock}
	c.wait=func(ctx context.Context,d time.Duration)error{if ctx.Err()!=nil{return ctx.Err()};clock=clock.Add(d);return nil}
	return c
}
func TestFetchStructuredFactsAndRequestPolicy(t *testing.T) {
	calls:=0
	c:=fixtureClient(func(r *http.Request)(*http.Response,error){
		calls++
		if r.URL.Scheme!="https" || r.URL.Host!="www.wikidata.org" || r.URL.Path!="/w/api.php" || r.Method!="GET" || r.Header.Get("Authorization")!="" || !strings.Contains(r.Header.Get("User-Agent"),"github.com/khordestan/anime-intelligence") || r.URL.Query().Get("maxlag")!="5" {t.Fatal("request policy",r.URL)}
		if calls==1 {return response(r,200,envelope(map[string]entity{"Q1":item("Q1","Synthetic Wiki Sky")})),nil}
		return response(r,200,envelope(map[string]entity{"Q9001":{ID:"Q9001",Type:"item",Labels:map[string]text{"en":{Value:"Synthetic Adventure"}}}})),nil
	})
	s,err:=c.Fetch(context.Background(),[]string{"Q1"})
	if err!=nil || calls!=2 || s.Provider!="wikidata" || len(s.Titles)!=1 {t.Fatal("fetch",s,err)}
	a:=s.Titles[0]
	if a.Title!="Synthetic Wiki Sky" || a.MediaType!="TV" || a.Status!="UNKNOWN" || a.ReleaseYear!=2026 || a.EpisodeCount==nil || *a.EpisodeCount!=12 || len(a.Genres)!=1 || len(a.Synonyms)!=1 || len(a.Episodes)!=0 || a.Synopsis!="" || len(a.Studios)!=0 {t.Fatal("facts or unsupported inference",a)}
}
func TestValidationAndUnsupportedItems(t *testing.T) {
	calls:=0
	c:=fixtureClient(func(r *http.Request)(*http.Response,error){calls++;e:=item("Q1","Synthetic Not Anime");e.Claims["P31"]=nil;return response(r,200,envelope(map[string]entity{"Q1":e})),nil})
	for _,ids:=range [][]string{nil,{"Q0"},{"Q1","Q1"},{"Q1|Q2"},{"https://example.test"},make([]string,21)} {
		if _,err:=c.Fetch(context.Background(),ids);err!=feed.ErrInvalid {t.Fatal("invalid input accepted",ids,err)}
	}
	if calls!=0 {t.Fatal("invalid input caused network activity")}
	if _,err:=c.Fetch(context.Background(),[]string{"Q1"});!errors.Is(err,ErrUnsupported) {t.Fatal("non-anime entity accepted",err)}
}
func TestProviderFailuresNeverForwardBodiesOrFollowRedirects(t *testing.T) {
	for _,tc:=range []struct{status int;body string}{{200,`{"error":{"code":"maxlag","info":"synthetic-secret"}}`},{200,`{"entities":{}}`},{200,"<html>synthetic-secret</html>"},{503,"synthetic-secret"},{302,"synthetic-secret"},{200,strings.Repeat(" ",feed.MaxBytes+1)}} {
		calls:=0
		c:=fixtureClient(func(r *http.Request)(*http.Response,error){calls++;res:=response(r,tc.status,tc.body);res.Header.Set("Location","https://example.test/secret");return res,nil})
		_,err:=c.Fetch(context.Background(),[]string{"Q1"})
		if err==nil || strings.Contains(err.Error(),"synthetic-secret") || calls!=1 {t.Fatal("unsafe failure/redirect behavior",err,calls)}
	}
	c:=fixtureClient(func(r *http.Request)(*http.Response,error){res:=response(r,429,"synthetic-secret");res.Header.Set("Retry-After","6");return res,nil})
	_,err:=c.Fetch(context.Background(),[]string{"Q1"})
	var limited *RateLimitError
	if !errors.As(err,&limited) || limited.Wait!=6*time.Second || !errors.Is(err,ErrRateLimited) {t.Fatal("rate-limit advice",err)}
	ctx,cancel:=context.WithCancel(context.Background());cancel()
	if _,err=c.Fetch(ctx,[]string{"Q1"});err!=ErrProvider {t.Fatal("cancelled fetch",err)}
}
func TestRankAmbiguityAndUnknownFacts(t *testing.T) {
	e:=item("Q1","Synthetic Facts")
	e.Claims["P1113"]=[]claim{property("normal",map[string]string{"amount":"+12","unit":"1"}),property("normal",map[string]string{"amount":"+13","unit":"1"})}
	if episodeCount(e)!=nil {t.Fatal("conflicting episode totals guessed")}
	e.Claims["P1113"]=append(e.Claims["P1113"],property("preferred",map[string]string{"amount":"+24","unit":"1"}))
	if got:=episodeCount(e);got==nil || *got!=24 {t.Fatal("preferred rank not honored")}
	e.Claims["P1113"]=[]claim{property("deprecated",map[string]string{"amount":"+99","unit":"1"})}
	if episodeCount(e)!=nil {t.Fatal("deprecated episode count used")}
	e.Claims["P577"]=[]claim{property("normal",map[string]any{"time":"+1900-00-00T00:00:00Z","precision":8})}
	if releaseYear(e)!=0 {t.Fatal("century precision became an exact year")}
}

func TestRetryAfterPreservesLongAndOverflowingCooldowns(t *testing.T){
	now:=time.Date(2026,1,1,0,0,0,0,time.UTC)
	if got:=retryAfter("86400",now);got!=24*time.Hour{t.Fatal("one-day cooldown shortened",got)}
	if got:=retryAfter(now.Add(48*time.Hour).Format(http.TimeFormat),now);got!=48*time.Hour{t.Fatal("HTTP-date cooldown shortened",got)}
	if got:=retryAfter("99999999999999999999999999999",now);got<365*24*time.Hour{t.Fatal("overflow became short retry",got)}
	if got:=retryAfter("invalid",now);got!=time.Minute{t.Fatal("invalid-header fallback",got)}
}

func TestExplicitAnimeWorkMediaTypes(t *testing.T){
 for _,fixture:=range []struct{classes []string;want string}{
  {[]string{"Q63952888"},"TV"},{[]string{"Q100269041"},"TV"},
  {[]string{"Q63952888","Q100269041"},"TV"},{[]string{"Q20650540"},"MOVIE"},
  {[]string{"Q220898"},"OVA"},{[]string{"Q1047299"},"ONA"},
  {[]string{"Q20650540","Q220898"},"OVA"},{[]string{"Q1047299","Q20650540"},"ONA"},
 }{
  calls:=0
  client:=fixtureClient(func(r *http.Request)(*http.Response,error){
   calls++
   e:=item("Q1","Synthetic Work")
   e.Claims["P31"]=nil;e.Claims["P136"]=nil;e.Claims["P1113"]=nil
   for _,class:=range fixture.classes{e.Claims["P31"]=append(e.Claims["P31"],property("normal",map[string]string{"id":class}))}
   return response(r,200,envelope(map[string]entity{"Q1":e})),nil
  })
  snapshot,err:=client.Fetch(context.Background(),[]string{"Q1"})
  if err!=nil||calls!=1||len(snapshot.Titles)!=1{t.Fatal("explicit media fetch",fixture,err)}
  title:=snapshot.Titles[0]
  if title.MediaType!=fixture.want||title.Status!="UNKNOWN"||title.EpisodeCount!=nil||len(title.Episodes)!=0||len(title.Relations)!=0{t.Fatal("inferred unavailable work facts",fixture,title)}
 }
 for _,classes:=range [][]string{{"Q11424"},{"Q1107"},{"Q63952888","Q20650540"},{"Q63952888","Q220898"},{"Q220898","Q1047299"}}{
  e:=item("Q1","Synthetic Ambiguous Work");e.Claims["P31"]=nil
  for _,class:=range classes{e.Claims["P31"]=append(e.Claims["P31"],property("normal",map[string]string{"id":class}))}
  if _,err:=animeMediaType(e);!errors.Is(err,ErrUnsupported){t.Fatal("unsupported/conflicting type guessed",classes,err)}
 }
 e:=item("Q1","Synthetic Rank Type")
 e.Claims["P31"]=[]claim{property("deprecated",map[string]string{"id":"Q20650540"})}
 if _,err:=animeMediaType(e);!errors.Is(err,ErrUnsupported){t.Fatal("deprecated class used")}
 e.Claims["P31"]=[]claim{property("normal",map[string]string{"id":"Q63952888"}),property("preferred",map[string]string{"id":"Q20650540"})}
 if kind,err:=animeMediaType(e);err!=nil||kind!="MOVIE"{t.Fatal("type rank not honored",kind,err)}
}
