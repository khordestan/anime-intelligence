package catalogjob

import "testing"

func TestWorkerImportScopeIsExplicit(t *testing.T){
 for _,scope:=range []string{"","full","schedules"}{
  c,err:=ParseWorker("tvmaze","show-101-season-501","1h",true,scope)
  if err!=nil||c.SchedulesOnly!=(scope=="schedules"){t.Fatal(c,err)}
 }
 if _,err:=ParseWorker("wikidata","Q1","1h",false,"schedules");err==nil{t.Fatal("unsupported schedule capability accepted")}
 if _,err:=ParseWorker("tvmaze","show-101-season-501","1h",true,"unknown");err==nil{t.Fatal("unknown scope accepted")}
 if c,err:=ParseWorker("unknown","","invalid",false,"unknown");err!=nil||len(c.IDs)!=0{t.Fatal("unused scope activated worker",c,err)}
}
