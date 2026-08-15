package httpapi

import (
	"log/slog"
	"net/http"
	"path"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"opskeeper/backend/health"
)

func NewRouter(logger *slog.Logger, healthService *health.Service, version, basePath string, organizationService organizationService, webUI http.Handler) http.Handler {
	router := chi.NewRouter()
	router.Use(middleware.RequestID)
	router.Use(middleware.RealIP)
	router.Use(recoverer(logger))
	router.Use(requestLogger(logger))

	app := chi.NewRouter()
	app.Get("/health/live", func(writer http.ResponseWriter, _ *http.Request) {
		writeJSON(writer, http.StatusOK, health.Liveness(healthService.Name(), version))
	})
	app.Get("/health/ready", func(writer http.ResponseWriter, request *http.Request) {
		report := healthService.Readiness(request.Context(), version)
		status := http.StatusOK
		if report.Status != "ready" {
			status = http.StatusServiceUnavailable
		}
		writeJSON(writer, status, report)
	})
	if organizationService != nil {
		app.Route("/api/v1", func(router chi.Router) {
			registerOrganizationRoutes(router, organizationService, path.Join(basePath, "api/v1"))
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

	return router
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
