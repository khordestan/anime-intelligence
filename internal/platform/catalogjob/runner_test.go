package catalogjob

import (
	"context"
	"errors"
	"testing"
	"time"
)
func TestConfigurationOptInAndBounds(t *testing.T){
	c,err:=Parse("","invalid")
	if err!=nil||len(c.IDs)!=0{t.Fatal("disabled worker should remain idle",err)}
	c,err=Parse(" Q1,Q2 ","2h")
	if err!=nil||len(c.IDs)!=2||c.IDs[0]!="Q1"||c.Interval!=2*time.Hour{t.Fatal("configuration",c,err)}
	for _,tc:=range [][2]string{{"Q1,Q1","24h"},{"Q0","24h"},{"Q1","1s"},{"Q1","200h"},{"Q1","bad"},{"Q1|Q2","24h"}}{
		if _,err:=Parse(tc[0],tc[1]);err==nil{t.Fatal("invalid configuration",tc)}
	}
}
func TestRunnerWaitsAfterFailureAndStopsOnCancellation(t *testing.T){
	ctx,cancel:=context.WithCancel(context.Background())
	calls,waits,reports:=0,0,[]bool{}
	err:=Run(ctx,time.Hour,func(context.Context)error{calls++;if calls==1{return errors.New("synthetic failure")};return nil},
		func(ctx context.Context,d time.Duration)error{waits++;if d!=time.Hour{t.Fatal("unexpected retry interval")};if waits==2{cancel();return ctx.Err()};return nil},
		func(ok bool){reports=append(reports,ok)})
	if !errors.Is(err,context.Canceled)||calls!=2||waits!=2||len(reports)!=2||reports[0]||!reports[1]{t.Fatal("retry/cancellation",calls,waits,reports,err)}
}
func TestWaitRespectsCancellation(t *testing.T){
	ctx,cancel:=context.WithCancel(context.Background());cancel()
	if err:=Wait(ctx,time.Hour);!errors.Is(err,context.Canceled){t.Fatal("cancelled wait",err)}
}
