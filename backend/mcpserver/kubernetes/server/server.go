package server

import (
	"bytes"
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	StreamableHTTPPath = "/mcp"
	SSEPath            = "/sse"
	LogTimeLayout      = "2006-01-02T15:04:05.000"
)

type Config struct {
	Address     string
	BearerToken string
	CORSEnabled bool
	Logger      *log.Logger
}
type ToolInfo struct {
	Name        string
	Description string
}

var toolCatalog = []ToolInfo{
	{Name: "kubernetes_cluster_info", Description: "Read Kubernetes version and connection information."}, {Name: "kubernetes_api_resources", Description: "List API resources supported by the connected cluster."}, {Name: "kubernetes_namespaces", Description: "List Kubernetes namespaces."}, {Name: "kubernetes_nodes", Description: "List Kubernetes nodes."}, {Name: "kubernetes_pods", Description: "List Kubernetes pods."}, {Name: "kubernetes_workloads", Description: "List Kubernetes workloads."}, {Name: "kubernetes_services", Description: "List Kubernetes services."}, {Name: "kubernetes_configmaps", Description: "List Kubernetes ConfigMaps."}, {Name: "kubernetes_ingresses", Description: "List Kubernetes ingresses."}, {Name: "kubernetes_events", Description: "List Kubernetes events."}, {Name: "kubernetes_pod_logs", Description: "Read bounded pod logs."}, {Name: "kubernetes_resource_get", Description: "Get an allowlisted Kubernetes resource."}, {Name: "kubernetes_health", Description: "Check Kubernetes API health."},
}

func AvailableTools() []ToolInfo { return append([]ToolInfo(nil), toolCatalog...) }
func ConfigFromEnv() Config {
	return Config{Address: envOrDefault("KUBERNETES_MCP_HTTP_ADDRESS", "0.0.0.0:8812"), BearerToken: os.Getenv("KUBERNETES_MCP_BEARER_TOKEN"), CORSEnabled: envBool("KUBERNETES_MCP_CORS_ENABLED", false)}
}
func New(cfg Config) (http.Handler, error) {
	if strings.TrimSpace(cfg.Address) == "" {
		return nil, errors.New("Kubernetes MCP HTTP address must not be empty")
	}
	s := mcp.NewServer(&mcp.Implementation{Name: "opskeeper-kubernetes-mcp", Title: "OpsKeeper Kubernetes MCP", Description: "Read-only Kubernetes inspection tools.", Version: "dev"}, nil)
	RegisterTools(s)
	getServer := func(*http.Request) *mcp.Server { return s }
	stream := mcp.NewStreamableHTTPHandler(getServer, nil)
	sse := mcp.NewSSEHandler(getServer, nil)
	var h http.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case StreamableHTTPPath, StreamableHTTPPath + "/":
			stream.ServeHTTP(w, r)
		case SSEPath, SSEPath + "/":
			sse.ServeHTTP(w, r)
		default:
			http.NotFound(w, r)
		}
	})
	if strings.TrimSpace(cfg.BearerToken) != "" {
		configured := []byte(cfg.BearerToken)
		verify := func(_ context.Context, token string, _ *http.Request) (*auth.TokenInfo, error) {
			if subtle.ConstantTimeCompare([]byte(token), configured) != 1 {
				return nil, auth.ErrInvalidToken
			}
			return &auth.TokenInfo{UserID: "kubernetes-mcp-static-token"}, nil
		}
		h = auth.RequireBearerToken(verify, &auth.RequireBearerTokenOptions{AllowMissingExpiration: true})(h)
	}
	if cfg.CORSEnabled {
		h = allowCORS(h)
	}
	if cfg.Logger != nil {
		h = requestLogger(h, cfg.Logger)
	}
	return h, nil
}
func LogStartup(logger *log.Logger, cfg Config) {
	if logger == nil {
		return
	}
	logLine(logger, "MCP server listening on %s", cfg.Address)
	logLine(logger, "Endpoint: streamable_http=%s | sse=%s", StreamableHTTPPath, SSEPath)
	logLine(logger, "Bearer auth: %t | CORS: %t", strings.TrimSpace(cfg.BearerToken) != "", cfg.CORSEnabled)
	for _, tool := range AvailableTools() {
		logLine(logger, "Available tool: %s (%s)", tool.Name, tool.Description)
	}
}
func logLine(logger *log.Logger, format string, args ...any) {
	logger.Printf("%s %s", time.Now().Format(LogTimeLayout), fmt.Sprintf(format, args...))
}
func envOrDefault(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}
func envBool(name string, fallback bool) bool {
	value := strings.ToLower(strings.TrimSpace(os.Getenv(name)))
	switch value {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	}
	return fallback
}
func allowCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Origin") == "" {
			next.ServeHTTP(w, r)
			return
		}
		h := w.Header()
		h.Set("Access-Control-Allow-Origin", "*")
		h.Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		h.Set("Access-Control-Allow-Headers", "Authorization, Content-Type, Accept, Mcp-Session-Id, Last-Event-ID")
		h.Set("Access-Control-Expose-Headers", "Mcp-Session-Id, Last-Event-ID")
		h.Set("Access-Control-Max-Age", "600")
		h.Add("Vary", "Origin")
		if r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != "" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
func requestLogger(next http.Handler, logger *log.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		session := r.Header.Get("Mcp-Session-Id")
		if session == "" {
			session = "-"
		}
		method := r.Method
		if r.Body != nil {
			body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
			if err == nil {
				r.Body.Close()
				r.Body = io.NopCloser(bytes.NewReader(body))
				var message struct {
					Method string `json:"method"`
				}
				if json.Unmarshal(body, &message) == nil && message.Method != "" {
					method = message.Method
				}
			}
		}
		logLine(logger, "[REQUEST] Session: %s | Method: %s", session, method)
		response := &loggingResponseWriter{ResponseWriter: w}
		next.ServeHTTP(response, r)
		status := response.status
		if status == 0 {
			status = http.StatusOK
		}
		logLine(logger, "[RESPONSE] Session: %s | Method: %s | Status: %s | Duration: %s", session, method, http.StatusText(status), time.Since(started))
	})
}

type loggingResponseWriter struct {
	http.ResponseWriter
	status int
}

func (w *loggingResponseWriter) WriteHeader(status int) {
	if w.status == 0 {
		w.status = status
		w.ResponseWriter.WriteHeader(status)
	}
}
func (w *loggingResponseWriter) Write(body []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(body)
}
func (w *loggingResponseWriter) Flush() {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}
