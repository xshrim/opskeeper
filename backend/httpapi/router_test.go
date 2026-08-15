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

	"opskeeper/backend/health"
	"opskeeper/backend/version"
)

var testBuild = version.Info{Version: "test", Commit: "abc123", BuildTime: "2026-01-01T00:00:00Z"}

func TestLiveness(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	router := NewRouter(logger, health.NewService("test-api", time.Second, nil), testBuild, Options{BasePath: "/test"}, nil, nil)
	request := httptest.NewRequest(http.MethodGet, "/test/health/live", nil)
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("GET /test/health/live status = %d, want %d", response.Code, http.StatusOK)
	}
	if !strings.Contains(response.Body.String(), `"status":"alive"`) {
		t.Fatalf("GET /test/health/live body = %s", response.Body.String())
	}
	if !strings.Contains(response.Body.String(), `"service":"test-api"`) {
		t.Fatalf("GET /test/health/live body = %s", response.Body.String())
	}
	if !strings.Contains(response.Body.String(), `"commit":"abc123"`) {
		t.Fatalf("GET /test/health/live body = %s", response.Body.String())
	}
}

func TestReadinessFailure(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	service := health.NewService("test-api", time.Second, []health.Check{
		{Name: "postgres", Run: func(context.Context) error { return context.DeadlineExceeded }},
	})
	router := NewRouter(logger, service, testBuild, Options{BasePath: "/test"}, nil, nil)
	request := httptest.NewRequest(http.MethodGet, "/test/health/ready", nil)
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("GET /test/health/ready status = %d, want %d", response.Code, http.StatusServiceUnavailable)
	}
}

func TestRouterRejectsUnprefixedRoute(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	router := NewRouter(logger, health.NewService("test-api", time.Second, nil), testBuild, Options{BasePath: "/test"}, nil, nil)
	request := httptest.NewRequest(http.MethodGet, "/health/live", nil)
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusNotFound {
		t.Fatalf("GET /health/live status = %d, want %d", response.Code, http.StatusNotFound)
	}
}

func TestRouterKeepsUnknownAPIAsJSONWhenWebUIIsEnabled(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	webUI := http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "text/html")
		_, _ = writer.Write([]byte("web UI"))
	})
	router := NewRouter(logger, health.NewService("test-api", time.Second, nil), testBuild, Options{BasePath: "/test"}, nil, webUI)

	apiRequest := httptest.NewRequest(http.MethodGet, "/test/api/v1/missing", nil)
	apiResponse := httptest.NewRecorder()
	router.ServeHTTP(apiResponse, apiRequest)
	if apiResponse.Code != http.StatusNotFound || !strings.Contains(apiResponse.Header().Get("Content-Type"), "application/json") {
		t.Fatalf("unknown API response = %d %q", apiResponse.Code, apiResponse.Header().Get("Content-Type"))
	}

	pageRequest := httptest.NewRequest(http.MethodGet, "/test/teams/example", nil)
	pageResponse := httptest.NewRecorder()
	router.ServeHTTP(pageResponse, pageRequest)
	if pageResponse.Code != http.StatusOK || pageResponse.Body.String() != "web UI" {
		t.Fatalf("SPA response = %d %q", pageResponse.Code, pageResponse.Body.String())
	}
}

func TestRouterSupportsRootBasePath(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	webUI := http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "text/html")
		_, _ = writer.Write([]byte("web UI"))
	})
	router := NewRouter(logger, health.NewService("opskeeper-api", time.Second, nil), testBuild, Options{BasePath: "/"}, nil, webUI)

	for _, test := range []struct {
		path        string
		status      int
		contentType string
	}{
		{path: "/health/live", status: http.StatusOK, contentType: "application/json"},
		{path: "/api/v1/missing", status: http.StatusNotFound, contentType: "application/json"},
		{path: "/teams/example", status: http.StatusOK, contentType: "text/html"},
	} {
		t.Run(test.path, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, test.path, nil)
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			if response.Code != test.status || !strings.Contains(response.Header().Get("Content-Type"), test.contentType) {
				t.Fatalf("GET %s response = %d %q", test.path, response.Code, response.Header().Get("Content-Type"))
			}
		})
	}
}
