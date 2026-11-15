package main

import (
 "context"
 "log/slog"
 "net/http"
 "os"
 "os/signal"
 "syscall"
 "time"
 "github.com/khordestan/anime-intelligence/internal/httpapi"
 "github.com/khordestan/anime-intelligence/internal/platform/store"
)

func main() {
 logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
 dataPath := os.Getenv("DATA_FILE")
 if dataPath == "" { dataPath = "./data/local.json" }
 data, err := store.New(dataPath)
 if err != nil { logger.Error("load local data failed", "error", err); os.Exit(1) }
 addr := os.Getenv("HTTP_ADDR")
 if addr == "" { addr = "127.0.0.1:8080" }
 ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
 defer stop()
 server := &http.Server{Addr: addr, Handler: httpapi.NewRouter(logger, data), ReadHeaderTimeout: 5*time.Second, ReadTimeout: 15*time.Second, WriteTimeout: 30*time.Second, IdleTimeout: 60*time.Second}
 go func() {
  logger.Info("starting API", "addr", addr, "data_file", dataPath)
  if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed { logger.Error("API server failed", "error", err); stop() }
 }()
 <-ctx.Done()
 shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
 defer cancel()
 if err := server.Shutdown(shutdownCtx); err != nil { logger.Error("graceful shutdown failed", "error", err) }
}
