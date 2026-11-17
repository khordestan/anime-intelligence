package catalogidentity

import (
 "encoding/json"
 "strings"
 "testing"
 "time"
)

func retractionFixture()RetractionReview{
 base:=fixture()
 return RetractionReview{Version:1,Provider:base.Provider,ReviewedAt:base.ReviewedAt,Reference:base.Reference,
 Retractions:[]Retraction{{ExternalID:"work-1",AnimeID:base.Mappings[0].AnimeID,ExpectedMappingID:"20000000-0000-4000-8000-000000000001"}}}
}
func TestRetractionStrictManifest(t *testing.T){
 now:=fixture().ReviewedAt
 raw,_:=json.Marshal(retractionFixture())
 if s,err:=DecodeRetractions(strings.NewReader(string(raw)),now);err!=nil||len(s.Retractions)!=1{t.Fatal("retraction roundtrip",err)}
 for _,mutate:=range []func(*RetractionReview){
  func(s *RetractionReview){s.Version=2},
  func(s *RetractionReview){s.ReviewedAt=now.Add(time.Second)},
  func(s *RetractionReview){s.Provider="bad/provider"},
  func(s *RetractionReview){s.Reference="https://example.test/review?secret=synthetic"},
  func(s *RetractionReview){s.Retractions=nil},
  func(s *RetractionReview){s.Retractions[0].AnimeID="bad"},
  func(s *RetractionReview){s.Retractions[0].ExpectedMappingID=""},
  func(s *RetractionReview){s.Retractions[0].ExpectedMappingID="bad"},
  func(s *RetractionReview){s.Retractions=append(s.Retractions,s.Retractions[0])},
 }{
  s:=retractionFixture();mutate(&s)
  if s.Validate(now)==nil{t.Fatal("invalid retraction accepted")}
 }
 for _,input:=range []string{string(raw)+" {}",strings.Replace(string(raw),`"version":1`,`"version":1,"userId":"spoofed"`,1),"null",strings.Repeat("x",(1<<20)+1)}{
  if _,err:=DecodeRetractions(strings.NewReader(input),now);err!=ErrInvalid{t.Fatal("invalid retraction JSON accepted",err)}
 }
}
