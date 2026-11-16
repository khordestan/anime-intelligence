package store

import (
 "context"
 "errors"
)
var ErrWatchCycleConflict=errors.New("viewing pass changed or not completed")
var ErrWatchCycleInvalid=errors.New("invalid viewing pass")
type CycleUpdater interface{
 UpdateCycle(context.Context,string,*string,*int,int)(Entry,bool,error)
}
