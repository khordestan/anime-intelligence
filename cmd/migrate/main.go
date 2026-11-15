package main

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/khordestan/anime-intelligence/migrations"
)

func main() {
	dsn := os.Getenv("MIGRATION_DATABASE_URL")
	if dsn == "" {
		slog.Error("MIGRATION_DATABASE_URL is required")
		os.Exit(1)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		slog.Error("migration database connection failed")
		os.Exit(1)
	}
	defer conn.Close(context.Background())
	if err = migrations.Apply(ctx, conn); err != nil {
		slog.Error("migration failed", "error", err)
		os.Exit(1)
	}
	slog.Info("schema migrations applied")
}
