package catalogsync

import(
 "encoding/json"
 "errors"
 "reflect"
 "strings"
 "testing"
 "time"

 "github.com/khordestan/anime-intelligence/internal/modules/catalog/feed"
)
func TestHistoryFieldChanges(t *testing.T){
 zero:=0
 before:=feed.Title{ExternalID:"one",Title:"Synthetic unchanged",MediaType:"TV",Status:"UNKNOWN"}
 after:=before;after.EpisodeCount=&zero;after.Themes=[]string{"Synthetic theme"};after.Synopsis="Synthetic private-looking source prose"
 want:=[]string{"episodeCount","synopsis","themes"}
 if got:=historyChanges(after,before);!reflect.DeepEqual(got,want){t.Fatal(got)}
 if got:=historyChanges(after,after);len(got)!=0{t.Fatal(got)}
 copy:=after;copy.ExternalID="ignored"
 if got:=historyChanges(after,copy);len(got)!=0{t.Fatal(got)}
 before.Synonyms=[]string{} // Empty and absent optional lists carry no value.
 if got:=historyChanges(before,feed.Title{ExternalID:"one",Title:"Synthetic unchanged",MediaType:"TV",Status:"UNKNOWN"});len(got)!=0{t.Fatal(got)}
}
func TestHistoryEnvelopeValidationAndSafeSummary(t *testing.T){
 at:=time.Date(2026,1,2,3,4,5,0,time.FixedZone("fixture",3600))
 r:=metadataArchive{Kind:metadataRecordType,AnimeID:"10000000-0000-4000-8000-000000000001",Provider:"fixture",ObservedAt:at,Title:feed.Title{ExternalID:"one",Title:"Synthetic hidden title",Synopsis:"Synthetic hidden prose",MediaType:"TV",Status:"UNKNOWN"}}
 payload,_:=json.Marshal(r)
 got,err:=decodeHistory(payload,"fixture","one")
 if err!=nil||got.ObservedAt.Location()!=time.UTC||!got.ObservedAt.Equal(at){t.Fatal(got,err)}
 for _,request:=range [][2]string{{"other","one"},{"fixture","other"}}{if _,err=decodeHistory(payload,request[0],request[1]);!errors.Is(err,ErrHistory){t.Fatal(err)}}
 for _,bad:=range []string{"invalid","{}"}{if _,err=decodeHistory([]byte(bad),"fixture","one");!errors.Is(err,ErrHistory){t.Fatal(err)}}
 summary,_:=json.Marshal(HistoryResult{Entries:[]HistoryEntry{{ObservedAt:at,ChangedFields:[]string{"synopsis"}}}})
 if strings.Contains(string(summary),"Synthetic")||strings.Contains(string(summary),"synthetic-id"){t.Fatal("summary leaked values")}
 for _,id:=range []string{"","has space","slash/id",strings.Repeat("a",129)}{if historyIdentifier(id){t.Fatal("invalid identifier accepted")}}
 if !historyIdentifier("fixture-A_1.2"){t.Fatal("valid identifier rejected")}
}

func TestHistoryRejectsInvalidArchivedFacts(t *testing.T){
 at:=time.Date(2026,1,1,0,0,0,0,time.UTC)
 valid:=metadataArchive{Kind:metadataRecordType,AnimeID:"10000000-0000-4000-8000-000000000001",Provider:"fixture",ObservedAt:at,Title:feed.Title{ExternalID:"one",Title:"Synthetic",MediaType:"TV",Status:"UNKNOWN"}}
 future:=at.Add(time.Hour)
 for _,mutate:=range []func(*metadataArchive){
 func(r *metadataArchive){r.AnimeID="invalid"},
 func(r *metadataArchive){r.Title.Status="INVALID"},
 func(r *metadataArchive){r.Title.Title=""},
 func(r *metadataArchive){r.Title.Episodes=[]feed.Episode{{Number:1,Track:"SUB",ReleasedAt:&future}}},
 func(r *metadataArchive){r.Title.Relations=[]feed.Relation{{ExternalID:"one",Type:"SEQUEL"}}},
 }{
 r:=valid;mutate(&r);payload,_:=json.Marshal(r)
 if _,err:=decodeHistory(payload,"fixture","one");!errors.Is(err,ErrHistory){t.Fatal("invalid archive facts accepted")}
 }
}
