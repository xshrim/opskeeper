package httpapi

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/opskeeper/opskeeper/backend/internal/health"
)

func NewRouter(logger *slog.Logger, healthService *health.Service, version string) http.Handler {
	router := chi.NewRouter()
	router.Use(middleware.RequestID)
	router.Use(middleware.RealIP)
	router.Use(recoverer(logger))
	router.Use(requestLogger(logger))

	router.Get("/health/live", func(writer http.ResponseWriter, _ *http.Request) {
		writeJSON(writer, http.StatusOK, health.Liveness(version))
	})
	router.Get("/health/ready", func(writer http.ResponseWriter, request *http.Request) {
		report := healthService.Readiness(request.Context(), version)
		status := http.StatusOK
		if report.Status != "ready" {
			status = http.StatusServiceUnavailable
		}
		writeJSON(writer, status, report)
	})

	router.NotFound(func(writer http.ResponseWriter, request *http.Request) {
		writeError(writer, request, http.StatusNotFound, "not_found", "Route not found")
	})
	router.MethodNotAllowed(func(writer http.ResponseWriter, request *http.Request) {
		writeError(writer, request, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed")
	})

	return router
}
