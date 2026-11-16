// Package wikidata retrieves selected CC0 structured facts from fixed Wikidata
// endpoints. It never reads user lists, downloads media or confirms releases.
package wikidata

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/khordestan/anime-intelligence/internal/modules/catalog/feed"
)
var ErrProvider = errors.New("wikidata request or response failed")
var ErrUnsupported = errors.New("wikidata item has no supported unambiguous anime media type")
var ErrRateLimited = errors.New("wikidata rate limit reached")
type RateLimitError struct{ Wait time.Duration; Until time.Time }
func (e *RateLimitError) Error() string { return ErrRateLimited.Error() }
func (e *RateLimitError) Unwrap() error { return ErrRateLimited }
func retryAfter(value string,now time.Time) time.Duration {
	value=strings.TrimSpace(value)
	digits:=value!=""
	for _,r:=range value {if r<'0'||r>'9'{digits=false}}
	if digits{
		seconds,err:=strconv.ParseUint(value,10,64)
		if err!=nil||seconds>uint64(math.MaxInt64/int64(time.Second)){return time.Duration(math.MaxInt64)}
		delay:=time.Duration(seconds)*time.Second
		if delay<time.Second{return time.Second}
		return delay
	}
	if until,err:=http.ParseTime(value);err==nil{
		delay:=until.Sub(now)
		if delay<time.Second{return time.Second}
		return delay
	}
	return time.Minute
}

var qid = regexp.MustCompile(`^Q[1-9][0-9]{0,11}$`)
var yearPrefix = regexp.MustCompile(`^\+([0-9]{4})-`)
var gate sync.Mutex
var nextRequest time.Time

type Client struct {
	http *http.Client
	now func() time.Time
	wait func(context.Context,time.Duration) error
}
func New() *Client { return newClient(http.DefaultTransport) }
func newClient(transport http.RoundTripper) *Client {
	return &Client{
		http:&http.Client{Transport:transport,Timeout:15*time.Second,CheckRedirect:func(*http.Request,[]*http.Request)error{return http.ErrUseLastResponse}},
		now:time.Now,
		wait:func(ctx context.Context,d time.Duration)error{timer:=time.NewTimer(d);defer timer.Stop();select{case <-ctx.Done():return ctx.Err();case <-timer.C:return nil}},
	}
}
type text struct {Value string `json:"value"`}
type claim struct {
	Rank string `json:"rank"`
	Snak struct {
		Type string `json:"snaktype"`
		Value struct{Value json.RawMessage `json:"value"`} `json:"datavalue"`
	} `json:"mainsnak"`
}
type entity struct {
	ID string `json:"id"`
	Type string `json:"type"`
	Missing *string `json:"missing"`
	Labels map[string]text `json:"labels"`
	Aliases map[string][]text `json:"aliases"`
	Claims map[string][]claim `json:"claims"`
}
func (c *Client) entities(ctx context.Context,ids []string,props string)(map[string]entity,error) {
	params:=url.Values{"action":{"wbgetentities"},"ids":{strings.Join(ids,"|")},"props":{props},"languages":{"en|ja"},"format":{"json"},"maxlag":{"5"}}
	req,err:=http.NewRequestWithContext(ctx,http.MethodGet,"https://www.wikidata.org/w/api.php?"+params.Encode(),nil)
	if err!=nil {return nil,ErrProvider}
	req.Header.Set("User-Agent","AnimeIntelligence/0.1 (https://github.com/khordestan/anime-intelligence)")
	req.Header.Set("Accept","application/json")
	// Serialize and pace requests process-wide, including multiple clients.
	gate.Lock()
	defer gate.Unlock()
	if delay:=nextRequest.Sub(c.now());delay>0 {if c.wait(ctx,delay)!=nil {return nil,ErrProvider}}
	nextRequest=c.now().Add(time.Second)
	res,err:=c.http.Do(req)
	if err!=nil {return nil,ErrProvider}
	defer res.Body.Close()
	if res.StatusCode==429 || (res.StatusCode==503 && res.Header.Get("Retry-After")!="") {
		delay:=retryAfter(res.Header.Get("Retry-After"),c.now())
		nextRequest=c.now().Add(delay)
		return nil,&RateLimitError{Wait:delay,Until:nextRequest}
	}
	if res.StatusCode!=http.StatusOK {return nil,ErrProvider}
	b,err:=io.ReadAll(io.LimitReader(res.Body,feed.MaxBytes+1))
	if err!=nil || len(b)>feed.MaxBytes {return nil,ErrProvider}
	var envelope struct {
		Entities map[string]entity `json:"entities"`
		Error json.RawMessage `json:"error"`
	}
	d:=json.NewDecoder(strings.NewReader(string(b)))
	if d.Decode(&envelope)!=nil || len(envelope.Error)>0 {return nil,ErrProvider}
	var extra any
	if d.Decode(&extra)!=io.EOF {return nil,ErrProvider}
	for _,id:=range ids {
		e,ok:=envelope.Entities[id]
		if !ok || e.ID!=id || e.Type!="item" || e.Missing!=nil {return nil,ErrProvider}
	}
	return envelope.Entities,nil
}
func values(e entity,property string) []json.RawMessage {
	preferred:=false
	for _,c:=range e.Claims[property] {if c.Rank=="preferred" && c.Snak.Type=="value" {preferred=true}}
	out:=[]json.RawMessage{}
	for _,c:=range e.Claims[property] {
		if c.Snak.Type!="value" || c.Rank=="deprecated" || (preferred && c.Rank!="preferred") {continue}
		if c.Rank!="normal" && c.Rank!="preferred" {continue}
		out=append(out,c.Snak.Value.Value)
	}
	return out
}
func itemIDs(e entity,property string) []string {
	out:=[]string{}
	seen:=map[string]bool{}
	for _,raw:=range values(e,property) {
		var v struct{ID string `json:"id"`}
		if json.Unmarshal(raw,&v)==nil && qid.MatchString(v.ID) && !seen[v.ID] {out=append(out,v.ID);seen[v.ID]=true}
	}
	return out
}
func label(e entity) string {
	if v:=strings.TrimSpace(e.Labels["en"].Value);v!="" {return v}
	return strings.TrimSpace(e.Labels["ja"].Value)
}
func episodeCount(e entity) *int {
	unique:=map[int]bool{}
	for _,raw:=range values(e,"P1113") {
		var v struct{Amount string `json:"amount"`;Unit string `json:"unit"`}
		if json.Unmarshal(raw,&v)!=nil || v.Unit!="1" {continue}
		n,err:=strconv.Atoi(strings.TrimPrefix(v.Amount,"+"))
		if err==nil && n>=0 && n<=100000 {unique[n]=true}
	}
	if len(unique)!=1 {return nil}
	for n:=range unique {return &n}
	return nil
}
func releaseYear(e entity) int {
	years:=map[int]bool{}
	for _,raw:=range values(e,"P577") {
		var v struct{Time string `json:"time"`;Precision int `json:"precision"`}
		if json.Unmarshal(raw,&v)!=nil || v.Precision<9 {continue}
		m:=yearPrefix.FindStringSubmatch(v.Time)
		if len(m)!=2 {continue}
		n,_:=strconv.Atoi(m[1])
		if n>=1900 && n<=9999 {years[n]=true}
	}
	// Publication dates may describe different regions; choose the earliest
	// explicitly asserted year without claiming an exact broadcast timestamp.
	year:=0
	for n:=range years {if year==0 || n<year {year=n}}
	return year
}
// Only explicitly asserted supported P31 classes participate. OVA/ONA are
// more specific distribution formats than the broad anime-film class. No
// subclass crawl, label guessing, parent-season inference or status promotion.
func animeMediaType(e entity)(string,error){
 types:=map[string]bool{}
 for _,class:=range itemIDs(e,"P31"){
  switch class{
  case "Q63952888","Q100269041":types["TV"]=true
  case "Q20650540":types["MOVIE"]=true
  case "Q220898":types["OVA"]=true
  case "Q1047299":types["ONA"]=true
  }
 }
 if types["OVA"]||types["ONA"]{
  if types["TV"]||(types["OVA"]&&types["ONA"]){return "",ErrUnsupported}
  if types["OVA"]{return "OVA",nil};return "ONA",nil
 }
 if len(types)!=1{return "",ErrUnsupported}
 for kind:=range types{return kind,nil}
 return "",ErrUnsupported
}
// Fetch accepts up to twenty explicit item IDs. Direct supported anime-work
// classes are required; broader/subclass inference is intentionally unsupported.
// At most one item request and two genre-label requests are issued; no crawl.
func (c *Client) Fetch(ctx context.Context,ids []string)(feed.Snapshot,error) {
	if ctx.Err()!=nil {return feed.Snapshot{},ErrProvider}
	if len(ids)==0 || len(ids)>20 {return feed.Snapshot{},feed.ErrInvalid}
	seen:=map[string]bool{}
	for _,id:=range ids {if !qid.MatchString(id) || seen[id] {return feed.Snapshot{},feed.ErrInvalid};seen[id]=true}
	items,err:=c.entities(ctx,ids,"labels|aliases|claims")
	if err!=nil {return feed.Snapshot{},err}
	genreIDs:=map[string]bool{}
	mediaTypes:=map[string]string{}
	for _,id:=range ids {
		e:=items[id]
		kind,err:=animeMediaType(e)
		if err!=nil{return feed.Snapshot{},err}
		mediaTypes[id]=kind
		for _,genre:=range itemIDs(e,"P136") {genreIDs[genre]=true}
	}
	if len(genreIDs)>100 {return feed.Snapshot{},ErrProvider}
	ordered:=make([]string,0,len(genreIDs))
	for id:=range genreIDs {ordered=append(ordered,id)}
	sort.Strings(ordered)
	genres:=map[string]entity{}
	for offset:=0;offset<len(ordered);offset+=50 {
		end:=offset+50;if end>len(ordered){end=len(ordered)}
		batch,err:=c.entities(ctx,ordered[offset:end],"labels")
		if err!=nil {return feed.Snapshot{},err}
		for id,e:=range batch {genres[id]=e}
	}
	s:=feed.Snapshot{Version:1,Provider:"wikidata",ObservedAt:c.now().UTC(),Titles:[]feed.Title{}}
	for _,id:=range ids {
		e:=items[id]
		t:=feed.Title{ExternalID:id,Title:label(e),NativeTitle:e.Labels["ja"].Value,MediaType:mediaTypes[id],Status:"UNKNOWN",ReleaseYear:releaseYear(e),EpisodeCount:episodeCount(e)}
		names:=map[string]bool{strings.ToLower(t.Title):true}
		for _,language:=range []string{"en","ja"} {
			for _,alias:=range e.Aliases[language] {
				v:=strings.TrimSpace(alias.Value);key:=strings.ToLower(v)
				if v!="" && !names[key] {t.Synonyms=append(t.Synonyms,v);names[key]=true}
			}
		}
		usedGenres:=map[string]bool{}
		for _,genre:=range itemIDs(e,"P136") {
			name:=label(genres[genre]);key:=strings.ToLower(name)
			if name!="" && !usedGenres[key] {t.Genres=append(t.Genres,name);usedGenres[key]=true}
		}
		s.Titles=append(s.Titles,t)
	}
	if s.Validate(c.now().UTC())!=nil {return feed.Snapshot{},feed.ErrInvalid}
	return s,nil
}
