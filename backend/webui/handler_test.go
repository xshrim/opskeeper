package webui

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

func TestHandlerServesIndexAndStaticAssets(t *testing.T) {
	handler := newTestHandler(t)

	tests := []struct {
		path         string
		status       int
		bodyContains string
		cacheControl string
	}{
		{path: "/opskeeper/", status: http.StatusOK, bodyContains: `href="/opskeeper/"`, cacheControl: "no-cache"},
		{path: "/opskeeper/teams/example", status: http.StatusOK, bodyContains: `href="/opskeeper/"`, cacheControl: "no-cache"},
		{path: "/opskeeper/assets/app.js", status: http.StatusOK, bodyContains: "console.log", cacheControl: "public, max-age=31536000, immutable"},
		{path: "/opskeeper/assets/missing.js", status: http.StatusNotFound},
	}

	for _, test := range tests {
		t.Run(test.path, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, test.path, nil)
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)

			if response.Code != test.status {
				t.Fatalf("GET %s status = %d, want %d", test.path, response.Code, test.status)
			}
			if test.bodyContains != "" && !strings.Contains(response.Body.String(), test.bodyContains) {
				t.Fatalf("GET %s body = %q, want %q", test.path, response.Body.String(), test.bodyContains)
			}
			if test.cacheControl != "" && response.Header().Get("Cache-Control") != test.cacheControl {
				t.Fatalf("GET %s Cache-Control = %q, want %q", test.path, response.Header().Get("Cache-Control"), test.cacheControl)
			}
		})
	}
}

func TestHandlerRejectsUnsupportedMethod(t *testing.T) {
	handler := newTestHandler(t)
	request := httptest.NewRequest(http.MethodPost, "/opskeeper/", nil)
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST /opskeeper/ status = %d, want %d", response.Code, http.StatusMethodNotAllowed)
	}
}

func newTestHandler(t *testing.T) http.Handler {
	t.Helper()
	assets := fstest.MapFS{
		"index.html":    {Data: []byte(`<html><head><base href="./" data-opsk-runtime-base /></head></html>`)},
		"assets/app.js": {Data: []byte(`console.log("ready")`)},
	}
	handler, err := newHandler(assets, "/opskeeper")
	if err != nil {
		t.Fatalf("newHandler() error = %v", err)
	}
	return handler
}
