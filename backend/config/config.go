package config

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"time"
)

const (
	defaultDatabaseURL = "postgres://opskeeper:opskeeper@localhost:5432/opskeeper?sslmode=disable"
	defaultRedisURL    = "redis://localhost:6379/0"
	defaultPrefix      = "opskeeper"
	defaultLogFormat   = "text"
	maxPrefixLength    = 40
)

var prefixPattern = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]*[a-z0-9])?$`)

type Config struct {
	Prefix            string
	Environment       string
	LogFormat         string
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
		Prefix:            envOrDefault("OPSK_PREFIX", defaultPrefix),
		Environment:       envOrDefault("OPSK_ENVIRONMENT", "development"),
		LogFormat:         envOrDefault("OPSK_LOG_FORMAT", defaultLogFormat),
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
	if len(cfg.Prefix) > maxPrefixLength || !prefixPattern.MatchString(cfg.Prefix) {
		return Config{}, fmt.Errorf("OPSK_PREFIX must contain 1-%d lowercase letters, digits, or internal hyphens", maxPrefixLength)
	}
	if cfg.LogFormat != "text" && cfg.LogFormat != "json" {
		return Config{}, errors.New("OPSK_LOG_FORMAT must be text or json")
	}
	if cfg.DatabaseURL == "" {
		return Config{}, errors.New("OPSK_DATABASE_URL must not be empty")
	}
	if cfg.RedisURL == "" {
		return Config{}, errors.New("OPSK_REDIS_URL must not be empty")
	}

	return cfg, nil
}

func (c Config) ServiceName(component string) string {
	return c.Prefix + "-" + component
}

func (c Config) HTTPBasePath() string {
	return "/" + c.Prefix
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
