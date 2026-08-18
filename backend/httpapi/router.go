package httpapi

import (
	"log/slog"
	"net/http"
	"net/netip"
	"path"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"opskeeper/backend/audit"
	"opskeeper/backend/authorization"
	"opskeeper/backend/health"
	"opskeeper/backend/version"
)

type Options struct {
	BasePath           string
	TrustedProxies     []netip.Prefix
	Identity           identityService
	Users              userManagementService
	Authorization      authorizationService
	Access             accessManagementService
	Auditor            audit.Logger
	AuditLog           auditQueryService
	Resources          resourceService
	Credentials        credentialService
	Discovery          discoveryService
	Connectors         connectorService
	LLMs               llmService
	Skills             skillService
	SkillRunner        skillRunner
	Diagnosis          diagnosisService
	Inspection         inspectionService
	MCP                mcpService
	Operations         operationService
	CookieSecure       bool
	Production         bool
	AllowedOrigins     []string
	MaxBodyBytes       int64
	RateLimitPerMinute int
	LogHealthIgnore    bool
}

func NewRouter(logger *slog.Logger, healthService *health.Service, build version.Info, options Options, organizationService organizationService, webUI http.Handler) http.Handler {
	basePath := options.BasePath
	router := chi.NewRouter()
	router.Use(middleware.RequestID)
	router.Use(trustedProxyClientIP(options.TrustedProxies))
	router.Use(recoverer(logger))
	router.Use(securityHeaders(options.Production))
	router.Use(corsPolicy(options.AllowedOrigins))
	router.Use(csrfProtection(options.AllowedOrigins))
	router.Use(requestBodyLimit(options.MaxBodyBytes))
	router.Use(newClientRateLimiter(options.RateLimitPerMinute).middleware)
	router.Use(requestLogger(logger, basePath, options.LogHealthIgnore))

	app := chi.NewRouter()
	app.Get("/health/live", func(writer http.ResponseWriter, _ *http.Request) {
		writeJSON(writer, http.StatusOK, health.Liveness(healthService.Name(), build))
	})
	app.Get("/health/ready", func(writer http.ResponseWriter, request *http.Request) {
		report := healthService.Readiness(request.Context(), build)
		status := http.StatusOK
		if report.Status != "ready" {
			status = http.StatusServiceUnavailable
		}
		writeJSON(writer, status, report)
	})
	if organizationService != nil || options.Identity != nil {
		app.Route("/api/v1", func(router chi.Router) {
			if organizationService != nil {
				organizationRouter := router
				var requirePermission func(authorization.Permission) func(http.Handler) http.Handler
				if options.Identity != nil {
					organizationRouter = router.With(authHandler{service: options.Identity}.requireAuth)
				}
				if options.Authorization != nil {
					authorizationMiddleware := authorizationHandler{service: options.Authorization}
					requirePermission = authorizationMiddleware.requirePermission
				}
				registerOrganizationRoutes(organizationRouter, organizationService, path.Join(basePath, "api/v1"), requirePermission)
			}
			if options.Identity != nil {
				registerAuthRoutes(router, options.Identity, basePath, options.CookieSecure)
			}
			if options.Identity != nil && (options.Users != nil || options.Access != nil) {
				managementRouter := router.With(authHandler{service: options.Identity}.requireAuth)
				registerAccessRoutes(managementRouter, options.Users, options.Access, options.Auditor, options.AuditLog)
			}
			if options.Identity != nil && options.Authorization != nil && options.AuditLog != nil {
				auditRouter := router.With(authHandler{service: options.Identity}.requireAuth)
				registerAuditAuthorizationRoutes(auditRouter, options.Authorization, options.AuditLog)
			}
			if options.Identity != nil && (options.Resources != nil || options.Credentials != nil || options.Discovery != nil || options.Connectors != nil) {
				resourceRouter := router.With(authHandler{service: options.Identity}.requireAuth)
				var requirePermission func(authorization.Permission) func(http.Handler) http.Handler
				if options.Authorization != nil {
					requirePermission = (authorizationHandler{service: options.Authorization}).requirePermission
				}
				registerResourceRoutes(resourceRouter, options.Resources, options.Credentials, options.Auditor, requirePermission)
				registerDiscoveryRoutes(resourceRouter, options.Discovery, requirePermission)
				registerConnectorRoutes(resourceRouter, options.Connectors, options.Auditor, requirePermission)
			}
			if options.Identity != nil && (options.LLMs != nil || options.Skills != nil || options.SkillRunner != nil) {
				aiRouter := router.With(authHandler{service: options.Identity}.requireAuth)
				var requirePermission func(authorization.Permission) func(http.Handler) http.Handler
				if options.Authorization != nil {
					requirePermission = (authorizationHandler{service: options.Authorization}).requirePermission
				}
				registerAIRoutes(aiRouter, options.LLMs, options.Skills, options.SkillRunner, options.Authorization, options.Auditor, requirePermission)
			}
			if options.Identity != nil && options.Diagnosis != nil {
				diagnosisRouter := router.With(authHandler{service: options.Identity}.requireAuth)
				var requirePermission func(authorization.Permission) func(http.Handler) http.Handler
				if options.Authorization != nil {
					requirePermission = (authorizationHandler{service: options.Authorization}).requirePermission
				}
				registerDiagnosisRoutes(diagnosisRouter, options.Diagnosis, options.Auditor, requirePermission)
			}
			if options.Identity != nil && options.Inspection != nil {
				inspectionRouter := router.With(authHandler{service: options.Identity}.requireAuth)
				var requirePermission func(authorization.Permission) func(http.Handler) http.Handler
				if options.Authorization != nil {
					requirePermission = (authorizationHandler{service: options.Authorization}).requirePermission
				}
				registerInspectionRoutes(inspectionRouter, options.Inspection, requirePermission)
			}
			if options.Identity != nil && (options.MCP != nil || options.Operations != nil) {
				operationRouter := router.With(authHandler{service: options.Identity}.requireAuth)
				var requirePermission func(authorization.Permission) func(http.Handler) http.Handler
				if options.Authorization != nil {
					requirePermission = (authorizationHandler{service: options.Authorization}).requirePermission
				}
				registerMCPRoutes(operationRouter, options.MCP, options.Auditor, requirePermission)
				registerOperationRoutes(operationRouter, options.Operations, options.Auditor, requirePermission)
			}
		})
	}

	app.NotFound(func(writer http.ResponseWriter, request *http.Request) {
		if webUI != nil && canServeWebUI(request, basePath) {
			webUI.ServeHTTP(writer, request)
			return
		}
		writeError(writer, request, http.StatusNotFound, "not_found", "Route not found")
	})
	app.MethodNotAllowed(func(writer http.ResponseWriter, request *http.Request) {
		writeError(writer, request, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed")
	})
	router.Mount(basePath, app)

	router.NotFound(func(writer http.ResponseWriter, request *http.Request) {
		writeError(writer, request, http.StatusNotFound, "not_found", "Route not found")
	})
	router.MethodNotAllowed(func(writer http.ResponseWriter, request *http.Request) {
		writeError(writer, request, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed")
	})

	return otelhttp.NewHandler(router, "opskeeper.http")
}

func canServeWebUI(request *http.Request, basePath string) bool {
	if request.Method != http.MethodGet && request.Method != http.MethodHead {
		return false
	}
	relativePath := request.URL.Path
	if basePath != "/" {
		relativePath = strings.TrimPrefix(relativePath, basePath)
	}
	return relativePath != "/api" && !strings.HasPrefix(relativePath, "/api/") && relativePath != "/health" && !strings.HasPrefix(relativePath, "/health/")
}
