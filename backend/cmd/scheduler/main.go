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
	logger = logger.With("service", cfg.ServiceName("scheduler"))
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	logger.Info("scheduler started", "version", version.Value)
	<-ctx.Done()
	logger.Info("scheduler stopped")
}
