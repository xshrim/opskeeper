//go:build embed_webui

package webui

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestEmbeddedAssetsUseRuntimeBasePath(t *testing.T) {
	handler, err := New("/test-ops")
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if handler == nil {
		t.Fatal("New() handler = nil")
	}

	request := httptest.NewRequest(http.MethodGet, "/test-ops/", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("GET /test-ops/ status = %d, want %d", response.Code, http.StatusOK)
	}
	if !strings.Contains(response.Body.String(), `href="/test-ops/" data-opsk-runtime-base`) {
		t.Fatalf("GET /test-ops/ did not contain the runtime base path")
	}
}
