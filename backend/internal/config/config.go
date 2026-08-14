package config

import (
	"errors"
	"fmt"
	"os"
	"time"
)

const (
	defaultDatabaseURL = "postgres://opskeeper:opskeeper@localhost:5432/opskeeper?sslmode=disable"
	defaultRedisURL    = "redis://localhost:6379/0"
)

type Config struct {
	Environment       string
	HTTPAddress       string
	DatabaseURL       string
	RedisURL          string
	ShutdownTimeout   time.Duration
	DependencyTimeout time.Duration
	ReadHeaderTimeout time.Duration
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
}

func Load() (Config, error) {
	cfg := Config{
		Environment:       envOrDefault("OPSK_ENVIRONMENT", "development"),
		HTTPAddress:       envOrDefault("OPSK_HTTP_ADDRESS", ":8080"),
		DatabaseURL:       envOrDefault("OPSK_DATABASE_URL", defaultDatabaseURL),
		RedisURL:          envOrDefault("OPSK_REDIS_URL", defaultRedisURL),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	var err error
	if cfg.ShutdownTimeout, err = durationFromEnv("OPSK_SHUTDOWN_TIMEOUT", 10*time.Second); err != nil {
		return Config{}, err
	}
	if cfg.DependencyTimeout, err = durationFromEnv("OPSK_DEPENDENCY_TIMEOUT", 2*time.Second); err != nil {
		return Config{}, err
	}

	if cfg.HTTPAddress == "" {
		return Config{}, errors.New("OPSK_HTTP_ADDRESS must not be empty")
	}
	if cfg.DatabaseURL == "" {
		return Config{}, errors.New("OPSK_DATABASE_URL must not be empty")
	}
	if cfg.RedisURL == "" {
		return Config{}, errors.New("OPSK_REDIS_URL must not be empty")
	}

	return cfg, nil
}

func envOrDefault(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

func durationFromEnv(key string, fallback time.Duration) (time.Duration, error) {
	value, ok := os.LookupEnv(key)
	if !ok {
		return fallback, nil
	}
	duration, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("parse %s: %w", key, err)
	}
	if duration <= 0 {
		return 0, fmt.Errorf("%s must be positive", key)
	}
	return duration, nil
}
