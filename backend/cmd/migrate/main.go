package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"opskeeper/backend/config"
	"opskeeper/backend/logging"
	"opskeeper/backend/migrations"
	"opskeeper/backend/observability"
	"opskeeper/backend/version"
)

const serviceName = "opskeeper-migrate"

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
	logger = logger.With(append([]any{"service", serviceName}, version.LogAttributes()...)...)
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	build := version.Current()
	shutdownTelemetry, err := observability.Setup(ctx, serviceName, cfg.Environment, cfg.OTLPExporterEndpoint, observability.Build{Version: build.Version, Commit: build.Commit})
	if err != nil {
		logger.Error("configure telemetry", "error", err)
		os.Exit(1)
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := shutdownTelemetry(shutdownCtx); err != nil {
			logger.Warn("shutdown telemetry", "error", err)
		}
	}()

	direction := "up"
	if len(os.Args) > 1 {
		direction = os.Args[1]
	}
	started := time.Now()
	if err := run(ctx, direction, cfg); err != nil {
		observability.RecordTask(ctx, "migration", "failure", time.Since(started))
		logger.Error("migration failed", "error", err)
		os.Exit(1)
	}
	observability.RecordTask(ctx, "migration", "success", time.Since(started))
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
