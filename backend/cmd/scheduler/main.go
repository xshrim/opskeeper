package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"opskeeper/backend/config"
	"opskeeper/backend/inspection"
	"opskeeper/backend/logging"
	"opskeeper/backend/observability"
	"opskeeper/backend/version"
)

const serviceName = "opskeeper-scheduler"

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
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Error("configure PostgreSQL client", "error", err)
		os.Exit(1)
	}
	defer pool.Close()
	store := inspection.NewStore(pool)
	run := func() {
		started := time.Now()
		created, err := store.ScheduleDue(ctx, time.Now())
		if err != nil {
			logger.Error("schedule inspections", "error", err)
			observability.RecordError(ctx, "scheduler", "schedule")
			observability.RecordTask(ctx, "schedule", "failure", time.Since(started))
			return
		}
		observability.RecordTask(ctx, "schedule", "success", time.Since(started))
		if created > 0 {
			logger.Info("scheduled inspection runs", "count", created)
		}
	}
	run()
	ticker := time.NewTicker(cfg.InspectionScheduleInterval)
	defer ticker.Stop()
	logger.Info("scheduler started", "interval", cfg.InspectionScheduleInterval)
	for {
		select {
		case <-ctx.Done():
			logger.Info("scheduler stopped")
			return
		case <-ticker.C:
			run()
		}
	}
}
