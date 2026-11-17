package catalogsync

import (
 "encoding/json"
 "strings"
 "testing"
 "time"
)

func TestTextChoicesStrictTypesAndBounds(t *testing.T){
 now:=time.Date(2026,1,3,0,0,0,0,time.UTC)
 valid:=[]struct{field,value string}{
 {"title","Synthetic reviewed name"},{"title","架空の作品"},{"nativeTitle",""},{"synopsis",""},
 {"synopsis","First paragraph.\nSecond paragraph.\tDetails."},
 {"title",strings.Repeat("a",1024)},{"synopsis",strings.Repeat("a",65536)},
 }
 for _,status:=range []string{"UNKNOWN","ANNOUNCED","RELEASING","FINISHED","CANCELLED","HIATUS"}{
  valid=append(valid,struct{field,value string}{"status",status})
 }
 for _,tc:=range valid{
  s:=validReviewFixture();s.Selections[0].Field=tc.field;s.Selections[0].Values=nil
  s.Selections[0].Scalar,_=json.Marshal(tc.value)
  if s.Validate(now)!=nil{t.Fatal("valid text rejected",tc.field)}
  raw,_:=json.Marshal(s)
  if _,err:=DecodeFieldReview(strings.NewReader(string(raw)),now);err!=nil{t.Fatal("text decode",tc.field,err)}
 }
 for _,tc:=range []struct{field,value string}{
 {"title",""},{"title"," padded"},{"title","line\nbreak"},{"nativeTitle","name\t"},{"nativeTitle","bad\x00name"},
 {"synopsis","bad\x00prose"},{"synopsis","bad\x7fprose"},{"status","finished"},
 {"title",strings.Repeat("a",1025)},{"nativeTitle",strings.Repeat("a",1025)},{"synopsis",strings.Repeat("a",65537)},
 }{
  raw,_:=json.Marshal(tc.value)
  if _,ok:=scalarValue(tc.field,raw);ok{t.Fatal("invalid text accepted",tc.field)}
 }
 for _,field:=range []string{"title","nativeTitle","synopsis","status"}{
  for _,raw:=range []string{"null","12","true","[]","{}"}{
   if _,ok:=scalarValue(field,json.RawMessage(raw));ok{t.Fatal("wrong text type",field,raw)}
  }
 }
 if _,ok:=scalarValue("title",json.RawMessage{'"',0xff,'"'});ok{t.Fatal("invalid UTF-8 accepted")}
}
