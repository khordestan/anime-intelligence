// Package tvmaze normalizes reviewed animation seasons and unconfirmed schedules.
// TVmaze data is CC BY-SA 4.0; attribution and adaptation notices accompany use.
package tvmaze

import (
 "context"
 "encoding/json"
 "errors"
 "io"
 "math"
 "net/http"
 "regexp"
 "sort"
 "strconv"
 "strings"
 "sync"
 "time"

 "github.com/khordestan/anime-intelligence/internal/modules/catalog/feed"
)

var ErrProvider=errors.New("tvmaze request or response failed")
var ErrReview=errors.New("tvmaze anime season review is required")
var ErrRateLimited=errors.New("tvmaze rate limit reached")
type RateLimitError struct{Wait time.Duration;Until time.Time}
func(e *RateLimitError)Error()string{return ErrRateLimited.Error()}
func(e *RateLimitError)Unwrap()error{return ErrRateLimited}

var keyPattern=regexp.MustCompile(`^show-([1-9][0-9]{0,8})-season-([1-9][0-9]{0,8})$`)
func ParseID(id string)(int,int,error){
 parts:=keyPattern.FindStringSubmatch(id)
 if len(parts)!=3{return 0,0,ErrReview}
 show,_:=strconv.Atoi(parts[1]);season,_:=strconv.Atoi(parts[2])
 return show,season,nil
}
type Client struct{
 http *http.Client
 now func()time.Time
 wait func(context.Context,time.Duration)error
 reviewed bool
}
var requestGate=make(chan struct{},1)
var cooldownMu sync.Mutex
var nextRequest time.Time

// New requires explicit operator review of anime classification and season
// identity. Animation alone cannot establish that a work is anime.
func New(reviewedAnimeSeasons bool)*Client{return newClient(http.DefaultTransport,reviewedAnimeSeasons)}
func newClient(transport http.RoundTripper,reviewed bool)*Client{
 return &Client{http:&http.Client{Transport:transport,Timeout:15*time.Second,
 CheckRedirect:func(*http.Request,[]*http.Request)error{return http.ErrUseLastResponse}},
 now:time.Now,reviewed:reviewed,wait:func(ctx context.Context,d time.Duration)error{
 timer:=time.NewTimer(d);defer timer.Stop()
 select{case <-ctx.Done():return ctx.Err();case <-timer.C:return nil}
 }}
}
type show struct{ID int `json:"id"`;Name string `json:"name"`;Type string `json:"type"`}
type season struct{ID int `json:"id"`;Number int `json:"number"`;Premiere *string `json:"premiereDate"`}
type episode struct{ID int `json:"id"`;Season int `json:"season"`;Number *int `json:"number"`;Type string `json:"type"`;Airstamp *string `json:"airstamp"`}

func retryAfter(value string,now time.Time)time.Duration{
 value=strings.TrimSpace(value)
 digits:=value!=""
 for _,r:=range value{if r<'0'||r>'9'{digits=false}}
 if digits{
  n,err:=strconv.ParseUint(value,10,64)
  if err!=nil||n>uint64(math.MaxInt64/int64(time.Second)){return time.Duration(math.MaxInt64)}
  d:=time.Duration(n)*time.Second;if d<time.Second{return time.Second};return d
 }
 if until,err:=http.ParseTime(value);err==nil{d:=until.Sub(now);if d<time.Second{return time.Second};return d}
 return time.Minute
}
func(c *Client)read(ctx context.Context,path string,out any)error{
 // A channel makes cancellation interrupt waiting for the process-wide gate.
 select{case requestGate<-struct{}{}:case <-ctx.Done():return ErrProvider}
 defer func(){<-requestGate}()
 cooldownMu.Lock();next:=nextRequest;cooldownMu.Unlock()
 if delay:=next.Sub(c.now());delay>0{if c.wait(ctx,delay)!=nil{return ErrProvider}}
 if ctx.Err()!=nil{return ErrProvider}
 req,err:=http.NewRequestWithContext(ctx,http.MethodGet,"https://api.tvmaze.com"+path,nil)
 if err!=nil{return ErrProvider}
 req.Header.Set("User-Agent","AnimeIntelligence/0.1 (https://github.com/khordestan/anime-intelligence)")
 req.Header.Set("Accept","application/json")
 cooldownMu.Lock();nextRequest=c.now().Add(time.Second);cooldownMu.Unlock()
 res,err:=c.http.Do(req)
 if err!=nil{return ErrProvider}
 defer res.Body.Close()
 if res.StatusCode==429||res.StatusCode==503{
  delay:=retryAfter(res.Header.Get("Retry-After"),c.now())
  until:=c.now().Add(delay)
  cooldownMu.Lock();if until.After(nextRequest){nextRequest=until};cooldownMu.Unlock()
  return &RateLimitError{Wait:delay,Until:until}
 }
 if res.StatusCode!=http.StatusOK{return ErrProvider}
 b,err:=io.ReadAll(io.LimitReader(res.Body,feed.MaxBytes+1))
 if err!=nil||len(b)>feed.MaxBytes{return ErrProvider}
 d:=json.NewDecoder(strings.NewReader(string(b)))
 if d.Decode(out)!=nil{return ErrProvider}
 var extra any
 if d.Decode(&extra)!=io.EOF{return ErrProvider}
 return nil
}

// Fetch accepts exactly one reviewed show/season pair. Provider season membership
// is checked before episode retrieval. Returned dates never become ReleasedAt;
// unknown totals stay unknown and specials are excluded without renumbering.
func(c *Client)Fetch(ctx context.Context,ids []string)(feed.Snapshot,error){
 if !c.reviewed{return feed.Snapshot{},ErrReview}
 if len(ids)!=1||ctx.Err()!=nil{return feed.Snapshot{},ErrReview}
 showID,seasonID,err:=ParseID(ids[0]);if err!=nil{return feed.Snapshot{},ErrReview}
 var work show
 if err=c.read(ctx,"/shows/"+strconv.Itoa(showID),&work);err!=nil{return feed.Snapshot{},err}
 if work.ID!=showID||work.Type!="Animation"||strings.TrimSpace(work.Name)==""{return feed.Snapshot{},ErrReview}
 var seasons []season
 if err=c.read(ctx,"/shows/"+strconv.Itoa(showID)+"/seasons",&seasons);err!=nil{return feed.Snapshot{},err}
 if len(seasons)>1000{return feed.Snapshot{},ErrProvider}
 var selected *season
 seenSeasons:=map[int]bool{}
 for i:=range seasons{
  entry:=&seasons[i]
  if entry.ID<1||entry.Number<0||seenSeasons[entry.ID]{return feed.Snapshot{},ErrProvider}
  seenSeasons[entry.ID]=true
  if entry.ID==seasonID{selected=entry}
 }
 if selected==nil||selected.Number<1{return feed.Snapshot{},ErrReview}
 var episodes []episode
 if err=c.read(ctx,"/seasons/"+strconv.Itoa(seasonID)+"/episodes",&episodes);err!=nil{return feed.Snapshot{},err}
 if episodes==nil||len(episodes)>10000{return feed.Snapshot{},ErrProvider}
 title:=feed.Title{ExternalID:ids[0],Title:strings.TrimSpace(work.Name)+" — Season "+strconv.Itoa(selected.Number),
 MediaType:"TV",Status:"UNKNOWN",Genres:[]string{},Episodes:[]feed.Episode{}}
 if selected.Premiere!=nil&&*selected.Premiere!=""{
  date,err:=time.Parse("2006-01-02",*selected.Premiere)
  if err!=nil||date.Year()<1900||date.Year()>9999{return feed.Snapshot{},ErrProvider}
  title.ReleaseYear=date.Year()
 }
 seenEpisodes:=map[int]bool{}
 for _,ep:=range episodes{
  if ep.ID<1||ep.Season!=selected.Number||seenEpisodes[ep.ID]{return feed.Snapshot{},ErrProvider}
  seenEpisodes[ep.ID]=true
  // TVmaze returns specials too. Neither named nor numbered specials get
  // mixed into the regular season's progress or assigned synthetic numbers.
  if ep.Number==nil||ep.Type!="regular"{continue}
  out:=feed.Episode{Number:*ep.Number,Track:"UNKNOWN"}
  if ep.Airstamp!=nil&&*ep.Airstamp!=""{
   stamp,err:=time.Parse(time.RFC3339,*ep.Airstamp)
   if err!=nil{return feed.Snapshot{},ErrProvider}
   stamp=stamp.UTC();out.ScheduledAt=&stamp
  }
  title.Episodes=append(title.Episodes,out)
 }
 sort.Slice(title.Episodes,func(i,j int)bool{return title.Episodes[i].Number<title.Episodes[j].Number})
 now:=c.now().UTC()
 snapshot:=feed.Snapshot{Version:1,Provider:"tvmaze",ObservedAt:now,Titles:[]feed.Title{title}}
 if snapshot.Validate(now)!=nil{return feed.Snapshot{},ErrProvider}
 return snapshot,nil
}
