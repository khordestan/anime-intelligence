package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/khordestan/anime-intelligence/internal/platform/catalogjob"
	"github.com/khordestan/anime-intelligence/internal/platform/catalogsync"
	"github.com/khordestan/anime-intelligence/internal/providers/wikidata"
 "github.com/khordestan/anime-intelligence/internal/providers/tvmaze"
)
func main(){
	logger:=slog.New(slog.NewJSONHandler(os.Stdout,nil))
	config,err:=catalogjob.ParseWorker(os.Getenv("CATALOG_SYNC_PROVIDER"),os.Getenv("CATALOG_SYNC_IDS"),os.Getenv("CATALOG_SYNC_INTERVAL"),os.Getenv("CATALOG_SYNC_REVIEWED_ANIME_SEASONS")=="1",os.Getenv("CATALOG_SYNC_SCOPE"))
	if err!=nil{logger.Error("invalid catalog worker configuration");os.Exit(1)}
	ctx,stop:=signal.NotifyContext(context.Background(),os.Interrupt,syscall.SIGTERM);defer stop()
	if len(config.IDs)==0{
		logger.Info("worker idle; no catalog synchronization configured")
		<-ctx.Done();logger.Info("worker stopped");return
	}
	dsn:=os.Getenv("CATALOG_DATABASE_URL")
	if dsn==""{logger.Error("catalog worker connection is required");os.Exit(1)}
	var client catalogsync.Fetcher=wikidata.New()
 if config.Provider=="tvmaze"{client=tvmaze.New(true)}
	task:=func(parent context.Context)error{
		cycle,cancel:=context.WithTimeout(parent,2*time.Minute);defer cancel()
		settings,err:=pgx.ParseConfig(dsn)
		if err!=nil{return catalogsync.ErrWorker}
		settings.ConnectTimeout=5*time.Second
		settings.RuntimeParams["search_path"]="pg_catalog,public"
		conn,err:=pgx.ConnectConfig(cycle,settings)
		if err!=nil{return catalogsync.ErrWorker}
		defer func(){cleanup,done:=context.WithTimeout(context.Background(),5*time.Second);defer done();_=conn.Close(cleanup)}()
		_,err=catalogsync.SyncQueuedProviderWithOptions(cycle,conn,config.Provider,client,config.IDs,config.Interval,time.Now().UTC(),catalogsync.ImportOptions{SchedulesOnly:config.SchedulesOnly})
		return err
	}
	logger.Info("catalog queue worker enabled","refresh_interval_seconds",int(config.Interval.Seconds()),"item_count",len(config.IDs))
	err=catalogjob.RunQueue(ctx,time.Minute,task,catalogjob.Wait,func(ok bool){
		if ok{logger.Info("catalog cycle completed or not yet due")}else{logger.Warn("catalog cycle failed; no immediate retry")}
	})
	if err!=nil&&ctx.Err()==nil{logger.Error("catalog scheduler stopped unexpectedly");os.Exit(1)}
	logger.Info("worker stopped")
}
