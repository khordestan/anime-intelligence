package postgres

import (
 "encoding/base64"
 "strings"
 "testing"
 "time"
)

func TestNotificationCursorBoundaries(t *testing.T){
 n:=Notification{ID:"10000000-0000-4000-8000-000000000001",CreatedAt:time.Date(2026,1,2,3,4,5,123456000,time.FixedZone("fixture",3600))}
 stamp,id,err:=parseNotificationCursor(notificationCursor(n))
 if err!=nil||stamp==nil||!stamp.Equal(n.CreatedAt)||id==nil||*id!=n.ID{t.Fatal("cursor round trip",err)}
 stamp,id,err=parseNotificationCursor("")
 if err!=nil||stamp!=nil||id!=nil{t.Fatal("empty boundary",err)}
 encode:=func(s string)string{return base64.RawURLEncoding.EncodeToString([]byte(s))}
 for _,input:=range []string{"?",strings.Repeat("x",257),encode("2026-01-01T00:00:00Z"),encode("2026-01-01T00:00:00Z|invalid"),encode("bad|"+n.ID),encode("0001-01-01T00:00:00Z|"+n.ID),encode("2026-01-01T00:00:00Z|10000000-0000-4000-8000-00000000000z"),encode("2026-01-01T00:00:00Z|"+n.ID+"|extra")}{
  if _,_,err=parseNotificationCursor(input);err!=ErrNotificationCursor{t.Fatal("invalid cursor accepted")}
 }
}
