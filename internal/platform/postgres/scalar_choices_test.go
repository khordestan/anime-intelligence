package postgres_test

import (
 "context"
 "encoding/json"
 "errors"
 "os"
 "testing"
 "time"

 "github.com/khordestan/anime-intelligence/internal/platform/catalogsync"
 "github.com/khordestan/anime-intelligence/internal/platform/postgres"
 "github.com/khordestan/anime-intelligence/internal/platform/store"
 "github.com/khordestan/anime-intelligence/internal/modules/catalog/feed"
)

func scalarReview(id,field,raw string,revision int,at time.Time)catalogsync.FieldReview{
 s:=labelReview(id,field,nil,revision,at)
 s.Selections[0].Scalar=json.RawMessage(raw)
 return s
}
func TestScalarChoicesPreviewRefreshRestoreAndPublicFilters(t *testing.T){
 owner,writer,curator,_,runtimeDSN:=fieldReviewDB(t)
 ctx:=context.Background()
 id,primary,base:=fieldFixture(t,owner)
 review:=scalarReview(id,"releaseYear","2028",0,base.Add(time.Hour))
 for _,tc:=range []struct{field,value string}{{"season","\"SPRING\""},{"mediaType","\"MOVIE\""},{"episodeCount","12"}}{
  value:=scalarReview(id,tc.field,tc.value,0,review.ReviewedAt).Selections[0];review.Selections=append(review.Selections,value)
 }
 if result,err:=catalogsync.ReviewFields(ctx,curator,review,time.Now(),false);err!=nil||result.Applied||result.Selected!=4{t.Fatal("scalar preview",result,err)}
 db,err:=postgres.Open(ctx,runtimeDSN);if err!=nil{t.Fatal(err)};defer db.Close()
 repo:=db.Repository("")
 details,_,err:=repo.CatalogDetails(ctx,id)
 if err!=nil||details.Anime.MediaType!="TV"||details.Anime.ReleaseYear!=0||details.Anime.EpisodeCountKnown{t.Fatal("preview changed canonical data",details,err)}
 if _,err=catalogsync.ReviewFields(ctx,curator,review,time.Now(),true);err!=nil{t.Fatal(err)}
 details,_,err=repo.CatalogDetails(ctx,id)
 if err!=nil||details.Anime.MediaType!="MOVIE"||details.Anime.ReleaseYear!=2028||details.Anime.Season!="SPRING"||!details.Anime.EpisodeCountKnown||details.Anime.EpisodeCount!=12||len(details.FieldSources)!=4{t.Fatal("scalar projection",details,err)}
 count:=12
 matches,err:=repo.FilterCatalog(ctx,store.CatalogFilter{MediaType:"MOVIE",ReleaseYear:2028,Season:"SPRING",MinEpisodes:&count,MaxEpisodes:&count,Sort:"NEWEST"},10,0)
 if err!=nil||len(matches)!=1||matches[0].ID!=id{t.Fatal("scalar filters diverged",matches,err)}
 primary.ObservedAt=base.Add(2*time.Hour)
 primary.Titles[0].ReleaseYear=2027;primary.Titles[0].Season="FALL"
 total:=8;primary.Titles[0].EpisodeCount=&total
 if _,err=catalogsync.Import(ctx,writer,primary,time.Now());err!=nil{t.Fatal("refresh",err)}
 details,_,err=repo.CatalogDetails(ctx,id)
 if err!=nil||details.Anime.ReleaseYear!=2028||details.Anime.EpisodeCount!=12||details.Anime.MediaType!="MOVIE"{t.Fatal("refresh erased scalar choice",details,err)}
 restore:=catalogsync.FieldReview{Version:1,ReviewedAt:base.Add(3*time.Hour),Reference:"https://example.test/scalar-restore"}
 one:=1
 for _,field:=range []string{"releaseYear","season","mediaType","episodeCount"}{
  restore.Selections=append(restore.Selections,catalogsync.FieldSelection{AnimeID:id,Field:field,Mode:"BASE",ExpectedRevision:&one})
 }
 if result,err:=catalogsync.ReviewFields(ctx,curator,restore,time.Now(),true);err!=nil||result.Restored!=4{t.Fatal("scalar restore",result,err)}
 details,_,err=repo.CatalogDetails(ctx,id)
 if err!=nil||details.Anime.ReleaseYear!=2027||details.Anime.Season!="FALL"||details.Anime.MediaType!="TV"||details.Anime.EpisodeCount!=8{t.Fatal("restore used stale primary scalars",details,err)}
 if _,err=catalogsync.ReviewFields(ctx,curator,review,time.Now(),true);!errors.Is(err,catalogsync.ErrFieldConflict){t.Fatal("stale scalar decision resurrected",err)}
 unknown:=scalarReview(id,"episodeCount","null",2,base.Add(4*time.Hour))
 if _,err=catalogsync.ReviewFields(ctx,curator,unknown,time.Now(),true);err!=nil{t.Fatal("unknown total",err)}
 details,_,err=repo.CatalogDetails(ctx,id)
 if err!=nil||details.Anime.EpisodeCountKnown{t.Fatal("null became known zero",details,err)}
 zero:=scalarReview(id,"episodeCount","0",3,base.Add(5*time.Hour))
 if _,err=catalogsync.ReviewFields(ctx,curator,zero,time.Now(),true);err!=nil{t.Fatal("known zero",err)}
 details,_,err=repo.CatalogDetails(ctx,id)
 if err!=nil||!details.Anime.EpisodeCountKnown||details.Anime.EpisodeCount!=0{t.Fatal("known zero lost",details,err)}
 zero.Selections[0].Scalar=json.RawMessage(" 0 ")
 if result,err:=catalogsync.ReviewFields(ctx,curator,zero,time.Now(),true);err!=nil||result.Unchanged!=1||result.AuditRecords!=0{t.Fatal("scalar replay normalization",result,err)}
 // Changing factual classifications emits no release or season announcement.
 var events int
 if err=owner.QueryRow(ctx,`SELECT count(*) FROM domain_events WHERE aggregate_id=$1`,id).Scan(&events);err!=nil||events!=0{t.Fatal("curation inferred release events",events,err)}
}
func TestScalarTotalsRespectRetainedEpisodesAndBatchAtomicity(t *testing.T){
 owner,writer,curator,_,runtimeDSN:=fieldReviewDB(t)
 ctx:=context.Background()
 id,primary,base:=fieldFixture(t,owner)
 primary.ObservedAt=base.Add(time.Hour)
 primary.Titles[0].Episodes=[]feed.Episode{{Number:2,Track:"UNKNOWN",ScheduledAt:&base}}
 if _,err:=catalogsync.Import(ctx,writer,primary,time.Now());err!=nil{t.Fatal(err)}
 bad:=scalarReview(id,"episodeCount","1",0,base.Add(2*time.Hour))
 if _,err:=catalogsync.ReviewFields(ctx,curator,bad,time.Now(),true);!errors.Is(err,catalogsync.ErrFieldConflict){t.Fatal("current base episode invalidated",err)}
 // Partial source updates can omit episodes, but the revision history remains.
 primary.ObservedAt=base.Add(3*time.Hour);primary.Titles[0].Episodes=nil
 if _,err:=catalogsync.Import(ctx,writer,primary,time.Now());err!=nil{t.Fatal(err)}
 batch:=scalarReview(id,"releaseYear","2028",0,base.Add(4*time.Hour))
 batch.Selections=append(batch.Selections,scalarReview(id,"episodeCount","1",0,batch.ReviewedAt).Selections[0])
 if _,err:=catalogsync.ReviewFields(ctx,curator,batch,time.Now(),true);!errors.Is(err,catalogsync.ErrFieldConflict){t.Fatal("retained episode invalidated",err)}
 var year *int;var n int
 if err:=owner.QueryRow(ctx,`SELECT release_year FROM anime WHERE id=$1`,id).Scan(&year);err!=nil||year!=nil{t.Fatal("late conflict retained canonical write",year,err)}
 if err:=owner.QueryRow(ctx,`SELECT count(*) FROM catalog_field_choices`).Scan(&n);err!=nil||n!=0{t.Fatal("late scalar conflict retained choices",n,err)}
 good:=scalarReview(id,"episodeCount","2",0,base.Add(5*time.Hour))
 if _,err:=catalogsync.ReviewFields(ctx,curator,good,time.Now(),true);err!=nil{t.Fatal(err)}
 primary.ObservedAt=base.Add(6*time.Hour);primary.Titles[0].Episodes=[]feed.Episode{{Number:3,Track:"UNKNOWN",ScheduledAt:&base}}
 if _,err:=catalogsync.Import(ctx,writer,primary,time.Now());!errors.Is(err,catalogsync.ErrConflict){t.Fatal("refresh bypassed selected count",err)}
 if err:=owner.QueryRow(ctx,`SELECT count(*) FROM airing_schedule WHERE anime_id=$1 AND episode_number=3`,id).Scan(&n);err!=nil||n!=0{t.Fatal("failed refresh retained new episode",n,err)}
 // A smaller primary observation is retained while the valid reviewed total
 // stays resolved. Restoring that smaller value must still fail closed.
 primary.ObservedAt=base.Add(7*time.Hour);primary.Titles[0].Episodes=nil;small:=1;primary.Titles[0].EpisodeCount=&small
 if _,err:=catalogsync.Import(ctx,writer,primary,time.Now());err!=nil{t.Fatal("resolved total not honored",err)}
 revision:=1
 restore:=catalogsync.FieldReview{Version:1,ReviewedAt:base.Add(8*time.Hour),Reference:"https://example.test/count-restore",
 Selections:[]catalogsync.FieldSelection{{AnimeID:id,Field:"episodeCount",Mode:"BASE",ExpectedRevision:&revision}}}
 if _,err:=catalogsync.ReviewFields(ctx,curator,restore,time.Now(),true);!errors.Is(err,catalogsync.ErrFieldConflict){t.Fatal("restore invalidated retained history",err)}
 db,err:=postgres.Open(ctx,runtimeDSN);if err!=nil{t.Fatal(err)};defer db.Close()
 details,_,err:=db.Repository("").CatalogDetails(ctx,id)
 if err!=nil||details.Anime.EpisodeCount!=2||!details.Anime.EpisodeCountKnown{t.Fatal("failed restore changed resolved count",details,err)}
 for _,query:=range []string{`UPDATE anime SET cover_image_url='Forbidden'`,`UPDATE watch_progress SET episodes_watched=0`}{
  if _,err=curator.Exec(ctx,query);err==nil{t.Fatal("scalar curator exceeded boundary",query)}
 }
 down,err:=os.ReadFile("../../../migrations/000020_catalog_scalar_choices.down.sql");if err!=nil{t.Fatal(err)}
 if _,err=owner.Exec(ctx,string(down));err==nil{t.Fatal("downgrade erased scalar decisions")}
}
