package main

import (
	"context"
	"encoding/json"
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
	"opskeeper/backend/application"
	"opskeeper/backend/audit"
	"opskeeper/backend/authorization"
	"opskeeper/backend/config"
	"opskeeper/backend/connector"
	"opskeeper/backend/diagnosis"
	"opskeeper/backend/engine"
	"opskeeper/backend/health"
	"opskeeper/backend/httpapi"
	"opskeeper/backend/identity"
	"opskeeper/backend/inspection"
	"opskeeper/backend/llm"
	"opskeeper/backend/logging"
	"opskeeper/backend/mcp"
	"opskeeper/backend/observability"
	"opskeeper/backend/organization"
	"opskeeper/backend/persona"
	repositorysvc "opskeeper/backend/repository"
	"opskeeper/backend/resource"
	"opskeeper/backend/secret"
	"opskeeper/backend/skill"
	"opskeeper/backend/version"
	"opskeeper/backend/webui"
)

func resourceAllowedForContext(ctx context.Context, scopeID, resourceID string) bool {
	if filter, restricted := authorization.ResourceFilterFromContext(ctx); restricted {
		return filter.Allows(scopeID, resourceID)
	}
	if filter, restricted := authorization.ScopeFilterFromContext(ctx); restricted {
		return filter.Allows(scopeID)
	}
	return true
}

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
		logger.Error("api stopped", "kind", "error", "error_type", "api-stopped", "error_summary", apiErrorSummary(err))
		os.Exit(1)
	}
}

func apiErrorSummary(err error) string {
	if err == nil {
		return ""
	}
	const maxLength = 500
	summary := strings.Join(strings.Fields(err.Error()), " ")
	if len(summary) > maxLength {
		return summary[:maxLength] + "..."
	}
	return summary
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

	checks := []health.Check{{Name: "postgres", Run: pool.Ping}}
	var redisClient *redis.Client
	if cfg.CacheBackend == "redis" {
		redisOptions, err := redis.ParseURL(cfg.RedisURL)
		if err != nil {
			return errors.Join(errors.New("configure Redis client"), err)
		}
		redisClient = redis.NewClient(redisOptions)
		defer func() {
			if closeErr := redisClient.Close(); closeErr != nil {
				logger.Warn("close Redis client", "kind", "error", "error_type", "redis-close", "error", closeErr)
			}
		}()
		checks = append(checks, health.Check{Name: "redis", Run: func(checkCtx context.Context) error { return redisClient.Ping(checkCtx).Err() }})
	}
	healthService := health.NewService(serviceName, cfg.DependencyTimeout, checks)
	organizationStore := organization.NewStore(pool)
	organizationService := organization.NewService(organizationStore)
	identityStore := identity.NewStore(pool)
	auditService := audit.NewService(audit.NewStore(pool))
	identityService := identity.NewService(identityStore, cfg.SessionAccessTTL, cfg.SessionRefreshTTL, auditService)
	var authorizationCache authorization.ScopeCache
	switch cfg.CacheBackend {
	case "memory":
		authorizationCache = authorization.NewMemoryScopeCache()
	case "postgres":
		authorizationCache = authorization.NewPostgresScopeCache(pool)
	case "redis":
		authorizationCache = authorization.NewRedisScopeCache(redisClient)
	}
	authorizationStore := authorization.NewStore(pool, authorizationCache)
	authorizationService := authorization.NewService(authorizationStore)
	managementStore := authorization.NewManagementStore(pool)
	managementService := authorization.NewManagementService(managementStore, authorizationService, auditService)
	credentialEncryptor, err := secret.FromEnvironment(cfg.Environment)
	if err != nil {
		return errors.Join(errors.New("configure credential encryption"), err)
	}
	resourceService := resource.NewService(resource.NewStore(pool), credentialEncryptor)
	applicationService := application.NewService(application.NewStore(pool))
	if cfg.RepositoryStorageBackend == "s3" {
		logger.Warn("repository S3-compatible backend configured", "kind", "repository-storage", "provider", cfg.RepositoryS3Provider)
	}
	repositoryService := repositorysvc.NewServiceWithStorage(repositorysvc.StorageConfig{Backend: cfg.RepositoryStorageBackend, Root: cfg.RepositoryLocalRoot, Endpoint: cfg.RepositoryS3Endpoint, Bucket: cfg.RepositoryS3Bucket, Prefix: cfg.RepositoryS3Prefix, AccessKey: cfg.RepositoryS3AccessKey, SecretKey: cfg.RepositoryS3SecretKey, Provider: cfg.RepositoryS3Provider, UseSSL: cfg.RepositoryS3UseSSL, Postgres: pool}, cfg.RepositoryMaxBundleBytes, resourceService)
	connectorLimits := connector.DefaultLimits()
	connectorLimits.Timeout = cfg.ConnectorTimeout
	connectorLimits.MaxConcurrent = cfg.ConnectorMaxConcurrency
	connectorLimits.MaxResponseBytes = cfg.ConnectorMaxResponseBytes
	connectorRegistry, err := connector.DefaultRegistry(connectorLimits)
	if err != nil {
		return fmt.Errorf("build connector registry: %w", err)
	}
	connectorService := connector.NewService(connectorRegistry, resourceService, connector.NewStore(pool), connectorLimits)
	connectorService.SetPostgresPool(pool)
	llmService := llm.NewService(llm.NewStore(pool), llm.NewProviderStore(pool, credentialEncryptor))
	skillStore := skill.NewStore(pool)
	skillService := skill.NewService(skillStore, skill.NewCatalog(pool), resourceService)
	personaVersions := persona.NewVersionStore(pool)
	personaCatalog := persona.NewPersonaCatalog(pool)
	personaService := persona.NewService(personaVersions, personaCatalog)
	personaResolver := persona.NewResolver(personaCatalog, personaVersions)
	inspectionService := inspection.NewService(inspection.NewStore(pool), resourceService, personaCatalog)
	mcpService := mcp.NewServiceWithSecurity(resourceService, mcp.NewStore(pool), cfg.MCPEnhancedSecurity)
	connectorProvider := connectorService.EngineProvider()
	mcpProvider := mcpService.EngineProvider()
	contextTooling := engine.NewContextTooling(
		engine.ResourceServiceReader{Reader: resourceService},
		connectorProvider,
		mcpProvider,
	)
	aiStore := engine.NewPostgresStore(pool)
	workflowRunStore := engine.NewPostgresWorkflowRunStore(pool)
	workflowRetriever := engine.KnowledgeRetrieverFunc(func(queryCtx context.Context, query engine.KnowledgeQuery) (engine.RetrievalResult, error) {
		item, getErr := resourceService.Get(queryCtx, query.KnowledgeBaseID)
		if getErr != nil {
			return engine.RetrievalResult{}, getErr
		}
		if item.Kind != "KnowledgeBase" || item.ScopeID != query.ScopeID || !resourceAllowedForContext(queryCtx, item.ScopeID, item.ID) {
			return engine.RetrievalResult{}, authorization.ErrForbidden
		}
		encoded, marshalErr := json.Marshal(item.Config)
		if marshalErr != nil {
			return engine.RetrievalResult{}, fmt.Errorf("knowledge base config is invalid: %w", marshalErr)
		}
		var base engine.KnowledgeBase
		if err := json.Unmarshal(encoded, &base); err != nil {
			return engine.RetrievalResult{}, fmt.Errorf("knowledge base config is invalid: %w", err)
		}
		return engine.SearchDocuments(query, base)
	})
	contextTooling.Gateway.AuditStore = aiStore
	modelBuilder := func(ctx context.Context, scopeID, providerID, modelName string, purpose engine.Purpose) (engine.ModelBuildResult, error) {
		resolved, client, err := llmService.BuildModel(ctx, scopeID, providerID, modelName, llm.Purpose(purpose))
		if err != nil {
			return engine.ModelBuildResult{}, err
		}
		return engine.ModelBuildResult{Client: client, ProviderID: resolved.Provider.ID, ModelName: resolved.Model.Name, Capabilities: resolved.Model.Capabilities, ContextWindowTokens: resolved.Model.ContextWindowTokens, MaxOutputTokens: resolved.Model.MaxOutputTokens, Temperature: resolved.Model.Temperature}, nil
	}
	engineRuntime := engine.NewWithContextAndStore(engine.NewAgentRunner(modelBuilder), contextTooling.Resolver, contextTooling.Gateway, aiStore).
		WithPersonaResolver(personaResolver).
		WithPlanResolver(skillService)
	engineCatalog := engine.NewCatalogService(engine.NewCatalogStore(pool))
	diagnosisService := diagnosis.NewOrchestrator(diagnosis.NewService(diagnosis.NewStore(pool), resourceService, applicationService), engineRuntime, 30*time.Minute)
	workflowService := engine.NewWorkflowService(workflowRunStore, engineRuntime, contextTooling.Gateway, workflowRetriever, aiStore)

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
			Connectors:         connectorService,
			LLMs:               llmService,
			Skills:             skillService,
			Personas:           personaService,
			Engine:             engineRuntime,
			EngineCatalog:      engineCatalog,
			EngineEvents:       aiStore,
			EngineToolCalls:    aiStore,
			WorkflowRuns:       workflowRunStore,
			WorkflowExecutor:   workflowService,
			Diagnosis:          diagnosisService,
			Inspection:         inspectionService,
			MCP:                mcpService,
			RepositoryBundles:  repositoryService,
			Applications:       applicationService,
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
	accessAddress := accessURL(listenAddress, cfg.BasePath)
	serverErr := make(chan error, 1)
	go func() {
		logger.Info("api listening", "kind", "service-start", "listen", listenAddress, "url", accessAddress, "base_path", cfg.BasePath, "environment", cfg.Environment)
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

// accessURL turns the bound listener address into a browser-friendly URL.
// Wildcard addresses describe the bind interface, so localhost is clearer for
// local development while preserving an explicitly configured host.
func accessURL(listenAddress, basePath string) string {
	host, port, err := net.SplitHostPort(listenAddress)
	if err != nil || port == "" {
		return "http://" + strings.TrimRight(listenAddress, "/") + normalizedBasePath(basePath)
	}
	if host == "" || host == "0.0.0.0" || host == "::" || host == "[::]" {
		host = "localhost"
	}
	return "http://" + net.JoinHostPort(host, port) + normalizedBasePath(basePath)
}

func normalizedBasePath(basePath string) string {
	basePath = strings.TrimSpace(basePath)
	if basePath == "" || basePath == "/" {
		return "/"
	}
	return "/" + strings.Trim(basePath, "/") + "/"
}

func envOrDefault(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok && strings.TrimSpace(value) != "" {
		return value
	}
	return fallback
}
