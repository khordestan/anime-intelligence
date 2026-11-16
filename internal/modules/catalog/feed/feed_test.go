package feed

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

var clock = time.Date(2026,1,2,0,0,0,0,time.UTC)
func fixture() Snapshot {
	count := 12
	return Snapshot{Version:1,Provider:"synthetic",ObservedAt:clock.Add(-time.Hour),Titles:[]Title{{
		ExternalID:"season-1",Title:"Synthetic Sky",MediaType:"TV",Status:"RELEASING",
		EpisodeCount:&count,Genres:[]string{"Adventure"},Tags:[]string{"Space"},
		Relations:[]Relation{{ExternalID:"season-2",Type:"SEQUEL"}},
		Episodes:[]Episode{{Number:1,Track:"SUB"}},
	}}}
}
func TestDecode(t *testing.T) {
	b,_ := json.Marshal(fixture())
	got,err := Decode(strings.NewReader(string(b)),clock)
	if err != nil || got.Titles[0].Genres[0] != "Adventure" { t.Fatalf("roundtrip: %v",err) }
	for _, input := range []string{string(b)+" {}",string(b)+" garbage",strings.Replace(string(b),"\"version\":1","\"version\":1,\"token\":\"synthetic-secret\"",1),"null","",strings.Repeat(" ",MaxBytes+1)} {
		if _,err := Decode(strings.NewReader(input),clock); err != ErrInvalid { t.Fatal("invalid input accepted or provider content leaked") }
	}
}
func TestValidation(t *testing.T) {
	cases := map[string]func(*Snapshot){
		"future observation":func(s *Snapshot){s.ObservedAt=clock.Add(time.Hour)},
		"missing observation":func(s *Snapshot){s.ObservedAt=time.Time{}},
		"version":func(s *Snapshot){s.Version=2},
		"provider":func(s *Snapshot){s.Provider="https://example.test/token"},
		"blank title":func(s *Snapshot){s.Titles[0].Title=" "},
		"duplicate title":func(s *Snapshot){s.Titles=append(s.Titles,s.Titles[0])},
		"duplicate labels":func(s *Snapshot){s.Titles[0].Genres=[]string{"Action"," action "}},
		"duplicate themes":func(s *Snapshot){s.Titles[0].Themes=[]string{"Space"," space "}},
		"blank demographic":func(s *Snapshot){s.Titles[0].Demographics=[]string{" "}},
		"unknown media":func(s *Snapshot){s.Titles[0].MediaType="GAME"},
		"unknown status":func(s *Snapshot){s.Titles[0].Status="MAYBE"},
		"year":func(s *Snapshot){s.Titles[0].ReleaseYear=-1},
		"season":func(s *Snapshot){s.Titles[0].Season="MONSOON"},
		"negative count":func(s *Snapshot){v:=-1;s.Titles[0].EpisodeCount=&v},
		"duplicate episode":func(s *Snapshot){s.Titles[0].Episodes=append(s.Titles[0].Episodes,s.Titles[0].Episodes[0])},
		"zero episode":func(s *Snapshot){s.Titles[0].Episodes[0].Number=0},
		"beyond count":func(s *Snapshot){s.Titles[0].Episodes[0].Number=13},
		"unknown track":func(s *Snapshot){s.Titles[0].Episodes[0].Track="EN"},
		"future release":func(s *Snapshot){v:=clock;s.Titles[0].Episodes[0].ReleasedAt=&v},
		"zero schedule":func(s *Snapshot){s.Titles[0].Episodes[0].ScheduledAt=&time.Time{}},
		"self relation":func(s *Snapshot){s.Titles[0].Relations[0].ExternalID="season-1"},
		"unknown relation":func(s *Snapshot){s.Titles[0].Relations[0].Type="OTHER"},
		"duplicate relation":func(s *Snapshot){s.Titles[0].Relations=append(s.Titles[0].Relations,s.Titles[0].Relations[0])},
	}
	for name,change := range cases { t.Run(name,func(t *testing.T){s:=fixture();change(&s);if s.Validate(clock)==nil {t.Fatal("accepted invalid snapshot")}}) }
	s:=fixture();s.Titles[0].EpisodeCount=nil;s.Titles[0].Episodes[0].Number=500
	if s.Validate(clock)!=nil {t.Fatal("unknown episode count should remain unknown")}
}
func TestReleaseDetection(t *testing.T) {
	old:=fixture()
	next:=fixture();next.ObservedAt=clock
	past:=clock.Add(-2*time.Hour)
	next.Titles[0].Episodes[0].ScheduledAt=&past
	events,err:=Releases(&old,next,clock)
	if err!=nil || len(events)!=0 {t.Fatal("elapsed schedule must not imply release")}
	confirmed:=clock.Add(-30*time.Minute)
	next.Titles[0].Episodes[0].ReleasedAt=&confirmed
	events,err=Releases(nil,next,clock)
	if err!=nil || len(events)!=0 {t.Fatal("first sync must establish baseline")}
	events,err=Releases(&old,next,clock)
	if err!=nil || len(events)!=1 || events[0].Type!="EPISODE_RELEASED" {t.Fatalf("new confirmed release: %v %v",events,err)}
	key:=events[0].Key
	later:=next;later.ObservedAt=clock.Add(time.Hour)
	events,err=Releases(&next,later,clock.Add(time.Hour))
	if err!=nil || len(events)!=0 {t.Fatal("repeat release should be suppressed")}
	corrected:=clock.Add(-15*time.Minute)
	next.Titles[0].Episodes[0].ReleasedAt=&corrected
	events,err=Releases(&old,next,clock)
	if err!=nil || len(events)!=1 || events[0].Key!=key {t.Fatal("timestamp correction changed event identity")}
	next.Titles[0].Episodes[0].Track="DUB"
	events,err=Releases(&old,next,clock)
	if err!=nil || len(events)!=1 || events[0].Key==key {t.Fatal("release tracks must have separate identities")}
	for _, mutate := range []func(*Snapshot){
		func(s *Snapshot){s.Provider="other"},
		func(s *Snapshot){s.ObservedAt=old.ObservedAt},
		func(s *Snapshot){s.ObservedAt=old.ObservedAt.Add(-time.Second)},
	} { bad:=fixture();mutate(&bad);if _,err:=Releases(&old,bad,clock);err==nil {t.Fatal("invalid comparison accepted")} }
}
func TestReleaseOrderingAndPartialSnapshots(t *testing.T) {
	old:=fixture()
	next:=fixture();next.ObservedAt=clock
	at:=clock.Add(-30*time.Minute)
	next.Titles[0].Episodes=[]Episode{{Number:2,Track:"SUB",ReleasedAt:&at},{Number:1,Track:"SUB",ReleasedAt:&at}}
	a,err:=Releases(&old,next,clock)
	if err!=nil || len(a)!=2 {t.Fatal("expected two events")}
	next.Titles[0].Episodes[0],next.Titles[0].Episodes[1]=next.Titles[0].Episodes[1],next.Titles[0].Episodes[0]
	b,err:=Releases(&old,next,clock)
	if err!=nil || a[0].Key!=b[0].Key || a[1].Key!=b[1].Key {t.Fatal("unstable event ordering")}
	empty:=Snapshot{Version:1,Provider:old.Provider,ObservedAt:clock}
	events,err:=Releases(&old,empty,clock)
	if err!=nil || len(events)!=0 {t.Fatal("partial snapshot must not retract records")}
}

func TestReleasePreviewBaselinesHistoricalConfirmations(t *testing.T){
 previous:=fixture()
 next:=fixture();next.ObservedAt=clock
 for _,existing:=range []bool{true,false}{
  for name,offset:=range map[string]time.Duration{"historical":-time.Hour,"boundary":0,"sub-microsecond":999*time.Nanosecond,"new":time.Microsecond}{
   t.Run(name+map[bool]string{true:"-known",false:"-new-title"}[existing],func(t *testing.T){
    at:=previous.ObservedAt.Add(offset)
    title:=fixture().Titles[0]
    if !existing{title.ExternalID="new-season";title.Relations=nil}
    title.Episodes=[]Episode{{Number:1,Track:"SUB",ReleasedAt:&at}}
    next.Titles=[]Title{title}
    events,err:=Releases(&previous,next,clock)
    want:=0;if offset>=time.Microsecond{want=1}
    if err!=nil||len(events)!=want{t.Fatal("historical confirmation treated as new release",events,err)}
   })
  }
 }
 // The importer cannot distinguish observations within the same microsecond.
 same:=fixture();same.ObservedAt=previous.ObservedAt.Add(500*time.Nanosecond)
 if _,err:=Releases(&previous,same,clock);err!=ErrInvalid{t.Fatal("same stored observation precision accepted")}
}
