package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"opskeeper/backend/config"
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

	logger.Info("scheduler started")
	<-ctx.Done()
	logger.Info("scheduler stopped")
}
