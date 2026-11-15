package postgres_test

import (
	"context"
	"net/url"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)
func TestMatchSnapshotRetriesConcurrentSessionRowUpdate(t *testing.T){
	db,owner,ownerDSN,runtimeDSN:=testDB(t)
	ctx:=context.Background()
	s,err:=db.Register(ctx,"snapshot-match@example.test","synthetic-snapshot-match-password")
	if err!=nil{t.Fatal(err)}
	verifyAccount(t,db,s.Token)
	u,err:=url.Parse(runtimeDSN);if err!=nil{t.Fatal(err)}
	observer,err:=pgx.Connect(ctx,ownerDSN);if err!=nil{t.Fatal(err)}
	defer observer.Close(ctx)
	repo:=db.Repository(s.Token)
	for _,read:=range []func(context.Context)error{
		func(ctx context.Context)error{_,err:=repo.FeaturePreferences(ctx);return err},
		func(ctx context.Context)error{_,err:=repo.PersonalMatches(ctx,20);return err},
	}{
		tx,err:=owner.Begin(ctx);if err!=nil{t.Fatal(err)}
		defer tx.Rollback(ctx)
		if _,err=tx.Exec(ctx,`UPDATE auth_sessions SET last_used_at=last_used_at+interval '1 microsecond' WHERE user_id=$1`,s.User.ID);err!=nil{t.Fatal(err)}
		attempt,cancel:=context.WithTimeout(ctx,5*time.Second)
		done:=make(chan error,1)
		go func(){done<-read(attempt)}()
		waiting:=false
		for !waiting&&attempt.Err()==nil{
			if err=observer.QueryRow(attempt,`SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE datname=current_database() AND usename=$1 AND wait_event_type='Lock')`,u.User.Username()).Scan(&waiting);err!=nil{cancel();t.Fatal(err)}
			if !waiting{
				select{case <-attempt.Done():case <-time.After(10*time.Millisecond):}
			}
		}
		if !waiting{cancel();t.Fatal("recommendation snapshot never reached the held session lock")}
		if err=tx.Commit(ctx);err!=nil{cancel();t.Fatal(err)}
		err=<-done
		cancel()
		if err!=nil{t.Fatal("snapshot did not retry session serialization conflict",err)}
	}
}
