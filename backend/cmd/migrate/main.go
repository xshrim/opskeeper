package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/jackc/pgx/v5/pgxpool"
	"opskeeper/backend/config"
	"opskeeper/backend/logging"
	"opskeeper/backend/migrations"
)

func main() {
	logger := logging.NewText(os.Stdout)
	cfg, err := config.Load()
	if err != nil {
		logger.Error("load configuration", "error", err)
		os.Exit(1)
	}
	logger, err = logging.New(os.Stdout, cfg.LogFormat)
	if err != nil {
		logger.Error("configure logging", "error", err)
		os.Exit(1)
	}
	logger = logger.With("service", cfg.ServiceName("migrate"))
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	direction := "up"
	if len(os.Args) > 1 {
		direction = os.Args[1]
	}
	if err := run(ctx, direction, cfg); err != nil {
		logger.Error("migration failed", "error", err)
		os.Exit(1)
	}
	logger.Info("migration command completed", "direction", direction)
}

func run(ctx context.Context, direction string, cfg config.Config) error {
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
