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
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Error("configure PostgreSQL client", "error", err)
		os.Exit(1)
	}
	defer pool.Close()
	store := inspection.NewStore(pool)
	run := func() {
		created, err := store.ScheduleDue(ctx, time.Now())
		if err != nil {
			logger.Error("schedule inspections", "error", err)
			return
		}
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
