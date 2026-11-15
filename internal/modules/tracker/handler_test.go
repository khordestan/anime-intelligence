package tracker
import (
 "net/http/httptest"
 "strings"
 "testing"
)
func TestValidStatus(t *testing.T) {
 for _,s:=range []string{"PLANNING","WATCHING","COMPLETED","ON_HOLD","DROPPED","watching"}{if !validStatus(s){t.Errorf("expected %q valid",s)}}
 for _,s:=range []string{"","FINISHED","PAUSED","UNKNOWN"}{if validStatus(s){t.Errorf("expected %q invalid",s)}}
}

func TestValidUUID(t *testing.T) {
 valid := "10000000-0000-4000-8000-000000000001"
 if !validUUID(valid) { t.Fatalf("expected valid UUID to pass") }
 for _, value := range []string{
  "not-a-uuid-at-all-000000000000000000",
  "zzzzzzzz-0000-4000-8000-000000000001",
  "10000000-0000-4000-8000-00000000000g",
  "10000000000040008000000000000001",
 } {
  if validUUID(value) { t.Errorf("expected invalid UUID %q to fail", value) }
 }
}

func TestDecodeRejectsTrailingJSON(t *testing.T) {
 req := httptest.NewRequest("POST", "/", strings.NewReader(`{"animeId":"x"} {"unexpected":true}`))
 if err := decode(httptest.NewRecorder(), req, &createRequest{}); err == nil {
  t.Fatal("expected trailing JSON value to be rejected")
 }
}
func TestDecodeRejectsUnknownFields(t *testing.T) {
 req := httptest.NewRequest("POST", "/", strings.NewReader(`{"animeId":"x","unknown":true}`))
 if err := decode(httptest.NewRecorder(), req, &createRequest{}); err == nil {
  t.Fatal("expected unknown field to be rejected")
 }
}
