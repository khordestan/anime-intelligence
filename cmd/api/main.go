package main

import (
	"context"
	"github.com/khordestan/anime-intelligence/internal/httpapi"
	"github.com/khordestan/anime-intelligence/internal/platform/identitymail"
	"github.com/khordestan/anime-intelligence/internal/platform/postgres"
	"github.com/khordestan/anime-intelligence/internal/platform/store"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	var handler http.Handler
	storage := "local-file"
	if dsn := os.Getenv("DATABASE_URL"); dsn != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		db, err := postgres.Open(ctx, dsn)
		cancel()
		if err != nil {
			logger.Error("PostgreSQL runtime initialization failed; check schema and restricted runtime role")
			os.Exit(1)
		}
		defer db.Close()
		var delivery *identitymail.Service
		config := identitymail.SMTPConfig{Address: os.Getenv("MAIL_SMTP_ADDR"), Username: os.Getenv("MAIL_SMTP_USER"), Password: os.Getenv("MAIL_SMTP_PASSWORD"), From: os.Getenv("MAIL_FROM"), AppOrigin: os.Getenv("PUBLIC_APP_ORIGIN"), CAFile: os.Getenv("MAIL_SMTP_CA_FILE")}
		if config.Address != "" || config.Username != "" || config.Password != "" || config.From != "" || config.AppOrigin != "" || config.CAFile != "" {
			sender, err := identitymail.NewSMTP(config)
			if err != nil {
				logger.Error("identity mail configuration is incomplete or invalid")
				os.Exit(1)
			}
			delivery = identitymail.NewService(db, sender, logger)
			defer delivery.Close()
		}
		handler = httpapi.NewPostgresRouterWithIdentity(logger, db, delivery)
		storage = "postgresql"
	} else {
		dataPath := os.Getenv("DATA_FILE")
		if dataPath == "" {
			dataPath = "./data/local.json"
		}
		data, err := store.New(dataPath)
		if err != nil {
			logger.Error("load local data failed")
			os.Exit(1)
		}
		handler = httpapi.NewRouter(logger, data)
	}
	addr := os.Getenv("HTTP_ADDR")
	if addr == "" {
		addr = "127.0.0.1:8080"
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	server := &http.Server{Addr: addr, Handler: handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second}
	go func() {
		logger.Info("starting API", "addr", addr, "storage", storage)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("API server failed", "error", err)
			stop()
		}
	}()
	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("graceful shutdown failed", "error", err)
	}
}
