package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/opskeeper/opskeeper/backend/internal/config"
	"github.com/opskeeper/opskeeper/backend/internal/health"
	"github.com/opskeeper/opskeeper/backend/internal/httpapi"
	"github.com/opskeeper/opskeeper/backend/internal/organization"
	"github.com/opskeeper/opskeeper/backend/internal/version"
	"github.com/redis/go-redis/v9"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(logger); err != nil {
		logger.Error("api stopped", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return errors.Join(errors.New("configure PostgreSQL client"), err)
	}
	defer pool.Close()

	redisOptions, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		return errors.Join(errors.New("configure Redis client"), err)
	}
	redisClient := redis.NewClient(redisOptions)
	defer func() {
		if closeErr := redisClient.Close(); closeErr != nil {
			logger.Warn("close Redis client", "error", closeErr)
		}
	}()

	healthService := health.NewService(cfg.DependencyTimeout, []health.Check{
		{Name: "postgres", Run: pool.Ping},
		{Name: "redis", Run: func(checkCtx context.Context) error {
			return redisClient.Ping(checkCtx).Err()
		}},
	})
	organizationService := organization.NewService(organization.NewRepository(pool))

	server := &http.Server{
		Addr:              cfg.HTTPAddress,
		Handler:           httpapi.NewRouter(logger, healthService, version.Value, organizationService),
		ReadHeaderTimeout: cfg.ReadHeaderTimeout,
		ReadTimeout:       cfg.ReadTimeout,
		WriteTimeout:      cfg.WriteTimeout,
		IdleTimeout:       cfg.IdleTimeout,
	}

	serverErr := make(chan error, 1)
	go func() {
		logger.Info("api listening", "address", cfg.HTTPAddress, "environment", cfg.Environment, "version", version.Value)
		serverErr <- server.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			return errors.Join(errors.New("shutdown api"), err)
		}
		return nil
	case err := <-serverErr:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}
