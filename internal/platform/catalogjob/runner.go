// Package catalogjob provides opt-in, non-overlapping catalog cycle scheduling.
package catalogjob

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"time"
 "github.com/khordestan/anime-intelligence/internal/providers/tvmaze"
)
var ErrConfig=errors.New("invalid catalog synchronization configuration")
var qid=regexp.MustCompile(`^Q[1-9][0-9]{0,11}$`)
type Config struct{IDs []string;Interval time.Duration;Provider string;SchedulesOnly bool}
func Parse(ids,interval string)(Config,error){return ParseProvider("wikidata",ids,interval,false)}
// ParseProvider requires explicit anime-season review for TVmaze. Empty IDs
// remain completely idle, even when unused configuration is invalid.
func ParseProvider(provider,ids,interval string,reviewed bool)(Config,error){
 if provider==""{provider="wikidata"}
 c:=Config{Interval:24*time.Hour,Provider:provider}
	if strings.TrimSpace(ids)==""{return c,nil}
	if provider!="wikidata"&&provider!="tvmaze"{return c,ErrConfig}
 if provider=="tvmaze"&&!reviewed{return c,ErrConfig}
 if interval!=""{d,err:=time.ParseDuration(interval);if err!=nil{return c,ErrConfig};c.Interval=d}
	if c.Interval<time.Hour||c.Interval>7*24*time.Hour{return c,ErrConfig}
	c.IDs=strings.Split(ids,",")
	if len(c.IDs)>20{return c,ErrConfig}
	seen:=map[string]bool{}
	for i,id:=range c.IDs{id=strings.TrimSpace(id);valid:=qid.MatchString(id);if provider=="tvmaze"{_,_,err:=tvmaze.ParseID(id);valid=err==nil};if !valid||seen[id]{return c,ErrConfig};seen[id]=true;c.IDs[i]=id}
	return c,nil
}
func Wait(ctx context.Context,d time.Duration)error{
	timer:=time.NewTimer(d);defer timer.Stop()
	select{case <-ctx.Done():return ctx.Err();case <-timer.C:return nil}
}
// Run waits after every cycle, including failures. No overlapping tasks or
// automatic tight-loop retries occur; cancellation interrupts idle waiting.
func Run(ctx context.Context,interval time.Duration,task func(context.Context)error,wait func(context.Context,time.Duration)error,report func(bool))error{
	if interval<time.Hour||interval>7*24*time.Hour||task==nil||wait==nil{return ErrConfig}
 return run(ctx,interval,task,wait,report)
}
// RunQueue polls a durable queue at a bounded cadence, separate from each
// title's persisted refresh interval. Tasks stay serial and cancellation-aware.
func RunQueue(ctx context.Context,poll time.Duration,task func(context.Context)error,wait func(context.Context,time.Duration)error,report func(bool))error{
 if poll<time.Minute||poll>time.Hour||task==nil||wait==nil{return ErrConfig}
 return run(ctx,poll,task,wait,report)
}
func run(ctx context.Context,interval time.Duration,task func(context.Context)error,wait func(context.Context,time.Duration)error,report func(bool))error{
 for{
		if ctx.Err()!=nil{return ctx.Err()}
		err:=task(ctx)
		if ctx.Err()!=nil{return ctx.Err()}
		if report!=nil{report(err==nil)}
		if err=wait(ctx,interval);err!=nil{return err}
	}
}

// ParseWorker keeps unused configuration idle and makes secondary schedule
// collection explicit. Wikidata has no schedule capability.
func ParseWorker(provider,ids,interval string,reviewed bool,scope string)(Config,error){
 c,err:=ParseProvider(provider,ids,interval,reviewed)
 if err!=nil||len(c.IDs)==0{return c,err}
 switch scope{
 case "","full":return c,nil
 case "schedules":
  if c.Provider!="tvmaze"{return c,ErrConfig}
  c.SchedulesOnly=true;return c,nil
 default:return c,ErrConfig
 }
}
