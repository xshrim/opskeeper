package config

import (
	"errors"
	"fmt"
	"log/slog"
	"net/netip"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	defaultDBHost     = "localhost:5432"
	defaultDBName     = "opskeeper"
	defaultDBUser     = "opskeeper"
	defaultDBPassword = "opskeeper"
	defaultDBSSLMode  = "disable"
	defaultRedisURL   = "redis://localhost:6379/0"
	defaultBasePath   = "/opskeeper"
	defaultLogFormat  = "raw"
	maxBasePathLength = 128
)

var basePathPattern = regexp.MustCompile(`^/(?:[a-z0-9](?:[a-z0-9-]*[a-z0-9])?)(?:/[a-z0-9](?:[a-z0-9-]*[a-z0-9])?)*$`)

type Config struct {
	DiagnosisCollaboration        int
	DiagnosisMaxSubagents         int
	DiagnosisSubagentConcurrency  int
	BasePath                      string
	Environment                   string
	LogFormat                     string
	LogHealthIgnore               bool
	ListenPort                    int
	TrustedProxies                []netip.Prefix
	DatabaseURL                   string
	RedisURL                      string
	CacheBackend                  string
	ShutdownTimeout               time.Duration
	DependencyTimeout             time.Duration
	ReadHeaderTimeout             time.Duration
	ReadTimeout                   time.Duration
	WriteTimeout                  time.Duration
	IdleTimeout                   time.Duration
	AllowedOrigins                []string
	HTTPMaxBodyBytes              int64
	HTTPRateLimitPerMinute        int
	CookieSecure                  bool
	OTLPExporterEndpoint          string
	SessionAccessTTL              time.Duration
	SessionRefreshTTL             time.Duration
	ConnectorTimeout              time.Duration
	ConnectorMaxConcurrency       int
	ConnectorMaxResponseBytes     int64
	InspectionScheduleInterval    time.Duration
	InspectionWorkerPollInterval  time.Duration
	InspectionLeaseDuration       time.Duration
	NotificationQueueBackend      string
	NotificationQueuePollInterval time.Duration
	MCPEnhancedSecurity           bool
	StorageBackend                string
	RepositoryLocalRoot           string
	StorageS3Endpoint             string
	StorageS3Bucket               string
	StorageS3Prefix               string
	StorageS3AccessKey            string
	StorageS3SecretKey            string
	StorageS3UseSSL               bool
	StorageS3Provider             string
	RepositoryMaxBundleBytes      int64
}

func Load() (Config, error) {
	cfg := Config{
		DiagnosisCollaboration:       safeIntFromEnv("OPSK_DIAGNOSIS_COLLABORATION", 2, 1, 5),
		DiagnosisMaxSubagents:        safeIntFromEnv("OPSK_DIAGNOSIS_MAX_SUBAGENTS", 24, 1, 128),
		DiagnosisSubagentConcurrency: safeIntFromEnv("OPSK_DIAGNOSIS_SUBAGENT_CONCURRENCY", 4, 1, 16),
		BasePath:                     envOrDefault("OPSK_BASE_PATH", defaultBasePath),
		Environment:                  envOrDefault("OPSK_ENVIRONMENT", "development"),
		LogFormat:                    envOrDefault("OPSK_LOG_FORMAT", defaultLogFormat),
		RedisURL:                     strings.TrimSpace(os.Getenv("OPSK_REDIS_URL")),
		CacheBackend:                 envOrDefault("OPSK_CACHE_BACKEND", "postgres"),
		NotificationQueueBackend:     envOrDefault("OPSK_MQ_BACKEND", "postgres"),
		ReadHeaderTimeout:            5 * time.Second,
		ReadTimeout:                  15 * time.Second,
		// Diagnosis uses a long-lived SSE response. Keep the server-level write
		// deadline above the Engine's 30-minute execution budget so an idle
		// tool/model turn cannot terminate the stream prematurely.
		WriteTimeout:         35 * time.Minute,
		IdleTimeout:          60 * time.Second,
		OTLPExporterEndpoint: strings.TrimSpace(os.Getenv("OPSK_OTEL_ENDPOINT")),
		StorageBackend:       envOrDefault("OPSK_STORAGE_BACKEND", "postgres"),
		RepositoryLocalRoot:  envOrDefault("OPSK_REPOSITORY_LOCAL_ROOT", "/var/lib/opskeeper/repositories"),
		StorageS3Endpoint:    strings.TrimSpace(os.Getenv("OPSK_STORAGE_S3_ENDPOINT")),
		StorageS3Bucket:      strings.TrimSpace(os.Getenv("OPSK_STORAGE_S3_BUCKET")),
		StorageS3Prefix:      envOrDefault("OPSK_STORAGE_S3_PREFIX", "repositories"),
		StorageS3AccessKey:   strings.TrimSpace(os.Getenv("OPSK_STORAGE_S3_ACCESS_KEY")),
		StorageS3SecretKey:   strings.TrimSpace(os.Getenv("OPSK_STORAGE_S3_SECRET_KEY")),
		StorageS3Provider:    envOrDefault("OPSK_STORAGE_S3_PROVIDER", "minio"),
	}
	var databaseErr error
	cfg.DatabaseURL, databaseErr = databaseURLFromEnv()
	if databaseErr != nil {
		return Config{}, databaseErr
	}

	var err error
	if cfg.ListenPort, err = intFromEnv("OPSK_LISTEN_PORT", 8080, 1, 65535); err != nil {
		return Config{}, err
	}
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
	if cfg.LogHealthIgnore, err = boolFromEnv("OPSK_LOG_HEALTH_IGNORE", true); err != nil {
		return Config{}, err
	}
	if cfg.AllowedOrigins, err = originsFromEnv("OPSK_ALLOWED_ORIGINS"); err != nil {
		return Config{}, err
	}
	if cfg.HTTPMaxBodyBytes, err = int64FromEnv("OPSK_HTTP_MAX_BODY_BYTES", 5<<20, 1024, 64<<20); err != nil {
		return Config{}, err
	}
	if cfg.HTTPRateLimitPerMinute, err = intFromEnv("OPSK_HTTP_RATE_LIMIT_PER_MINUTE", 600, 1, 100000); err != nil {
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
	if cfg.NotificationQueuePollInterval, err = durationFromEnv("OPSK_NOTIFICATION_QUEUE_POLL_INTERVAL", 5*time.Second); err != nil {
		return Config{}, err
	}
	if cfg.MCPEnhancedSecurity, err = boolFromEnv("OPSK_MCP_ENHANCED_SECURITY", false); err != nil {
		return Config{}, err
	}
	if cfg.RepositoryMaxBundleBytes, err = int64FromEnv("OPSK_REPOSITORY_MAX_BUNDLE_BYTES", 512<<20, 1<<20, 4<<30); err != nil {
		return Config{}, err
	}
	if cfg.CacheBackend != "memory" && cfg.CacheBackend != "postgres" && cfg.CacheBackend != "redis" {
		return Config{}, errors.New("OPSK_CACHE_BACKEND must be memory, postgres or redis")
	}
	if cfg.NotificationQueueBackend != "postgres" {
		return Config{}, errors.New("OPSK_MQ_BACKEND must be postgres")
	}
	if cfg.StorageBackend != "local" && cfg.StorageBackend != "postgres" && cfg.StorageBackend != "s3" {
		return Config{}, errors.New("OPSK_STORAGE_BACKEND must be local, postgres or s3")
	}
	if cfg.StorageBackend == "s3" && (cfg.StorageS3Endpoint == "" || cfg.StorageS3Bucket == "") {
		return Config{}, errors.New("OPSK_STORAGE_S3_ENDPOINT and OPSK_STORAGE_S3_BUCKET are required for s3 backend")
	}
	if strings.TrimSpace(cfg.StorageS3Provider) == "" {
		return Config{}, errors.New("OPSK_STORAGE_S3_PROVIDER must not be empty")
	}
	if cfg.StorageS3UseSSL, err = boolFromEnv("OPSK_STORAGE_S3_USE_SSL", false); err != nil {
		return Config{}, err
	}

	if !validBasePath(cfg.BasePath) {
		return Config{}, fmt.Errorf("OPSK_BASE_PATH must be / or a slash-prefixed path of lowercase letters, digits, or internal hyphens (maximum %d characters)", maxBasePathLength)
	}
	if cfg.LogFormat != "raw" && cfg.LogFormat != "text" && cfg.LogFormat != "json" {
		return Config{}, errors.New("OPSK_LOG_FORMAT must be raw, text, or json")
	}
	if cfg.CacheBackend == "redis" && cfg.RedisURL == "" {
		return Config{}, errors.New("OPSK_REDIS_URL must not be empty when OPSK_CACHE_BACKEND=redis")
	}
	if cfg.Environment == "production" && !cfg.CookieSecure {
		return Config{}, errors.New("OPSK_COOKIE_SECURE must be true in production")
	}
	if cfg.Environment == "production" {
		if cfg.DatabaseURL == developmentDatabaseURL() {
			return Config{}, errors.New("OPSK_DB_HOST, OPSK_DB_NAME, OPSK_DB_USER, and OPSK_DB_PASSWORD must not use development defaults in production")
		}
		if cfg.CacheBackend == "redis" && cfg.RedisURL == defaultRedisURL {
			return Config{}, errors.New("OPSK_REDIS_URL must not use the development default in production")
		}
		for _, origin := range cfg.AllowedOrigins {
			if !strings.HasPrefix(origin, "https://") {
				return Config{}, errors.New("OPSK_ALLOWED_ORIGINS must use https in production")
			}
		}
	}
	if cfg.SessionRefreshTTL <= cfg.SessionAccessTTL {
		return Config{}, errors.New("OPSK_SESSION_REFRESH_TTL must be greater than OPSK_SESSION_ACCESS_TTL")
	}

	return cfg, nil
}

func databaseURLFromEnv() (string, error) {
	host := strings.TrimSpace(envOrDefault("OPSK_DB_HOST", defaultDBHost))
	name := strings.TrimSpace(envOrDefault("OPSK_DB_NAME", defaultDBName))
	user := strings.TrimSpace(envOrDefault("OPSK_DB_USER", defaultDBUser))
	password := os.Getenv("OPSK_DB_PASSWORD")
	if password == "" {
		password = defaultDBPassword
	}
	sslMode := strings.TrimSpace(envOrDefault("OPSK_DB_SSLMODE", defaultDBSSLMode))
	if host == "" || name == "" || user == "" || strings.ContainsAny(host, " \t\r\n") {
		return "", errors.New("OPSK_DB_HOST must not be empty or contain whitespace; OPSK_DB_NAME and OPSK_DB_USER must not be empty")
	}
	switch sslMode {
	case "disable", "allow", "prefer", "require", "verify-ca", "verify-full":
	default:
		return "", errors.New("OPSK_DB_SSLMODE must be disable, allow, prefer, require, verify-ca, or verify-full")
	}
	return (&url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(user, password),
		Host:     host,
		Path:     "/" + name,
		RawQuery: "sslmode=" + url.QueryEscape(sslMode),
	}).String(), nil
}

func developmentDatabaseURL() string {
	return (&url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(defaultDBUser, defaultDBPassword),
		Host:     defaultDBHost,
		Path:     "/" + defaultDBName,
		RawQuery: "sslmode=" + defaultDBSSLMode,
	}).String()
}

func safeIntFromEnv(key string, fallback, minimum, maximum int) int {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < minimum || value > maximum {
		slog.Error("invalid diagnosis configuration; using safe default", "kind", "configuration", "key", key, "default", fallback)
		return fallback
	}
	return value
}

func originsFromEnv(key string) ([]string, error) {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return nil, nil
	}
	origins := make([]string, 0)
	seen := make(map[string]struct{})
	for _, raw := range strings.Split(value, ",") {
		parsed, err := url.Parse(strings.TrimSpace(raw))
		if err != nil || parsed.Scheme == "" || parsed.Host == "" || parsed.User != nil || parsed.Path != "" || parsed.RawQuery != "" || parsed.Fragment != "" {
			return nil, fmt.Errorf("%s entries must be origins such as https://ops.example.com", key)
		}
		if parsed.Scheme != "https" && parsed.Scheme != "http" {
			return nil, fmt.Errorf("%s entries must use http or https", key)
		}
		origin := strings.ToLower(parsed.Scheme) + "://" + strings.ToLower(parsed.Host)
		if _, exists := seen[origin]; !exists {
			seen[origin] = struct{}{}
			origins = append(origins, origin)
		}
	}
	return origins, nil
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
