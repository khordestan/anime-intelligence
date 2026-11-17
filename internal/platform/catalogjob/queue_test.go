package catalogjob

import (
 "context"
 "errors"
 "testing"
 "time"
)

func TestQueuePollsSeriallyAndStopsOnCancellation(t *testing.T){
 ctx,cancel:=context.WithCancel(context.Background())
 defer cancel()
 calls,waits:=0,0
 var outcomes []bool
 err:=RunQueue(ctx,time.Minute,func(context.Context)error{
  calls++
  if calls==1{return errors.New("synthetic")}
  return nil
 },func(_ context.Context,d time.Duration)error{
  waits++
  if d!=time.Minute{t.Fatal("wrong poll interval",d)}
  if waits==2{cancel();return context.Canceled}
  return nil
 },func(ok bool){outcomes=append(outcomes,ok)})
 if !errors.Is(err,context.Canceled)||calls!=2||waits!=2||len(outcomes)!=2||outcomes[0]||!outcomes[1]{t.Fatal(calls,waits,outcomes,err)}
 for _,poll:=range []time.Duration{0,time.Second,time.Hour+time.Second}{
  if err:=RunQueue(context.Background(),poll,func(context.Context)error{return nil},Wait,nil);!errors.Is(err,ErrConfig){t.Fatal("invalid polling accepted",poll,err)}
 }
}
