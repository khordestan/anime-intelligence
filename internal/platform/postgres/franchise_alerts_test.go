package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/khordestan/anime-intelligence/internal/modules/catalog/feed"
	"github.com/khordestan/anime-intelligence/internal/platform/catalogsync"
	"github.com/khordestan/anime-intelligence/internal/platform/postgres"
)
func TestLinkedSeasonAnnouncementsAndEpisodeAlerts(t *testing.T) {
	db,owner,_,_:=testDB(t)
	ctx:=context.Background()
	one,err:=db.Register(ctx,"franchise-one@example.test","synthetic-franchise-password")
	if err!=nil {t.Fatal(err)}
	two,err:=db.Register(ctx,"franchise-two@example.test","synthetic-franchise-password")
	if err!=nil {t.Fatal(err)}
	verifyAccount(t,db,one.Token);verifyAccount(t,db,two.Token)
	if _,err=owner.Exec(ctx,`INSERT INTO data_sources(provider_key,display_name,enabled,policy_url) VALUES('franchise-fixture','Synthetic linked seasons',true,'https://example.test/policy')`);err!=nil {t.Fatal(err)}
	now:=time.Now().UTC().Truncate(time.Microsecond)
	s:=feed.Snapshot{Version:1,Provider:"franchise-fixture",ObservedAt:now.Add(-4*time.Hour),Titles:[]feed.Title{{ExternalID:"one",Title:"Synthetic Season One",MediaType:"TV",Status:"RELEASING",Relations:[]feed.Relation{{ExternalID:"two",Type:"SEQUEL"}}}}}
	if _,err=catalogsync.Import(ctx,owner,s,now);err!=nil {t.Fatal(err)}
	var root string
	if err=owner.QueryRow(ctx,`SELECT anime_id::text FROM anime_external_ids WHERE provider='franchise-fixture' AND external_id='one'`).Scan(&root);err!=nil {t.Fatal(err)}
	a,b:=db.Repository(one.Token),db.Repository(two.Token)
	if ok,err:=a.FollowScope(ctx,root,"SUB","FRANCHISE");err!=nil || !ok {t.Fatal(err)}
	if ok,err:=b.Follow(ctx,root,"SUB");err!=nil || !ok {t.Fatal(err)}
	if _,err=owner.Exec(ctx,`UPDATE user_follows SET created_at=$1 WHERE anime_id=$2`,now.Add(-3*time.Hour),root);err!=nil {t.Fatal(err)}
	s.ObservedAt=now.Add(-2*time.Hour)
	s.Titles=[]feed.Title{{ExternalID:"two",Title:"Synthetic Season Two",MediaType:"TV",Status:"ANNOUNCED",Relations:[]feed.Relation{{ExternalID:"one",Type:"PREQUEL"},{ExternalID:"three",Type:"SEQUEL"}}}}
	out,err:=catalogsync.Import(ctx,owner,s,now)
	if err!=nil || out.SeasonAnnouncements!=1 {t.Fatal("new season event",out,err)}
	s.ObservedAt=now.Add(-time.Hour)
	s.Titles=[]feed.Title{{ExternalID:"three",Title:"Synthetic Season Three",MediaType:"TV",Status:"ANNOUNCED"}}
	out,err=catalogsync.Import(ctx,owner,s,now)
	if err!=nil || out.SeasonAnnouncements!=1 {t.Fatal("transitive season event",out,err)}
	na,err:=a.Notifications(ctx)
	if err!=nil || len(na)!=2 || na[0].Title!="New season announced" {t.Fatal("franchise announcement delivery",na,err)}
	nb,err:=b.Notifications(ctx)
	if err!=nil || len(nb)!=0 {t.Fatal("single-season follow expanded to franchise",nb,err)}
	released:=now.Add(-30*time.Minute)
	s.ObservedAt=now.Add(-20*time.Minute)
	s.Titles=[]feed.Title{{ExternalID:"two",Title:"Synthetic Season Two",MediaType:"TV",Status:"RELEASING",Episodes:[]feed.Episode{{Number:1,Track:"SUB",ReleasedAt:&released}}}}
	out,err=catalogsync.Import(ctx,owner,s,now)
	if err!=nil || out.ReleaseEvents!=1 || out.SeasonAnnouncements!=0 {t.Fatal("linked-season release",out,err)}
	na,err=a.Notifications(ctx)
	if err!=nil || len(na)!=3 || na[0].Title!="New episode released" {t.Fatal("linked-season episode delivery",na,err)}
	na,err=a.Notifications(ctx)
	if err!=nil || len(na)!=3 {t.Fatal("franchise duplicates",err)}
	if ok,err:=a.FollowScope(ctx,root,"SUB","ANIME");err!=nil || !ok {t.Fatal(err)}
	follows,err:=a.Follows(ctx)
	if err!=nil || len(follows)!=1 || follows[0].Scope!="ANIME" || !follows[0].CreatedAt.Equal(now.Add(-3*time.Hour)) {t.Fatal("scope replacement/reset subscription time",follows,err)}
	if _,err=a.FollowScope(ctx,root,"SUB","INVALID");err==nil {t.Fatal("invalid follow scope accepted")}
}

func TestFollowQuotaAllowsUpdatesButRejectsNewTitles(t *testing.T) {
	db,owner,_,_:=testDB(t)
	ctx:=context.Background()
	s,err:=db.Register(ctx,"follow-quota@example.test","synthetic-follow-quota-password")
	if err!=nil {t.Fatal(err)}
	verifyAccount(t,db,s.Token)
	if _,err=owner.Exec(ctx,`WITH seeded AS (
INSERT INTO anime(canonical_title) SELECT 'Synthetic Follow ' || n FROM generate_series(1,1000) n RETURNING id
) INSERT INTO user_follows(user_id,anime_id) SELECT $1,id FROM seeded`,s.User.ID);err!=nil {t.Fatal(err)}
	a:=db.Repository(s.Token)
	if _,err=a.Follow(ctx,"10000000-0000-4000-8000-000000000001","SUB");err!=postgres.ErrFollowLimit {t.Fatal("follow quota not enforced",err)}
	var id string
	if err=owner.QueryRow(ctx,`SELECT anime_id::text FROM user_follows WHERE user_id=$1 LIMIT 1`,s.User.ID).Scan(&id);err!=nil {t.Fatal(err)}
	if ok,err:=a.FollowScope(ctx,id,"ALL","FRANCHISE");err!=nil || !ok {t.Fatal("quota blocked existing preference update",err)}
}
