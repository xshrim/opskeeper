package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/jackc/pgx/v5/pgxpool"
	"opskeeper/backend/config"
	"opskeeper/backend/logging"
	"opskeeper/backend/migrations"
)

const serviceName = "opskeeper-migrate"

func main() {
	logger := logging.NewRaw(os.Stdout).With("service", serviceName)
	cfg, err := config.Load()
	if err != nil {
		logger.Error("load configuration", "kind", "error", "error_type", "configuration", "error", err)
		os.Exit(1)
	}
	logger, err = logging.New(os.Stdout, cfg.LogFormat)
	if err != nil {
		logger.Error("configure logging", "kind", "error", "error_type", "logging", "error", err)
		os.Exit(1)
	}
	logger = logger.With("service", serviceName)
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	direction := "up"
	if len(os.Args) > 1 {
		direction = os.Args[1]
	}
	if err := run(ctx, direction, cfg); err != nil {
		logger.Error("migration failed", "kind", "error", "error_type", "migration", "error_summary", migrationErrorSummary(err))
		os.Exit(1)
	}
	logger.Info("migration command completed", "kind", "job", "direction", direction)
}

func migrationErrorSummary(err error) string {
	if err == nil {
		return ""
	}
	const maxLength = 500
	summary := strings.Join(strings.Fields(err.Error()), " ")
	if len(summary) > maxLength {
		return summary[:maxLength] + "..."
	}
	return summary
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
