package catalogjob

import (
 "testing"
 "time"
)
func TestReviewedProviderConfiguration(t *testing.T){
 c,err:=ParseProvider("tvmaze"," show-101-season-501 ","2h",true)
 if err!=nil||c.Provider!="tvmaze"||len(c.IDs)!=1||c.IDs[0]!="show-101-season-501"||c.Interval!=2*time.Hour{t.Fatal(c,err)}
 c,err=ParseProvider("","Q1","",false)
 if err!=nil||c.Provider!="wikidata"{t.Fatal("legacy default",c,err)}
 c,err=ParseProvider("unknown","","invalid",false)
 if err!=nil||len(c.IDs)!=0{t.Fatal("empty IDs must stay idle",c,err)}
 for _,tc:=range []struct{provider,ids string;reviewed bool}{
 {"tvmaze","show-101-season-501",false},{"tvmaze","Q1",true},{"wikidata","show-101-season-501",true},
 {"unknown","Q1",true},{"tvmaze","show-101-season-501,show-101-season-501",true},
 }{if _,err:=ParseProvider(tc.provider,tc.ids,"1h",tc.reviewed);err==nil{t.Fatal("invalid config",tc)}}
}
