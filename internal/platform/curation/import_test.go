package curation

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)
func fixture()Snapshot{
	value,confidence:=0.5,1.0
	return Snapshot{Version:1,ObservedAt:time.Date(2026,1,1,0,0,0,0,time.UTC),Records:[]Record{{AnimeID:"10000000-0000-4000-8000-000000000001",FeatureKey:"synthetic.power",Value:&value,Confidence:&confidence,Reference:"https://example.test/evidence"}}}
}
func TestStrictPublicFeatureInput(t *testing.T){
	now:=time.Date(2026,1,2,0,0,0,0,time.UTC)
	b,_:=json.Marshal(fixture())
	if _,err:=Decode(strings.NewReader(string(b)),now);err!=nil{t.Fatal(err)}
	for _,input:=range []string{string(b)+" {}",strings.Replace(string(b),"\"version\":1","\"version\":1,\"userId\":\"spoofed\"",1),strings.Repeat(" ",(1<<20)+1),"null"}{
		if _,err:=Decode(strings.NewReader(input),now);err!=ErrInvalid{t.Fatal("invalid input accepted")}
	}
	for _,change:=range []func(*Snapshot){
		func(s *Snapshot){s.ObservedAt=now.Add(time.Second)},
		func(s *Snapshot){s.Records[0].Value=nil},
		func(s *Snapshot){v:=0.12345;s.Records[0].Value=&v},
		func(s *Snapshot){v:=0.0;s.Records[0].Confidence=&v},
		func(s *Snapshot){s.Records[0].Reference="https://example.test/?token=synthetic"},
		func(s *Snapshot){s.Records[0].Reference="https://user:synthetic@example.test/"},
		func(s *Snapshot){s.Records=append(s.Records,s.Records[0])},
		func(s *Snapshot){s.Records[0].AnimeID="not-an-anime-uuid"},
	}{
		s:=fixture();change(&s);if s.Validate(now)!=ErrInvalid{t.Fatal("invalid curation accepted")}
	}
}
