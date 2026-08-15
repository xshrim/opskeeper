package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"opskeeper/backend/config"
	"opskeeper/backend/version"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	cfg, err := config.Load()
	if err != nil {
		logger.Error("load configuration", "error", err)
		os.Exit(1)
	}
	logger = logger.With("service", cfg.ServiceName("scheduler"))
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	logger.Info("scheduler started", "version", version.Value)
	<-ctx.Done()
	logger.Info("scheduler stopped")
}
