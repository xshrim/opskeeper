package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
	"opskeeper/backend/config"
	"opskeeper/backend/migrations"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	direction := "up"
	if len(os.Args) > 1 {
		direction = os.Args[1]
	}
	if err := run(context.Background(), direction); err != nil {
		logger.Error("migration failed", "error", err)
		os.Exit(1)
	}
	logger.Info("migration command completed", "direction", direction)
}

func run(ctx context.Context, direction string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	switch direction {
	case "up":
		return migrations.Apply(ctx, pool)
	case "down":
		return migrations.RollbackLast(ctx, pool)
	default:
		return fmt.Errorf("unknown migration direction %q; use up or down", direction)
	}
}
