package httpapi

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/opskeeper/opskeeper/backend/internal/health"
)

func TestLiveness(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	router := NewRouter(logger, health.NewService(time.Second, nil), "test", nil)
	request := httptest.NewRequest(http.MethodGet, "/health/live", nil)
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("GET /health/live status = %d, want %d", response.Code, http.StatusOK)
	}
	if !strings.Contains(response.Body.String(), `"status":"alive"`) {
		t.Fatalf("GET /health/live body = %s", response.Body.String())
	}
}

func TestReadinessFailure(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	service := health.NewService(time.Second, []health.Check{
		{Name: "postgres", Run: func(context.Context) error { return context.DeadlineExceeded }},
	})
	router := NewRouter(logger, service, "test", nil)
	request := httptest.NewRequest(http.MethodGet, "/health/ready", nil)
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("GET /health/ready status = %d, want %d", response.Code, http.StatusServiceUnavailable)
	}
}
