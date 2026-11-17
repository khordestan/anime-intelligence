package catalogsync

import (
 "encoding/json"
 "bytes"
 "testing"
 "time"
)

func TestScalarChoiceTypesAndUnknownSemantics(t *testing.T){
 now:=time.Date(2026,1,3,0,0,0,0,time.UTC)
 for _,tc:=range []struct{field,raw string}{
 {"releaseYear","0"},{"releaseYear","1900"},{"releaseYear","9999"},
 {"season","\"\""},{"season","\"SPRING\""},{"mediaType","\"TV\""},{"mediaType","\"UNKNOWN\""},
 {"episodeCount","null"},{"episodeCount","0"},{"episodeCount","100000"},
 }{
  s:=validReviewFixture();s.Selections[0].Field=tc.field;s.Selections[0].Values=nil;s.Selections[0].Scalar=json.RawMessage(tc.raw)
  if s.Validate(now)!=nil{t.Fatal("valid scalar rejected",tc)}
  raw,_:=json.Marshal(s)
  if _,err:=DecodeFieldReview(bytes.NewReader(raw),now);err!=nil{t.Fatal("scalar decode",tc,err)}
 }
 for _,tc:=range []struct{field,raw string}{
 {"releaseYear","null"},{"releaseYear","1899"},{"releaseYear","10000"},{"releaseYear","2026.0"},
 {"episodeCount","-1"},{"episodeCount","100001"},{"episodeCount","true"},{"episodeCount","\"12\""},
 {"mediaType","null"},{"mediaType","\"Anime\""},{"season","null"},{"season","\"AUTUMN\""},
 }{
  s:=validReviewFixture();s.Selections[0].Field=tc.field;s.Selections[0].Values=nil;s.Selections[0].Scalar=json.RawMessage(tc.raw)
  if s.Validate(now)==nil{t.Fatal("invalid scalar accepted",tc)}
 }
 s:=validReviewFixture();s.Selections[0].Scalar=json.RawMessage("12")
 if s.Validate(now)==nil{t.Fatal("mixed label/scalar accepted")}
 s=validReviewFixture();s.Selections[0].Field="episodeCount";s.Selections[0].Scalar=json.RawMessage("12")
 if s.Validate(now)==nil{t.Fatal("scalar with label values accepted")}
}
