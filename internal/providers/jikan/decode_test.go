package jikan

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/khordestan/anime-intelligence/internal/modules/catalog/feed"
)
const syntheticFull = `{"data":{"mal_id":1001,"title":"Synthetic Provider Sky","title_english":"Synthetic Sky English","title_japanese":"Synthetic Native","title_synonyms":["Alias","alias"],"synopsis":"A synthetic synopsis.","type":"TV","status":"Currently Airing","year":2026,"season":"winter","episodes":null,"genres":[{"name":"Adventure"},{"name":"adventure"}],"explicit_genres":[],"themes":[{"name":"Space"}],"demographics":[{"name":"Youth"}],"studios":[{"name":"Synthetic Studio"}],"relations":[{"relation":"Sequel","entry":[{"mal_id":1002,"type":"anime"},{"mal_id":1002,"type":"anime"},{"mal_id":1003,"type":"manga"}]},{"relation":"Adaptation","entry":[{"mal_id":1004,"type":"manga"}]}],"ignored_provider_field":"not-forwarded"}}`
func TestFullMetadataMapping(t *testing.T) {
	now:=time.Date(2026,1,2,0,0,0,0,time.UTC)
	s,err:=DecodeFull(strings.NewReader(syntheticFull),now,now)
	if err!=nil || s.Provider!="jikan" || len(s.Titles)!=1 {t.Fatal("full conversion",err)}
	a:=s.Titles[0]
	if a.ExternalID!="1001" || a.Title!="Synthetic Provider Sky" || a.NativeTitle!="Synthetic Native" || a.MediaType!="TV" || a.Status!="RELEASING" || a.Season!="WINTER" || a.EpisodeCount!=nil || len(a.Synonyms)!=2 || len(a.Genres)!=1 || len(a.Tags)!=2 || len(a.Themes)!=1 || a.Themes[0]!="Space" || len(a.Demographics)!=1 || a.Demographics[0]!="Youth" || len(a.Studios)!=1 || len(a.Relations)!=1 || a.Relations[0].ExternalID!="1002" {t.Fatalf("mapped metadata: %+v",a)}
	encoded,_:=json.Marshal(s)
	if strings.Contains(string(encoded),"ignored_provider_field") || strings.Contains(string(encoded),"not-forwarded") {t.Fatal("arbitrary provider fields escaped boundary")}
	for _,input:=range []string{"null","{}",`{"data":null}`,syntheticFull+" {}",strings.Replace(syntheticFull,"\"mal_id\":1001","\"mal_id\":0",1),strings.Repeat(" ",feed.MaxBytes+1)} {
		if _,err:=DecodeFull(strings.NewReader(input),now,now);err!=feed.ErrInvalid {t.Fatal("invalid full response accepted")}
	}
	if _,err:=DecodeFull(strings.NewReader(syntheticFull),now.Add(time.Second),now);err==nil {t.Fatal("future observation accepted")}
}
func TestEpisodeAiringIsNotConfirmedAvailability(t *testing.T) {
	page:=`{"data":[{"mal_id":1,"aired":"2026-01-01T12:00:00Z","url":"https://example.test/video"},{"mal_id":2,"aired":null}],"pagination":{"has_next_page":true}}`
	episodes,next,err:=EpisodePage(strings.NewReader(page))
	if err!=nil || !next || len(episodes)!=2 || episodes[0].Track!="UNKNOWN" || episodes[0].ScheduledAt==nil || episodes[0].ReleasedAt!=nil || episodes[1].ScheduledAt!=nil {t.Fatal("airing/availability distinction",episodes,err)}
	for _,input:=range []string{`{"data":[]}`,`{"data":null,"pagination":{}}`,`{"data":[{"mal_id":0}],"pagination":{}}`,`{"data":[{"mal_id":1},{"mal_id":1}],"pagination":{}}`,`{"data":[{"mal_id":1,"aired":"bad-date"}],"pagination":{}}`} {
		if _,_,err:=EpisodePage(strings.NewReader(input));err!=feed.ErrInvalid {t.Fatal("invalid episode page accepted")}
	}
	empty,next,err:=EpisodePage(strings.NewReader(`{"data":[],"pagination":{"has_next_page":false}}`))
	if err!=nil || next || empty==nil || len(empty)!=0 {t.Fatal("empty final page",err)}
}
func TestTypeStatusAndRelationMappings(t *testing.T) {
	now:=time.Date(2026,1,2,0,0,0,0,time.UTC)
	for _,tc:=range []struct{source,want string}{{"Movie","MOVIE"},{"TV Special","SPECIAL"},{"ONA","ONA"},{"Unknown future type","UNKNOWN"}} {
		input:=strings.Replace(syntheticFull,"\"type\":\"TV\"","\"type\":\""+tc.source+"\"",1)
		s,err:=DecodeFull(strings.NewReader(input),now,now)
		if err!=nil || s.Titles[0].MediaType!=tc.want {t.Fatal("media mapping",tc,err)}
	}
	for _,tc:=range []struct{source,want string}{{"Finished Airing","FINISHED"},{"Not yet aired","ANNOUNCED"},{"Unknown future status","UNKNOWN"}} {
		input:=strings.Replace(syntheticFull,"Currently Airing",tc.source,1)
		s,err:=DecodeFull(strings.NewReader(input),now,now)
		if err!=nil || s.Titles[0].Status!=tc.want {t.Fatal("status mapping",tc,err)}
	}
}

func TestCurrentTitlesAndLocalizedSearchAliases(t *testing.T){
 now:=time.Date(2026,1,2,0,0,0,0,time.UTC)
 input:=`{"data":{"mal_id":2001,"titles":[{"type":"Default","title":"Synthetic Modern Title"},{"type":"English","title":"Synthetic English"},{"type":"Japanese","title":"Synthetic Native"},{"type":"French","title":"Synthetic French"},{"type":"Synonym","title":"Synthetic Alias"},{"type":"Future Locale","title":"Synthetic Localized"},{"type":"Synonym","title":" synthetic alias "}],"type":"TV","status":"Currently Airing"}}`
 snapshot,err:=DecodeFull(strings.NewReader(input),now,now)
 if err!=nil{t.Fatal("modern-only response rejected",err)}
 title:=snapshot.Titles[0]
 if title.Title!="Synthetic Modern Title"||title.NativeTitle!="Synthetic Native"||len(title.Synonyms)!=4{t.Fatal("typed title mapping",title)}
 for _,alias:=range []string{"Synthetic English","Synthetic French","Synthetic Alias","Synthetic Localized"}{
  found:=false;for _,got:=range title.Synonyms{if got==alias{found=true}}
  if !found{t.Fatal("localized alias lost",alias)}
 }
 mixed:=strings.Replace(input,`"mal_id":2001`,`"mal_id":2001,"title":"Synthetic Legacy Title","title_japanese":"Synthetic Legacy Native","title_english":"Synthetic Legacy English"`,1)
 snapshot,err=DecodeFull(strings.NewReader(mixed),now,now)
 if err!=nil||snapshot.Titles[0].Title!="Synthetic Modern Title"||snapshot.Titles[0].NativeTitle!="Synthetic Native"||len(snapshot.Titles[0].Synonyms)!=7{t.Fatal("modern precedence or legacy aliases lost",snapshot,err)}
 for _,fallback:=range []struct{kind,want string}{{"English","Synthetic English Only"},{"Japanese","Synthetic Japanese Only"}}{
  raw:=`{"data":{"mal_id":2001,"titles":[{"type":"`+fallback.kind+`","title":"`+fallback.want+`"}],"type":"TV"}}`
  snapshot,err=DecodeFull(strings.NewReader(raw),now,now)
  if err!=nil||snapshot.Titles[0].Title!=fallback.want{t.Fatal("typed fallback",snapshot,err)}
 }
 for _,raw:=range []string{
  `{"data":{"mal_id":2001,"titles":[{"type":"Default","title":"One"},{"type":"Default","title":"Other"}]}}`,
  `{"data":{"mal_id":2001,"titles":[{"type":"Default","title":" "} ]}}`,
  `{"data":{"mal_id":2001,"titles":[{"type":"Unrecognized","title":"Unclassified Only"}]}}`,
  `{"data":{"mal_id":2001,"titles":[{"type":"Default","title":5}]}}`,
 }{
  if _,err=DecodeFull(strings.NewReader(raw),now,now);err!=feed.ErrInvalid{t.Fatal("ambiguous/invalid title accepted")}
 }
}
