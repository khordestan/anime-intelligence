package catalogsync

import (
 "encoding/json"
 "strings"
 "testing"
 "time"
)

func validReviewFixture()FieldReview{
 zero:=0
 at:=time.Date(2026,1,2,0,0,0,0,time.UTC)
 return FieldReview{Version:1,ReviewedAt:at,Reference:"https://example.test/review",
 Selections:[]FieldSelection{{AnimeID:"10000000-0000-4000-8000-000000000001",Field:"genres",Mode:"SELECT",
 ExpectedRevision:&zero,Provider:"synthetic",ExternalID:"one",Values:[]string{"Adventure"},ObservedAt:at.Add(-time.Hour)}}}
}
func TestFieldReviewStrictDecodeAndBounds(t *testing.T){
 now:=time.Date(2026,1,3,0,0,0,0,time.UTC)
 s:=validReviewFixture()
 for _,field:=range []string{"genres","studios","synonyms","themes","demographics","tags"}{
  s.Selections[0].Field=field
  if s.Validate(now)!=nil{t.Fatal("supported label field rejected",field)}
 }
 s=validReviewFixture();s.Selections[0].Values=[]string{}
 raw,_:=json.Marshal(s)
 if decoded,err:=DecodeFieldReview(strings.NewReader(string(raw)),now);err!=nil||decoded.Selections[0].Values==nil{t.Fatal("explicit empty choice lost",err)}
 for _,mutate:=range []func(*FieldReview){
  func(s *FieldReview){s.Version=2},func(s *FieldReview){s.ReviewedAt=now.Add(time.Hour)},
  func(s *FieldReview){s.Reference="https://name:secret@example.test/review"},
  func(s *FieldReview){s.Reference="https://example.test/review?token=secret"},
  func(s *FieldReview){s.Reference="http://example.test/review"},
  func(s *FieldReview){s.Selections[0].Field="synopsis"},
  func(s *FieldReview){s.Selections[0].Field="episodeCount"},
  func(s *FieldReview){s.Selections[0].AnimeID="invalid"},
  func(s *FieldReview){s.Selections[0].ExpectedRevision=nil},
  func(s *FieldReview){n:=-1;s.Selections[0].ExpectedRevision=&n},
  func(s *FieldReview){s.Selections[0].ObservedAt=now.Add(time.Hour)},
  func(s *FieldReview){s.Selections[0].Values=nil},
  func(s *FieldReview){s.Selections[0].Values=[]string{"Adventure","adventure"}},
  func(s *FieldReview){s.Selections[0].Values=[]string{" padded "}},
  func(s *FieldReview){s.Selections[0].Values=[]string{"bad\x00label"}},
  func(s *FieldReview){s.Selections=append(s.Selections,s.Selections[0])},
  func(s *FieldReview){s.Selections[0].Mode="UNKNOWN"},
  func(s *FieldReview){s.Selections[0].Mode="BASE"},
 }{
  value:=validReviewFixture();mutate(&value)
  if value.Validate(now)==nil{t.Fatal("invalid review accepted")}
 }
 s=validReviewFixture();s.Selections[0].Mode="BASE";s.Selections[0].Provider="";s.Selections[0].ExternalID=""
 s.Selections[0].Values=nil;s.Selections[0].ObservedAt=time.Time{}
 if s.Validate(now)!=nil{t.Fatal("base restoration rejected")}
 raw,_=json.Marshal(validReviewFixture())
 for _,input:=range []string{string(raw)+" {}",strings.Replace(string(raw),"\"version\":1","\"unexpected\":true,\"version\":1",1),strings.Repeat("x",(1<<20)+1)}{
  if _,err:=DecodeFieldReview(strings.NewReader(input),now);err==nil{t.Fatal("malformed input accepted")}
 }
}
