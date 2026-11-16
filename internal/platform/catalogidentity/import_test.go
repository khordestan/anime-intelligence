package catalogidentity

import (
 "encoding/json"
 "strings"
 "testing"
 "time"
)
func fixture()Snapshot{return Snapshot{Version:1,Provider:"synthetic",ReviewedAt:time.Date(2026,1,2,0,0,0,0,time.UTC),Reference:"https://example.test/identity-review",Mappings:[]Mapping{{ExternalID:"work-1",AnimeID:"10000000-0000-4000-8000-000000000001"}}}}
func TestReviewedIdentityValidationAndDecoder(t *testing.T){
 now:=fixture().ReviewedAt
 raw,_:=json.Marshal(fixture())
 if s,err:=Decode(strings.NewReader(string(raw)),now);err!=nil||len(s.Mappings)!=1{t.Fatal("identity roundtrip",err)}
 for name,mutate:=range map[string]func(*Snapshot){
  "future":func(s *Snapshot){s.ReviewedAt=now.Add(time.Second)},
  "version":func(s *Snapshot){s.Version=2},
  "provider":func(s *Snapshot){s.Provider="https://example.test/key"},
  "missing":func(s *Snapshot){s.Mappings=nil},
  "bad id":func(s *Snapshot){s.Mappings[0].AnimeID="bad"},
  "bad external":func(s *Snapshot){s.Mappings[0].ExternalID="work/id"},
  "duplicate external":func(s *Snapshot){s.Mappings=append(s.Mappings,Mapping{ExternalID:"work-1",AnimeID:"10000000-0000-4000-8000-000000000002"})},
  "duplicate target":func(s *Snapshot){s.Mappings=append(s.Mappings,Mapping{ExternalID:"work-2",AnimeID:s.Mappings[0].AnimeID})},
  "credentials":func(s *Snapshot){s.Reference="https://name:secret@example.test/review"},
  "query":func(s *Snapshot){s.Reference="https://example.test/review?token=synthetic"},
  "fragment":func(s *Snapshot){s.Reference="https://example.test/review#private"},
  "scheme":func(s *Snapshot){s.Reference="http://example.test/review"},
 }{t.Run(name,func(t *testing.T){s:=fixture();mutate(&s);if s.Validate(now)!=ErrInvalid{t.Fatal("invalid review accepted")}})}
 for _,input:=range []string{string(raw)+" {}",strings.Replace(string(raw),`"version":1`,`"version":1,"userId":"spoofed"`,1),"null","",strings.Repeat(" ",(1<<20)+1)}{
  if _,err:=Decode(strings.NewReader(input),now);err!=ErrInvalid{t.Fatal("invalid/oversized envelope accepted")}
 }
}
