package postgres_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/khordestan/anime-intelligence/internal/httpapi"
	"github.com/khordestan/anime-intelligence/internal/platform/store"
)
func TestPersonalMatchesEvidenceAndOwnerIsolation(t *testing.T){
	db,owner,_,_:=testDB(t)
	ctx:=context.Background()
	one,err:=db.Register(ctx,"match-one@example.test","synthetic-match-password")
	if err!=nil{t.Fatal(err)}
	two,err:=db.Register(ctx,"match-two@example.test","synthetic-match-password")
	if err!=nil{t.Fatal(err)}
	a,b:=db.Repository(one.Token),db.Repository(two.Token)
	if _,err=a.PersonalMatches(ctx,20);!errors.Is(err,store.ErrEmailUnverified){t.Fatal("unverified recommendations",err)}
	verifyAccount(t,db,one.Token);verifyAccount(t,db,two.Token)
	page,err:=a.PersonalMatches(ctx,20)
	if err!=nil||len(page.Items)!=0||len(page.Profile)!=0{t.Fatal("unsupported cold start",page,err)}
	if _,err=owner.Exec(ctx,`INSERT INTO anime_features(feature_key,display_name,category,spoiler_sensitive) VALUES
('synthetic.power','Synthetic power','SYNTHETIC',false),('synthetic.secret','Synthetic secret','SYNTHETIC',true)`);err!=nil{t.Fatal(err)}
	const watched="10000000-0000-4000-8000-000000000001"
	const strong="10000000-0000-4000-8000-000000000002"
	const weak="10000000-0000-4000-8000-000000000003"
	for _,fixture:=range []struct{id,kind,spoiler,reference string;value float64}{
		{watched,"CURATED","NONE","https://example.test/fixture",1},
		{strong,"CURATED","NONE","https://example.test/fixture",1},
		{weak,"CURATED","NONE","https://example.test/fixture",0.2},
		{"10000000-0000-4000-8000-000000000004","PROVIDER","NONE","https://example.test/fixture",1},
		{"10000000-0000-4000-8000-000000000005","CURATED","ENDING","https://example.test/fixture",1},
		{"10000000-0000-4000-8000-000000000006","CURATED","NONE","",1},
	}{
		if _,err=owner.Exec(ctx,`INSERT INTO anime_feature_observations(anime_id,feature_id,value,confidence,source_kind,evidence,scope,spoiler_level)
SELECT $1,id,$2,1,$3,jsonb_build_object('reference',$4::text),'SERIES',$5 FROM anime_features WHERE feature_key='synthetic.power'`,fixture.id,fixture.value,fixture.kind,fixture.reference,fixture.spoiler);err!=nil{t.Fatal(err)}
	}
	for _,repo:=range []interface{Add(context.Context,string,string,int)(store.Entry,error)}{a,b}{
		if _,err:=repo.Add(ctx,watched,"COMPLETED",12);err!=nil{t.Fatal(err)}
	}
	positive,negative:=1.0,-1.0
	if ok,err:=a.SetFeaturePreference(ctx,"synthetic.power",&positive);err!=nil||!ok{t.Fatal(err)}
	if ok,err:=b.SetFeaturePreference(ctx,"synthetic.power",&negative);err!=nil||!ok{t.Fatal(err)}
	if ok,err:=a.SetFeaturePreference(ctx,"synthetic.secret",&positive);err!=nil||ok{t.Fatal("spoiler feature accepted",err)}
	page,err=a.PersonalMatches(ctx,20)
	if err!=nil||len(page.Items)!=2||page.Items[0].AnimeID!=strong||page.Items[1].AnimeID!=weak||len(page.Items[0].Evidence)!=1{t.Fatal("positive evidence/exclusion",page,err)}
	other,err:=b.PersonalMatches(ctx,20)
	if err!=nil||len(other.Items)!=2||other.Items[0].AnimeID!=weak{t.Fatal("foreign preference used",other,err)}
	if len(page.Profile)!=1||page.Profile[0].Key!="synthetic.power"||page.Profile[0].Declared==nil||page.Profile[0].Declared.Weight!=1||page.Profile[0].Inferred!=nil{t.Fatal("declared profile",page.Profile)}
	if len(other.Profile)!=1||other.Profile[0].Declared==nil||other.Profile[0].Declared.Weight != -1||other.Profile[0].Inferred!=nil{t.Fatal("foreign profile leaked",other.Profile)}
	encoded,_:=json.Marshal(page)
	if strings.Contains(string(encoded),"example.test")||strings.Contains(string(encoded),two.User.ID)||strings.Contains(string(encoded),"synthetic.secret"){t.Fatal("private/raw/spoiler evidence exposed")}
	if _,err=a.SetFeaturePreference(ctx,"synthetic.power",nil);err!=nil{t.Fatal(err)}
	page,err=a.PersonalMatches(ctx,20)
	if err!=nil||len(page.Items)!=0{t.Fatal("cleared preference retained",page,err)}
	if _,ok,err:=a.Rate(ctx,watched,5);err!=nil||!ok{t.Fatal(err)}
	page,err=a.PersonalMatches(ctx,1)
	if err!=nil||len(page.Items)!=1||page.Items[0].AnimeID!=strong||len(page.Profile)!=1||page.Profile[0].Declared!=nil||page.Profile[0].Inferred==nil||page.Profile[0].Inferred.Weight<=0{t.Fatal("own rating inference",page,err)}
	prefs,err:=b.FeaturePreferences(ctx)
	if err!=nil{t.Fatal(err)}
	found:=false
	for _,p:=range prefs{if p.Key=="synthetic.power"{found=p.Weight!=nil&&*p.Weight == -1};if p.Key=="synthetic.secret"{t.Fatal("spoiler taxonomy exposed")}}
	if !found{t.Fatal("another account's declared preference changed")}
}
func TestRecommendationRoutesRejectOwnerOverrides(t *testing.T){
	db,_,_,_:=testDB(t)
	ctx:=context.Background()
	s,err:=db.Register(ctx,"match-http@example.test","synthetic-match-password")
	if err!=nil{t.Fatal(err)}
	h:=httpapi.NewPostgresRouter(slog.New(slog.NewTextHandler(io.Discard,nil)),db)
	call:=func(method,path,token,body string)*httptest.ResponseRecorder{
		r:=httptest.NewRequest(method,path,strings.NewReader(body));if token!=""{r.Header.Set("Authorization","Bearer "+token)}
		w:=httptest.NewRecorder();h.ServeHTTP(w,r);return w
	}
	for _,spec:=range [][2]string{{"GET","/api/v1/recommendations"},{"GET","/api/v1/preferences"},{"PUT","/api/v1/preferences/missing"},{"DELETE","/api/v1/preferences/missing"}}{
		if w:=call(spec[0],spec[1],"",`{"weight":1}`);w.Code!=401{t.Fatal("unprotected route",w.Code)}
		if w:=call(spec[0],spec[1],s.Token,`{"weight":1}`);w.Code!=403{t.Fatal("unverified route",w.Code)}
	}
	verifyAccount(t,db,s.Token)
	for _,body:=range []string{`{"weight":1,"userId":"spoofed"}`,`{"weight":null}`,`{"weight":2}`}{
		if w:=call("PUT","/api/v1/preferences/missing",s.Token,body);w.Code!=400{t.Fatal("invalid preference body",w.Code,w.Body.String())}
	}
	if w:=call("GET","/api/v1/recommendations",s.Token,"");w.Code!=200||!strings.Contains(w.Body.String(),`"items":[]`)||w.Header().Get("Cache-Control")!="no-store"{t.Fatal("cold-start response",w.Code,w.Body.String())}
}
