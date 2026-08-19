package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"opskeeper/backend/audit"
	"opskeeper/backend/authorization"
	"opskeeper/backend/config"
	"opskeeper/backend/connector"
	"opskeeper/backend/credential"
	"opskeeper/backend/diagnosis"
	"opskeeper/backend/discovery"
	"opskeeper/backend/health"
	"opskeeper/backend/httpapi"
	"opskeeper/backend/identity"
	"opskeeper/backend/inspection"
	"opskeeper/backend/llm"
	"opskeeper/backend/logging"
	"opskeeper/backend/mcp"
	"opskeeper/backend/observability"
	"opskeeper/backend/operation"
	"opskeeper/backend/organization"
	"opskeeper/backend/resource"
	"opskeeper/backend/skill"
	"opskeeper/backend/version"
	"opskeeper/backend/webui"
)

const serviceName = "opskeeper-api"

func main() {
	logger := logging.NewRaw(os.Stdout).With("service", serviceName)
	cfg, err := config.Load()
	if err != nil {
		logger.Error("load configuration", "kind", "error", "error_type", "configuration", "error", err)
		os.Exit(1)
	}
	logger, err = logging.New(os.Stdout, cfg.LogFormat)
	if err != nil {
		logger.Error("configure logging", "kind", "error", "error_type", "logging", "error", err)
		os.Exit(1)
	}
	logger = logger.With("service", serviceName)
	if err := run(logger, cfg); err != nil {
		logger.Error("api stopped", "kind", "error", "error_type", "api-stopped", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger, cfg config.Config) error {
	webUI, err := webui.New(cfg.BasePath)
	if err != nil {
		return errors.Join(errors.New("configure web UI"), err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	build := version.Current()
	shutdownTelemetry, err := observability.Setup(ctx, serviceName, cfg.Environment, cfg.OTLPExporterEndpoint, observability.Build{Version: build.Version, Commit: build.Commit})
	if err != nil {
		return errors.Join(errors.New("configure telemetry"), err)
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := shutdownTelemetry(shutdownCtx); err != nil {
			logger.Warn("shutdown telemetry", "kind", "error", "error_type", "telemetry-shutdown", "error", err)
		}
	}()

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
			logger.Warn("close Redis client", "kind", "error", "error_type", "redis-close", "error", closeErr)
		}
	}()

	healthService := health.NewService(serviceName, cfg.DependencyTimeout, []health.Check{
		{Name: "postgres", Run: pool.Ping},
		{Name: "redis", Run: func(checkCtx context.Context) error {
			return redisClient.Ping(checkCtx).Err()
		}},
	})
	organizationStore := organization.NewStore(pool)
	organizationService := organization.NewService(organizationStore)
	identityStore := identity.NewStore(pool)
	auditService := audit.NewService(audit.NewStore(pool))
	identityService := identity.NewService(identityStore, cfg.SessionAccessTTL, cfg.SessionRefreshTTL, auditService)
	authorizationCache := authorization.NewRedisScopeCache(redisClient)
	authorizationStore := authorization.NewStore(pool, authorizationCache)
	authorizationService := authorization.NewService(authorizationStore)
	managementStore := authorization.NewManagementStore(pool)
	managementService := authorization.NewManagementService(managementStore, authorizationService, auditService)
	credentialEncryptor, err := credential.FromEnvironment(cfg.Environment)
	if err != nil {
		return errors.Join(errors.New("configure credential encryption"), err)
	}
	credentialService := credential.NewService(credential.NewStore(pool), credentialEncryptor)
	resourceService := resource.NewService(resource.NewStore(pool))
	discoveryService := discovery.NewService(discovery.NewStore(pool), resourceService, resourceService, organizationService, credentialService, discovery.NewKubernetesScanner())
	connectorLimits := connector.DefaultLimits()
	connectorLimits.Timeout = cfg.ConnectorTimeout
	connectorLimits.MaxConcurrent = cfg.ConnectorMaxConcurrency
	connectorLimits.MaxResponseBytes = cfg.ConnectorMaxResponseBytes
	connectorRegistry, err := connector.DefaultRegistry(connectorLimits)
	if err != nil {
		return fmt.Errorf("build connector registry: %w", err)
	}
	connectorService := connector.NewService(connectorRegistry, resourceService, credentialService, connector.NewStore(pool), connectorLimits)
	llmService := llm.NewService(llm.NewStore(pool), resourceService, credentialService)
	skillStore := skill.NewStore(pool)
	skillService := skill.NewService(skillStore, resourceService)
	skillRunner := skill.NewRunner(skillService, llmService, connectorService, skillStore)
	diagnosisService := diagnosis.NewOrchestrator(diagnosis.NewService(diagnosis.NewStore(pool), resourceService), skillRunner, 2*time.Minute)
	inspectionService := inspection.NewService(inspection.NewStore(pool), resourceService)
	mcpService := mcp.NewService(resourceService, mcp.NewStore(pool))
	operationStore := operation.NewStore(pool)
	var operationService *operation.Service
	if strings.EqualFold(strings.TrimSpace(os.Getenv("OPSK_OPERATION_SUBMITTER_ENABLED")), "true") {
		operationService = operation.NewServiceWithSubmitter(operationStore, resourceService, operation.NewInClusterSubmitter(resourceService, envOrDefault("OPSK_OPERATION_RUNNER_IMAGE", "opskeeper:local")))
	} else {
		operationService = operation.NewService(operationStore, resourceService)
	}

	server := &http.Server{
		Addr: cfg.HTTPAddress,
		Handler: httpapi.NewRouter(logger, healthService, build, httpapi.Options{
			BasePath:           cfg.BasePath,
			TrustedProxies:     cfg.TrustedProxies,
			Identity:           identityService,
			Users:              identityService,
			Authorization:      authorizationService,
			Access:             managementService,
			Auditor:            auditService,
			AuditLog:           auditService,
			Resources:          resourceService,
			Credentials:        credentialService,
			Discovery:          discoveryService,
			Connectors:         connectorService,
			LLMs:               llmService,
			Skills:             skillService,
			SkillRunner:        skillRunner,
			Diagnosis:          diagnosisService,
			Inspection:         inspectionService,
			MCP:                mcpService,
			Operations:         operationService,
			CookieSecure:       cfg.CookieSecure,
			Production:         cfg.Environment == "production",
			AllowedOrigins:     cfg.AllowedOrigins,
			MaxBodyBytes:       cfg.HTTPMaxBodyBytes,
			RateLimitPerMinute: cfg.HTTPRateLimitPerMinute,
			LogHealthIgnore:    cfg.LogHealthIgnore,
		}, organizationService, webUI),
		ReadHeaderTimeout: cfg.ReadHeaderTimeout,
		ReadTimeout:       cfg.ReadTimeout,
		WriteTimeout:      cfg.WriteTimeout,
		IdleTimeout:       cfg.IdleTimeout,
	}
	listener, err := net.Listen("tcp", cfg.HTTPAddress)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", cfg.HTTPAddress, err)
	}
	defer listener.Close()
	listenAddress := listener.Addr().String()
	serverErr := make(chan error, 1)
	go func() {
		logger.Info("api listening", "kind", "service-start", "listen", listenAddress, "base_path", cfg.BasePath, "environment", cfg.Environment)
		serverErr <- server.Serve(listener)
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

func envOrDefault(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok && strings.TrimSpace(value) != "" {
		return value
	}
	return fallback
}
