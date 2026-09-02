package server

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestConfigFromEnvBearerTokenIsOptional(t *testing.T) {
	t.Setenv("DOCKER_MCP_HTTP_ADDRESS", "")
	t.Setenv("DOCKER_MCP_BEARER_TOKEN", "")
	t.Setenv("DOCKER_MCP_CORS_ENABLED", "")
	cfg := ConfigFromEnv()
	if cfg.Address != "0.0.0.0:8811" {
		t.Fatalf("address = %q, want default 0.0.0.0:8811", cfg.Address)
	}
	if cfg.BearerToken != "" {
		t.Fatal("expected bearer authentication to be disabled when token is empty")
	}
	if cfg.CORSEnabled {
		t.Fatal("expected CORS to be disabled by default")
	}
	if _, err := New(cfg); err != nil {
		t.Fatalf("empty bearer token should be accepted: %v", err)
	}

	t.Setenv("DOCKER_MCP_BEARER_TOKEN", "secret")
	cfg = ConfigFromEnv()
	if cfg.BearerToken != "secret" {
		t.Fatalf("bearer token = %q, want configured token", cfg.BearerToken)
	}
	if _, err := New(cfg); err != nil {
		t.Fatalf("configured bearer token should enable authentication: %v", err)
	}

	t.Setenv("DOCKER_MCP_CORS_ENABLED", "true")
	cfg = ConfigFromEnv()
	if !cfg.CORSEnabled {
		t.Fatal("expected CORS to be enabled by configuration")
	}
}

func TestNewSupportsBothEndpoints(t *testing.T) {
	handler, err := New(Config{Address: "127.0.0.1:8811"})
	if err != nil {
		t.Fatal(err)
	}
	if handler == nil {
		t.Fatal("expected handler")
	}
	request := httptest.NewRequest(http.MethodGet, "http://localhost/unsupported", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusNotFound {
		t.Fatalf("unsupported endpoint status = %d, want %d", response.Code, http.StatusNotFound)
	}
}

func TestStreamableHTTPToolsOmitComplexOutputSchemas(t *testing.T) {
	handler, err := New(Config{Address: "127.0.0.1:8811"})
	if err != nil {
		t.Fatal(err)
	}
	httpServer := newIPv4HTTPServer(t, handler)

	client := mcp.NewClient(&mcp.Implementation{Name: "schema-test-client", Version: "test"}, nil)
	session, err := client.Connect(context.Background(), &mcp.StreamableClientTransport{Endpoint: httpServer.URL + "/mcp"}, nil)
	if err != nil {
		t.Fatalf("MCP client connect failed: %v", err)
	}
	defer session.Close()
	result, err := session.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("MCP tools/list failed: %v", err)
	}
	if len(result.Tools) != 6 {
		t.Fatalf("tool count = %d, want 6", len(result.Tools))
	}
	for _, tool := range result.Tools {
		encoded, err := json.Marshal(tool.InputSchema)
		if err != nil {
			t.Fatalf("marshal tool %q input schema: %v", tool.Name, err)
		}
		if strings.Contains(string(encoded), `"$ref"`) || strings.Contains(string(encoded), `"allOf"`) {
			t.Errorf("tool %q input schema contains a composition: %s", tool.Name, encoded)
		}
		if tool.OutputSchema != nil {
			t.Errorf("tool %q unexpectedly advertises output schema: %v", tool.Name, tool.OutputSchema)
		}
	}
}

func TestDockerToolSchemasDescribeEveryParameter(t *testing.T) {
	handler, err := New(Config{Address: "127.0.0.1:8811"})
	if err != nil {
		t.Fatal(err)
	}
	httpServer := newIPv4HTTPServer(t, handler)
	client := mcp.NewClient(&mcp.Implementation{Name: "schema-description-test", Version: "test"}, nil)
	session, err := client.Connect(context.Background(), &mcp.StreamableClientTransport{Endpoint: httpServer.URL + "/mcp"}, nil)
	if err != nil {
		t.Fatalf("MCP client connect failed: %v", err)
	}
	defer session.Close()
	result, err := session.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, tool := range result.Tools {
		var schema struct {
			Properties map[string]struct {
				Description string `json:"description"`
			} `json:"properties"`
		}
		encoded, _ := json.Marshal(tool.InputSchema)
		if err := json.Unmarshal(encoded, &schema); err != nil {
			t.Fatalf("tool %q schema: %v", tool.Name, err)
		}
		for name, property := range schema.Properties {
			if strings.TrimSpace(property.Description) == "" {
				t.Errorf("tool %q parameter %q has no description", tool.Name, name)
			}
		}
	}
}

func TestSSEToolsAreAvailableAtSSEEndpoint(t *testing.T) {
	handler, err := New(Config{Address: "127.0.0.1:8811"})
	if err != nil {
		t.Fatal(err)
	}
	httpServer := newIPv4HTTPServer(t, handler)

	client := mcp.NewClient(&mcp.Implementation{Name: "sse-test-client", Version: "test"}, nil)
	session, err := client.Connect(context.Background(), &mcp.SSEClientTransport{Endpoint: httpServer.URL + SSEPath}, nil)
	if err != nil {
		t.Fatalf("MCP SSE client connect failed: %v", err)
	}
	defer session.Close()
	result, err := session.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("MCP SSE tools/list failed: %v", err)
	}
	if len(result.Tools) != 6 {
		t.Fatalf("SSE tool count = %d, want 6", len(result.Tools))
	}
}

func TestRequestLoggingUsesMillisecondTimestamp(t *testing.T) {
	var logs bytes.Buffer
	logger := log.New(&logs, "", 0)
	handler, err := New(Config{Address: "127.0.0.1:8811", Logger: logger})
	if err != nil {
		t.Fatal(err)
	}
	httpServer := newIPv4HTTPServer(t, handler)

	client := mcp.NewClient(&mcp.Implementation{Name: "logging-test-client", Version: "test"}, nil)
	session, err := client.Connect(context.Background(), &mcp.StreamableClientTransport{Endpoint: httpServer.URL + "/mcp"}, nil)
	if err != nil {
		t.Fatalf("MCP client connect failed: %v", err)
	}
	if _, err := session.ListTools(context.Background(), nil); err != nil {
		t.Fatalf("MCP tools/list failed: %v", err)
	}
	_ = session.Close()

	output := logs.String()
	for _, method := range []string{"initialize", "notifications/initialized", "tools/list"} {
		if !strings.Contains(output, "Method: "+method) {
			t.Errorf("logs do not contain MCP method %q: %s", method, output)
		}
	}
	if !strings.Contains(output, "[REQUEST]") || !strings.Contains(output, "[RESPONSE]") {
		t.Fatalf("logs do not contain request and response entries: %s", output)
	}
	timestamp := regexp.MustCompile(`(?m)^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d{3} `)
	if !timestamp.MatchString(output) {
		t.Fatalf("log timestamps do not use millisecond ISO format: %s", output)
	}
	if strings.Contains(output, "logging-test-client") {
		t.Fatal("logs unexpectedly contain client payload data")
	}
}

func TestBearerHandlerRejectsInvalidToken(t *testing.T) {
	handler, err := New(Config{Address: "127.0.0.1:8811", BearerToken: "secret"})
	if err != nil {
		t.Fatal(err)
	}
	req := httptestNewRequest("GET", "/mcp", "wrong")
	response := httptestRecorder()
	handler.ServeHTTP(response, req)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusUnauthorized)
	}
}

func TestCORSIsDisabledByDefault(t *testing.T) {
	handler, err := New(Config{Address: "127.0.0.1:8811"})
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodOptions, "http://localhost/mcp", nil)
	request.Header.Set("Origin", "http://localhost:3000")
	request.Header.Set("Access-Control-Request-Method", http.MethodPost)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatal("CORS headers must be absent when CORS is disabled")
	}
}

func TestCORSAllowsMCPBrowserPreflight(t *testing.T) {
	handler, err := New(Config{Address: "127.0.0.1:8811", CORSEnabled: true})
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodOptions, "http://localhost/mcp", nil)
	request.Header.Set("Origin", "http://localhost:3000")
	request.Header.Set("Access-Control-Request-Method", http.MethodPost)
	request.Header.Set("Access-Control-Request-Headers", "authorization, content-type, mcp-session-id")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusNoContent)
	}
	if got := response.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Fatalf("allow origin = %q, want *", got)
	}
	if got := response.Header().Get("Access-Control-Allow-Headers"); got == "" {
		t.Fatal("expected MCP request headers to be allowed")
	}
	if got := response.Header().Get("Access-Control-Expose-Headers"); got == "" {
		t.Fatal("expected MCP response headers to be exposed")
	}
}

// Small wrappers keep this test independent of any application HTTP helpers.
func httptestNewRequest(method, target, token string) *http.Request {
	req, _ := http.NewRequest(method, target, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	return req
}

type recorder struct {
	header http.Header
	Code   int
}

func newIPv4HTTPServer(t *testing.T, handler http.Handler) *httptest.Server {
	t.Helper()
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen on IPv4 loopback: %v", err)
	}
	server := httptest.NewUnstartedServer(handler)
	server.Listener = listener
	server.Start()
	t.Cleanup(server.Close)
	return server
}

func httptestRecorder() *recorder        { return &recorder{header: make(http.Header)} }
func (r *recorder) Header() http.Header  { return r.header }
func (r *recorder) WriteHeader(code int) { r.Code = code }
func (r *recorder) Write(body []byte) (int, error) {
	if r.Code == 0 {
		r.Code = http.StatusOK
	}
	return len(body), nil
}
