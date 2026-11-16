package postgres_test

import (
 "context"
 "encoding/json"
 "fmt"
 "io"
 "log/slog"
 "net/http/httptest"
 "reflect"
 "strings"
 "testing"
 "time"

 "github.com/khordestan/anime-intelligence/internal/httpapi"
 "github.com/khordestan/anime-intelligence/internal/modules/catalog/feed"
 "github.com/khordestan/anime-intelligence/internal/platform/catalogsync"
 "github.com/khordestan/anime-intelligence/internal/platform/store"
)
func TestPublicContinuitySnapshotDirectionProvenanceAndValidation(t *testing.T){
 db,owner,_,_:=testDB(t);ctx:=context.Background();now:=time.Now().UTC().Truncate(time.Microsecond);zero:=0
 if _,err:=owner.Exec(ctx,`INSERT INTO data_sources(provider_key,display_name,enabled,policy_url) VALUES('graph-fixture','Synthetic graph',true,'https://example.test/policy'),('graph-other','Synthetic reciprocal',true,'https://example.test/policy')`);err!=nil{t.Fatal(err)}
 snapshot:=feed.Snapshot{Version:1,Provider:"graph-fixture",ObservedAt:now,Titles:[]feed.Title{
  {ExternalID:"one",Title:"Synthetic Graph One",MediaType:"TV",Status:"FINISHED",Synopsis:"Synthetic omitted prose",EpisodeCount:&zero,Relations:[]feed.Relation{{ExternalID:"two",Type:"SEQUEL"},{ExternalID:"side",Type:"SIDE_STORY"}}},
  {ExternalID:"two",Title:"Synthetic Graph Two",MediaType:"MOVIE",Status:"ANNOUNCED",Relations:[]feed.Relation{{ExternalID:"one",Type:"PREQUEL"},{ExternalID:"three",Type:"SEQUEL"}}},
  {ExternalID:"three",Title:"Synthetic Graph Three",MediaType:"TV",Status:"UNKNOWN"},
  {ExternalID:"side",Title:"Synthetic Excluded Side Story",MediaType:"OVA",Status:"UNKNOWN"},
 }}
 if _,err:=catalogsync.Import(ctx,owner,snapshot,now);err!=nil{t.Fatal(err)}
 ids:=map[string]string{}
 for _,key:=range []string{"one","two","three","side"}{var id string;if err:=owner.QueryRow(ctx,`SELECT anime_id::text FROM anime_external_ids WHERE provider='graph-fixture' AND external_id=$1`,key).Scan(&id);err!=nil{t.Fatal(err)};ids[key]=id}
 if _,err:=owner.Exec(ctx,`UPDATE anime_relations SET source_id=(SELECT id FROM data_sources WHERE provider_key='graph-other') WHERE source_anime_id=$1 AND relation_type='PREQUEL'`,ids["two"]);err!=nil{t.Fatal(err)}
 graph,found,err:=db.Repository("").CatalogContinuity(ctx,strings.ToUpper(ids["one"]))
 if err!=nil||!found||graph.RootID!=ids["one"]||graph.Truncated||graph.Limit!=50||len(graph.Nodes)!=3||len(graph.Edges)!=2{t.Fatal("public linked component",graph,err)}
 for i,n:=range graph.Nodes{
  if n.Distance!=i||n.ID!=ids[[]string{"one","two","three"}[i]]||n.Provider!="graph-fixture"||n.ObservedAt==nil||!n.ObservedAt.Equal(now){t.Fatal("distance/source snapshot",n)}
 }
 if !graph.Nodes[0].EpisodeCountKnown||graph.Nodes[0].EpisodeCount!=0||graph.Nodes[1].EpisodeCountKnown{t.Fatal("unknown and known-zero totals conflated")}
 pairs:=map[string]store.ContinuityEdge{}
 for _,edge:=range graph.Edges{pairs[edge.FromID+":"+edge.ToID]=edge}
 if e:=pairs[ids["one"]+":"+ids["two"]];!reflect.DeepEqual(e.Providers,[]string{"graph-fixture","graph-other"}){t.Fatal("reciprocal direction/deduplicated provenance",graph.Edges)}
 if e:=pairs[ids["two"]+":"+ids["three"]];!reflect.DeepEqual(e.Providers,[]string{"graph-fixture"}){t.Fatal("transitive direction",graph.Edges)}
 reverse,ok,err:=db.Repository("").CatalogContinuity(ctx,ids["three"])
 if err!=nil||!ok||len(reverse.Nodes)!=3||reverse.Nodes[2].ID!=ids["one"]||reverse.Nodes[2].Distance!=2{t.Fatal("incoming/transitive links lost",reverse,err)}
 h:=httpapi.NewPostgresRouter(slog.New(slog.NewTextHandler(io.Discard,nil)),db)
 w:=httptest.NewRecorder();h.ServeHTTP(w,httptest.NewRequest("GET","/api/v1/anime/"+ids["one"]+"/continuity",nil))
 if w.Code!=200||w.Header().Get("Cache-Control")!="no-store"{t.Fatal("public route boundary",w.Code)}
 if strings.Contains(w.Body.String(),"Synthetic omitted prose")||strings.Contains(w.Body.String(),"Synthetic Excluded Side Story")||strings.Contains(w.Body.String(),"synopsis"){t.Fatal("unrelated or prose fields leaked")}
 var decoded store.Continuity
 if json.Unmarshal(w.Body.Bytes(),&decoded)!=nil||len(decoded.Nodes)!=3{t.Fatal("graph JSON response")}
 for path,want:=range map[string]int{"not-a-uuid":400,"00000000-0000-4000-8000-000000000000":404}{
  w:=httptest.NewRecorder();h.ServeHTTP(w,httptest.NewRequest("GET","/api/v1/anime/"+path+"/continuity",nil));if w.Code!=want{t.Fatal(path,w.Code)}
 }
 seed,ok,err:=db.Repository("").CatalogContinuity(ctx,"10000000-0000-4000-8000-000000000001")
 if err!=nil||!ok||len(seed.Nodes)!=1||seed.Edges==nil||len(seed.Edges)!=0||seed.Nodes[0].Provider!=""{t.Fatal("unlinked starter state",seed,err)}
 cancelled,cancel:=context.WithCancel(ctx);cancel()
 if _,_,err=db.Repository("").CatalogContinuity(cancelled,ids["one"]);err==nil{t.Fatal("cancelled query succeeded")}
}
func TestContinuityExactCapacityOverflowAndDenseEdges(t *testing.T){
 db,owner,_,_:=testDB(t);ctx:=context.Background()
 if _,err:=owner.Exec(ctx,`INSERT INTO anime(id,canonical_title) SELECT ('50000000-0000-4000-8000-'||lpad(i::text,12,'0'))::uuid,'Synthetic Chain '||i FROM generate_series(1,50) i;
 INSERT INTO anime_relations(source_anime_id,target_anime_id,relation_type)
 SELECT ('50000000-0000-4000-8000-'||lpad(i::text,12,'0'))::uuid,('50000000-0000-4000-8000-'||lpad((i+1)::text,12,'0'))::uuid,'SEQUEL' FROM generate_series(1,49) i`);err!=nil{t.Fatal(err)}
 const root="50000000-0000-4000-8000-000000000001"
 graph,ok,err:=db.Repository("").CatalogContinuity(ctx,root)
 if err!=nil||!ok||len(graph.Nodes)!=50||len(graph.Edges)!=49||graph.Truncated||graph.Nodes[49].Distance!=49{t.Fatal("exact limit must not imply overflow",graph,err)}
 if _,err=owner.Exec(ctx,`INSERT INTO anime(id,canonical_title) VALUES('50000000-0000-4000-8000-000000000051','Synthetic Outside Capacity');
 INSERT INTO anime_relations(source_anime_id,target_anime_id,relation_type) VALUES('50000000-0000-4000-8000-000000000050','50000000-0000-4000-8000-000000000051','SEQUEL')`);err!=nil{t.Fatal(err)}
 graph,ok,err=db.Repository("").CatalogContinuity(ctx,root)
 if err!=nil||!ok||len(graph.Nodes)!=50||len(graph.Edges)!=49||!graph.Truncated{t.Fatal("overflow boundary not detected",graph,err)}
 if _,err=owner.Exec(ctx,`DELETE FROM anime_relations WHERE source_anime_id::text LIKE '50000000-%';
 INSERT INTO anime_relations(source_anime_id,target_anime_id,relation_type)
 SELECT ('50000000-0000-4000-8000-'||lpad(i::text,12,'0'))::uuid,('50000000-0000-4000-8000-'||lpad(j::text,12,'0'))::uuid,'SEQUEL'
 FROM generate_series(1,50) i CROSS JOIN generate_series(1,50) j WHERE i<j`);err!=nil{t.Fatal(err)}
 graph,ok,err=db.Repository("").CatalogContinuity(ctx,root)
 if err!=nil||!ok||len(graph.Nodes)!=50||len(graph.Edges)!=1225||graph.Truncated{t.Fatal("dense in-bound graph lost edges",len(graph.Nodes),len(graph.Edges),graph.Truncated,err)}
 for i,n:=range graph.Nodes{want:=fmt.Sprintf("50000000-0000-4000-8000-%012d",i+1);if n.ID!=want||(i>0&&n.Distance!=1){t.Fatal("stable same-distance ordering",n)}}
 // Defensive traversal is finite even if a manual write bypasses DAG validation.
 if _,err=owner.Exec(ctx,`INSERT INTO anime_relations(source_anime_id,target_anime_id,relation_type) VALUES('50000000-0000-4000-8000-000000000050','50000000-0000-4000-8000-000000000001','SEQUEL')`);err!=nil{t.Fatal(err)}
 if graph,ok,err=db.Repository("").CatalogContinuity(ctx,root);err!=nil||!ok||len(graph.Nodes)!=50||len(graph.Edges)!=1226||graph.Truncated{t.Fatal("manual cyclic data caused unbounded traversal",err)}
}
