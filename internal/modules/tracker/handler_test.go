package tracker
import "testing"
func TestValidStatus(t *testing.T) {
 for _,s:=range []string{"PLANNING","WATCHING","COMPLETED","ON_HOLD","DROPPED","watching"}{if !validStatus(s){t.Errorf("expected %q valid",s)}}
 for _,s:=range []string{"","FINISHED","PAUSED","UNKNOWN"}{if validStatus(s){t.Errorf("expected %q invalid",s)}}
}
