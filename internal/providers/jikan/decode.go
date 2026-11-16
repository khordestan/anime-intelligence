// Package jikan converts reviewed offline Jikan v4 responses. It does not make
// network requests or imply approval to collect/redistribute provider data.
package jikan

import (
	"encoding/json"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/khordestan/anime-intelligence/internal/modules/catalog/feed"
)
type named struct {Name string `json:"name"`}
type resource struct {ID int `json:"mal_id"`; Type string `json:"type"`}
type full struct {
	ID int `json:"mal_id"`
	Titles []struct{Type string `json:"type"`;Title string `json:"title"`} `json:"titles"`
	Title string `json:"title"`
	English string `json:"title_english"`
	Japanese string `json:"title_japanese"`
	Synonyms []string `json:"title_synonyms"`
	Synopsis string `json:"synopsis"`
	Type string `json:"type"`
	Status string `json:"status"`
	Year int `json:"year"`
	Season string `json:"season"`
	Episodes *int `json:"episodes"`
	Genres []named `json:"genres"`
	ExplicitGenres []named `json:"explicit_genres"`
	Themes []named `json:"themes"`
	Demographics []named `json:"demographics"`
	Studios []named `json:"studios"`
	Relations []struct {Relation string `json:"relation"`; Entries []resource `json:"entry"`} `json:"relations"`
}
func decode(r io.Reader,out any) error {
	b,err:=io.ReadAll(io.LimitReader(r,feed.MaxBytes+1))
	if err!=nil || len(b)>feed.MaxBytes {return feed.ErrInvalid}
	d:=json.NewDecoder(strings.NewReader(string(b)))
	// Provider envelopes evolve. Only selected typed fields are retained;
	// arbitrary unknown provider fields never pass into the canonical feed.
	if d.Decode(out)!=nil {return feed.ErrInvalid}
	var extra any
	if d.Decode(&extra)!=io.EOF {return feed.ErrInvalid}
	return nil
}
func unique(values []string) []string {
	out:=[]string{}
	seen:=map[string]bool{}
	for _,value:=range values {
		value=strings.TrimSpace(value)
		key:=strings.ToLower(value)
		if value!="" && !seen[key] {out=append(out,value);seen[key]=true}
	}
	return out
}
func names(groups ...[]named) []string {
	values:=[]string{}
	for _,group:=range groups {for _,value:=range group {values=append(values,value.Name)}}
	return unique(values)
}
func DecodeFull(r io.Reader,observed,now time.Time) (feed.Snapshot,error) {
	var envelope struct{Data *full `json:"data"`}
	if err:=decode(r,&envelope);err!=nil || envelope.Data==nil {return feed.Snapshot{},feed.ErrInvalid}
	d:=envelope.Data
	if d.ID<1 {return feed.Snapshot{},feed.ErrInvalid}
	// Prefer the current typed title collection. Deprecated fields remain
	// fallbacks and aliases, never the only accepted source of title identity.
	primary:=map[string]string{}
	aliases:=append([]string{},d.Synonyms...)
	aliases=append(aliases,d.English,d.Title,d.Japanese)
	for _,title:=range d.Titles{
		value:=strings.TrimSpace(title.Title)
		if value==""{continue}
		aliases=append(aliases,value)
		kind:=strings.ToLower(strings.TrimSpace(title.Type))
		switch kind{case "default","english","japanese":
			if prior:=primary[kind];prior!=""&&!strings.EqualFold(prior,value){return feed.Snapshot{},feed.ErrInvalid}
			primary[kind]=value
		}
	}
	first:=func(values ...string)string{for _,value:=range values{if value=strings.TrimSpace(value);value!=""{return value}};return ""}
	canonical:=first(primary["default"],d.Title,primary["english"],d.English,primary["japanese"],d.Japanese)
	native:=first(primary["japanese"],d.Japanese)
	namesForSearch:=[]string{}
	for _,alias:=range unique(aliases){if !strings.EqualFold(alias,canonical)&&!strings.EqualFold(alias,native){namesForSearch=append(namesForSearch,alias)}}
	t:=feed.Title{ExternalID:strconv.Itoa(d.ID),Title:canonical,NativeTitle:native,Synopsis:d.Synopsis,
		MediaType:"UNKNOWN",Status:"UNKNOWN",ReleaseYear:d.Year,EpisodeCount:d.Episodes,
		Synonyms:namesForSearch,Genres:names(d.Genres,d.ExplicitGenres),Tags:names(d.Themes,d.Demographics),Themes:names(d.Themes),Demographics:names(d.Demographics),Studios:names(d.Studios)}
	switch d.Type {
	case "TV","MOVIE","OVA","ONA","SPECIAL","MUSIC":t.MediaType=d.Type
	case "Movie":t.MediaType="MOVIE"
	case "Special","TV Special":t.MediaType="SPECIAL"
	case "Music":t.MediaType="MUSIC"
	}
	switch d.Status {case "Currently Airing":t.Status="RELEASING";case "Finished Airing":t.Status="FINISHED";case "Not yet aired":t.Status="ANNOUNCED"}
	season:=strings.ToUpper(d.Season)
	switch season {case "WINTER","SPRING","SUMMER","FALL":t.Season=season}
	seen:=map[string]bool{}
	for _,relation:=range d.Relations {
		kind:=""
		switch relation.Relation {
		case "Prequel":kind="PREQUEL"
		case "Sequel":kind="SEQUEL"
		case "Side story":kind="SIDE_STORY"
		case "Alternative version","Alternative setting":kind="ALTERNATIVE"
		case "Parent story":kind="PARENT"
		case "Summary":kind="SUMMARY"
		}
		if kind=="" {continue}
		for _,entry:=range relation.Entries {
			if entry.Type!="anime" || entry.ID<1 {continue}
			id:=strconv.Itoa(entry.ID)
			key:=kind+":"+id
			if seen[key] {continue}
			t.Relations=append(t.Relations,feed.Relation{ExternalID:id,Type:kind})
			seen[key]=true
		}
	}
	s:=feed.Snapshot{Version:1,Provider:"jikan",ObservedAt:observed,Titles:[]feed.Title{t}}
	if s.Validate(now)!=nil {return feed.Snapshot{},feed.ErrInvalid}
	return s,nil
}

// EpisodePage preserves an airing timestamp as a schedule with UNKNOWN track.
// It never promotes aired dates, videos, scores or an anime's status into
// confirmed regional/sub/dub availability.
func EpisodePage(r io.Reader) ([]feed.Episode,bool,error) {
	var envelope struct {
		Data *[]struct{Number int `json:"mal_id"`; Aired *time.Time `json:"aired"`} `json:"data"`
		Pagination *struct{HasNext bool `json:"has_next_page"`} `json:"pagination"`
	}
	if err:=decode(r,&envelope);err!=nil || envelope.Data==nil || envelope.Pagination==nil {return nil,false,feed.ErrInvalid}
	out:=[]feed.Episode{}
	seen:=map[int]bool{}
	for _,e:=range *envelope.Data {
		if e.Number<1 || e.Number>100000 || seen[e.Number] || (e.Aired!=nil && e.Aired.IsZero()) {return nil,false,feed.ErrInvalid}
		seen[e.Number]=true
		out=append(out,feed.Episode{Number:e.Number,Track:"UNKNOWN",ScheduledAt:e.Aired})
	}
	return out,envelope.Pagination.HasNext,nil
}
