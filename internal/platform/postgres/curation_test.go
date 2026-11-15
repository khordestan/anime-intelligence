package postgres_test

import (
	"context"
	"errors"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/khordestan/anime-intelligence/internal/platform/curation"
	"github.com/khordestan/anime-intelligence/internal/platform/catalogsync"
)
func TestCuratedImportAtomicIdempotentAndRecommendationReady(t *testing.T){
	db,owner,_,runtimeDSN:=testDB(t)
	ctx:=context.Background()
	if _,err:=owner.Exec(ctx,`INSERT INTO anime_features(feature_key,display_name,category) VALUES('synthetic.reviewed','Synthetic reviewed feature','SYNTHETIC')`);err!=nil{t.Fatal(err)}
	now:=time.Now().UTC().Truncate(time.Microsecond)
	value,confidence:=0.7500000000000001,1.0
	s:=curation.Snapshot{Version:1,ObservedAt:now,Records:[]curation.Record{{AnimeID:"10000000-0000-4000-8000-000000000001",FeatureKey:"synthetic.reviewed",Value:&value,Confidence:&confidence,Reference:"https://example.test/synthetic-evidence"}}}
	for i:=0;i<2;i++{
		result,err:=curation.Apply(ctx,owner,s,now)
		if err!=nil||(i==0&&result.Inserted!=1)||(i==1&&result.Unchanged!=1){t.Fatal("idempotent curation",result,err)}
	}
	changed:=0.5
	conflict:=s;conflict.Records=append([]curation.Record{},s.Records...);conflict.Records[0].Value=&changed
	if _,err:=curation.Apply(ctx,owner,conflict,now);!errors.Is(err,curation.ErrConflict){t.Fatal("same-time conflict overwritten",err)}
	s.ObservedAt=now.Add(-time.Hour)
	s.Records=append(s.Records,curation.Record{AnimeID:"10000000-0000-4000-8000-000000000002",FeatureKey:"missing.feature",Value:&value,Confidence:&confidence,Reference:"https://example.test/fixture"})
	if _,err:=curation.Apply(ctx,owner,s,now);err==nil{t.Fatal("invalid batch committed")}
	var count int
	if err:=owner.QueryRow(ctx,`SELECT count(*) FROM anime_feature_observations o JOIN anime_features f ON f.id=o.feature_id WHERE f.feature_key='synthetic.reviewed'`).Scan(&count);err!=nil||count!=1{t.Fatal("curation rollback/history",count,err)}
	raw,err:=pgx.Connect(ctx,runtimeDSN)
	if err!=nil{t.Fatal(err)}
	defer raw.Close(ctx)
	s.Records=s.Records[:1]
	if _,err=curation.Apply(ctx,raw,s,now);!errors.Is(err,curation.ErrImport){t.Fatal("API role curated public evidence",err)}
	account,err:=db.Register(ctx,"curation-match@example.test","synthetic-curation-password")
	if err!=nil{t.Fatal(err)}
	verifyAccount(t,db,account.Token)
	weight:=1.0
	if _,err=db.Repository(account.Token).SetFeaturePreference(ctx,"synthetic.reviewed",&weight);err!=nil{t.Fatal(err)}
	page,err:=db.Repository(account.Token).PersonalMatches(ctx,20)
	if err!=nil||len(page.Items)!=1||page.Items[0].AnimeID!=s.Records[0].AnimeID{t.Fatal("reviewed observation not used",page,err)}
}

func TestDedicatedCuratorCanAppendWithoutPrivateOrRewriteAccess(t *testing.T){
	_,owner,_,runtimeDSN:=testDB(t)
	ctx:=context.Background()
	u,err:=url.Parse(runtimeDSN);if err!=nil{t.Fatal(err)}
	role:=u.User.Username()+"_curator"
	script,err:=os.ReadFile("../../../deployments/curator-role.sql");if err!=nil{t.Fatal(err)}
	if _,err=owner.Exec(ctx,strings.ReplaceAll(string(script),"anime_curator",pgx.Identifier{role}.Sanitize()));err!=nil{t.Fatal(err)}
	t.Cleanup(func(){_,_=owner.Exec(ctx,`DROP OWNED BY `+pgx.Identifier{role}.Sanitize());_,_=owner.Exec(ctx,`DROP ROLE `+pgx.Identifier{role}.Sanitize())})
	password,_:=u.User.Password()
	if _,err=owner.Exec(ctx,`ALTER ROLE `+pgx.Identifier{role}.Sanitize()+` PASSWORD '`+password+`'`);err!=nil{t.Fatal(err)}
	u.User=url.UserPassword(role,password)
	curator,err:=pgx.Connect(ctx,u.String());if err!=nil{t.Fatal(err)}
	t.Cleanup(func(){_=curator.Close(ctx)})
	if err=catalogsync.CheckWriter(ctx,curator);err!=nil{t.Fatal("safe curator role rejected",err)}
	if err=catalogsync.CheckWriter(ctx,owner);err==nil{t.Fatal("migration owner accepted")}
	if _,err=owner.Exec(ctx,`INSERT INTO anime_features(feature_key,display_name,category) VALUES('synthetic.curator','Synthetic curator','SYNTHETIC')`);err!=nil{t.Fatal(err)}
	value,confidence:=0.5,1.0
	now:=time.Now().UTC().Truncate(time.Microsecond)
	s:=curation.Snapshot{Version:1,ObservedAt:now,Records:[]curation.Record{{AnimeID:"10000000-0000-4000-8000-000000000001",FeatureKey:"synthetic.curator",Value:&value,Confidence:&confidence,Reference:"https://example.test/synthetic-curator"}}}
	out,err:=curation.Apply(ctx,curator,s,now)
	if err!=nil||out.Inserted!=1{t.Fatal("curator append failed",out,err)}
	for _,sql:=range []string{`SELECT email FROM users`,`UPDATE anime_feature_observations SET confidence=0`,`DELETE FROM anime_feature_observations`,`UPDATE anime_features SET active=false`}{
		if _,err=curator.Exec(ctx,sql);err==nil{t.Fatal("excess curator privilege",sql)}
	}
	if _,err=owner.Exec(ctx,`GRANT SELECT(email) ON users TO `+pgx.Identifier{role}.Sanitize());err!=nil{t.Fatal(err)}
	if err=catalogsync.CheckWriter(ctx,curator);err==nil{t.Fatal("curator with private-column access accepted")}
}
