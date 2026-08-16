package config

import (
	"errors"
	"fmt"
	"net/netip"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	defaultDatabaseURL = "postgres://opskeeper:opskeeper@localhost:5432/opskeeper?sslmode=disable"
	defaultRedisURL    = "redis://localhost:6379/0"
	defaultBasePath    = "/opskeeper"
	defaultLogFormat   = "text"
	maxBasePathLength  = 128
)

var basePathPattern = regexp.MustCompile(`^/(?:[a-z0-9](?:[a-z0-9-]*[a-z0-9])?)(?:/[a-z0-9](?:[a-z0-9-]*[a-z0-9])?)*$`)

type Config struct {
	BasePath                     string
	Environment                  string
	LogFormat                    string
	HTTPAddress                  string
	TrustedProxies               []netip.Prefix
	DatabaseURL                  string
	RedisURL                     string
	ShutdownTimeout              time.Duration
	DependencyTimeout            time.Duration
	ReadHeaderTimeout            time.Duration
	ReadTimeout                  time.Duration
	WriteTimeout                 time.Duration
	IdleTimeout                  time.Duration
	CookieSecure                 bool
	SessionAccessTTL             time.Duration
	SessionRefreshTTL            time.Duration
	ConnectorTimeout             time.Duration
	ConnectorMaxConcurrency      int
	ConnectorMaxResponseBytes    int64
	InspectionScheduleInterval   time.Duration
	InspectionWorkerPollInterval time.Duration
	InspectionLeaseDuration      time.Duration
}

func Load() (Config, error) {
	cfg := Config{
		BasePath:          envOrDefault("OPSK_BASE_PATH", defaultBasePath),
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
	if cfg.TrustedProxies, err = prefixesFromEnv("OPSK_TRUSTED_PROXIES"); err != nil {
		return Config{}, err
	}
	if cfg.CookieSecure, err = boolFromEnv("OPSK_COOKIE_SECURE", cfg.Environment == "production"); err != nil {
		return Config{}, err
	}
	if cfg.SessionAccessTTL, err = durationFromEnv("OPSK_SESSION_ACCESS_TTL", 15*time.Minute); err != nil {
		return Config{}, err
	}
	if cfg.SessionRefreshTTL, err = durationFromEnv("OPSK_SESSION_REFRESH_TTL", 7*24*time.Hour); err != nil {
		return Config{}, err
	}
	if cfg.ConnectorTimeout, err = durationFromEnv("OPSK_CONNECTOR_TIMEOUT", 10*time.Second); err != nil {
		return Config{}, err
	}
	if cfg.ConnectorMaxConcurrency, err = intFromEnv("OPSK_CONNECTOR_MAX_CONCURRENCY", 8, 1, 128); err != nil {
		return Config{}, err
	}
	if cfg.ConnectorMaxResponseBytes, err = int64FromEnv("OPSK_CONNECTOR_MAX_RESPONSE_BYTES", 4<<20, 1024, 64<<20); err != nil {
		return Config{}, err
	}
	if cfg.InspectionScheduleInterval, err = durationFromEnv("OPSK_INSPECTION_SCHEDULE_INTERVAL", 15*time.Second); err != nil {
		return Config{}, err
	}
	if cfg.InspectionWorkerPollInterval, err = durationFromEnv("OPSK_INSPECTION_WORKER_POLL_INTERVAL", 2*time.Second); err != nil {
		return Config{}, err
	}
	if cfg.InspectionLeaseDuration, err = durationFromEnv("OPSK_INSPECTION_LEASE_DURATION", 45*time.Second); err != nil {
		return Config{}, err
	}

	if cfg.HTTPAddress == "" {
		return Config{}, errors.New("OPSK_HTTP_ADDRESS must not be empty")
	}
	if !validBasePath(cfg.BasePath) {
		return Config{}, fmt.Errorf("OPSK_BASE_PATH must be / or a slash-prefixed path of lowercase letters, digits, or internal hyphens (maximum %d characters)", maxBasePathLength)
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
	if cfg.Environment == "production" && !cfg.CookieSecure {
		return Config{}, errors.New("OPSK_COOKIE_SECURE must be true in production")
	}
	if cfg.SessionRefreshTTL <= cfg.SessionAccessTTL {
		return Config{}, errors.New("OPSK_SESSION_REFRESH_TTL must be greater than OPSK_SESSION_ACCESS_TTL")
	}

	return cfg, nil
}

func prefixesFromEnv(key string) ([]netip.Prefix, error) {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return nil, nil
	}

	prefixes := make([]netip.Prefix, 0)
	seen := make(map[string]struct{})
	for _, rawEntry := range strings.Split(value, ",") {
		entry := strings.TrimSpace(rawEntry)
		if entry == "" {
			return nil, fmt.Errorf("%s must be a comma-separated list of IP addresses or CIDR prefixes", key)
		}
		prefix, err := parsePrefix(entry)
		if err != nil {
			return nil, fmt.Errorf("parse %s entry %q: %w", key, entry, err)
		}
		canonical := prefix.String()
		if _, exists := seen[canonical]; exists {
			continue
		}
		seen[canonical] = struct{}{}
		prefixes = append(prefixes, prefix)
	}
	return prefixes, nil
}

func parsePrefix(value string) (netip.Prefix, error) {
	if strings.Contains(value, "/") {
		prefix, err := netip.ParsePrefix(value)
		if err != nil {
			return netip.Prefix{}, err
		}
		return prefix.Masked(), nil
	}
	address, err := netip.ParseAddr(value)
	if err != nil {
		return netip.Prefix{}, err
	}
	if address.Zone() != "" {
		return netip.Prefix{}, errors.New("scoped IPv6 addresses are not supported")
	}
	address = address.Unmap()
	return netip.PrefixFrom(address, address.BitLen()), nil
}

func envOrDefault(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

func validBasePath(value string) bool {
	return value == "/" || (len(value) <= maxBasePathLength && basePathPattern.MatchString(value))
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

func boolFromEnv(key string, fallback bool) (bool, error) {
	value, ok := os.LookupEnv(key)
	if !ok {
		return fallback, nil
	}
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "on":
		return true, nil
	case "0", "false", "no", "off":
		return false, nil
	default:
		return false, fmt.Errorf("parse %s: expected true or false", key)
	}
}

func intFromEnv(key string, fallback, minimum, maximum int) (int, error) {
	value, ok := os.LookupEnv(key)
	if !ok {
		return fallback, nil
	}
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || parsed < minimum || parsed > maximum {
		return 0, fmt.Errorf("%s must be an integer between %d and %d", key, minimum, maximum)
	}
	return parsed, nil
}

func int64FromEnv(key string, fallback, minimum, maximum int64) (int64, error) {
	value, ok := os.LookupEnv(key)
	if !ok {
		return fallback, nil
	}
	parsed, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil || parsed < minimum || parsed > maximum {
		return 0, fmt.Errorf("%s must be an integer between %d and %d", key, minimum, maximum)
	}
	return parsed, nil
}
