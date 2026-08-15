package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"opskeeper/backend/version"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	logger.Info("scheduler started", "version", version.Value)
	<-ctx.Done()
	logger.Info("scheduler stopped")
}
