package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"opskeeper/backend/audit"
	"opskeeper/backend/authorization"
	"opskeeper/backend/config"
	"opskeeper/backend/connector"
	"opskeeper/backend/credential"
	"opskeeper/backend/discovery"
	"opskeeper/backend/health"
	"opskeeper/backend/httpapi"
	"opskeeper/backend/identity"
	"opskeeper/backend/llm"
	"opskeeper/backend/logging"
	"opskeeper/backend/organization"
	"opskeeper/backend/resource"
	"opskeeper/backend/skill"
	"opskeeper/backend/version"
	"opskeeper/backend/webui"
)

const serviceName = "opskeeper-api"

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
	if err := run(logger, cfg); err != nil {
		logger.Error("api stopped", "error", err)
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

	server := &http.Server{
		Addr: cfg.HTTPAddress,
		Handler: httpapi.NewRouter(logger, healthService, version.Current(), httpapi.Options{
			BasePath:       cfg.BasePath,
			TrustedProxies: cfg.TrustedProxies,
			Identity:       identityService,
			Users:          identityService,
			Authorization:  authorizationService,
			Access:         managementService,
			Auditor:        auditService,
			AuditLog:       auditService,
			Resources:      resourceService,
			Credentials:    credentialService,
			Discovery:      discoveryService,
			Connectors:     connectorService,
			LLMs:           llmService,
			Skills:         skillService,
			SkillRunner:    skillRunner,
			CookieSecure:   cfg.CookieSecure,
		}, organizationService, webUI),
		ReadHeaderTimeout: cfg.ReadHeaderTimeout,
		ReadTimeout:       cfg.ReadTimeout,
		WriteTimeout:      cfg.WriteTimeout,
		IdleTimeout:       cfg.IdleTimeout,
	}

	serverErr := make(chan error, 1)
	go func() {
		logger.Info("api listening", "address", cfg.HTTPAddress, "base_path", cfg.BasePath, "environment", cfg.Environment)
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
