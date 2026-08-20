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
	// LogTimeLayout is the timestamp layout used by every Docker MCP log line.
	// It intentionally has millisecond precision and no timezone suffix so the
	// output remains compatible with the project's backend log convention.
	LogTimeLayout = "2006-01-02T15:04:05.000"
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
	{Name: "docker_info", Description: "Read Docker Engine information."},
	{Name: "docker_images", Description: "List Docker images."},
	{Name: "docker_containers", Description: "List Docker containers."},
	{Name: "docker_container_logs", Description: "Read bounded, non-following logs from a Docker container."},
	{Name: "docker_container_inspect", Description: "Inspect a Docker container with sensitive values removed."},
	{Name: "docker_container_stats", Description: "Read one snapshot of Docker container statistics."},
}

func AvailableTools() []ToolInfo {
	return append([]ToolInfo(nil), toolCatalog...)
}

func ConfigFromEnv() Config {
	return Config{
		Address:     envOrDefault("DOCKER_MCP_HTTP_ADDRESS", "0.0.0.0:8811"),
		BearerToken: os.Getenv("DOCKER_MCP_BEARER_TOKEN"),
		CORSEnabled: envBool("DOCKER_MCP_CORS_ENABLED", false),
	}
}

func New(cfg Config) (http.Handler, error) {
	if strings.TrimSpace(cfg.Address) == "" {
		return nil, errors.New("Docker MCP HTTP address must not be empty")
	}
	mcpServer := mcp.NewServer(&mcp.Implementation{
		Name:        "opskeeper-docker-mcp",
		Title:       "OpsKeeper Docker MCP",
		Description: "Read-only Docker Engine inspection tools.",
		Version:     "dev",
	}, nil)
	RegisterTools(mcpServer)

	getServer := func(*http.Request) *mcp.Server { return mcpServer }
	streamableHandler := mcp.NewStreamableHTTPHandler(getServer, nil)
	sseHandler := mcp.NewSSEHandler(getServer, nil)
	var handler http.Handler = http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case StreamableHTTPPath, StreamableHTTPPath + "/":
			streamableHandler.ServeHTTP(writer, request)
		case SSEPath, SSEPath + "/":
			sseHandler.ServeHTTP(writer, request)
		default:
			http.NotFound(writer, request)
		}
	})
	if strings.TrimSpace(cfg.BearerToken) != "" {
		configured := []byte(cfg.BearerToken)
		verifier := func(_ context.Context, token string, _ *http.Request) (*auth.TokenInfo, error) {
			if subtle.ConstantTimeCompare([]byte(token), configured) != 1 {
				return nil, auth.ErrInvalidToken
			}
			return &auth.TokenInfo{UserID: "docker-mcp-static-token"}, nil
		}
		handler = auth.RequireBearerToken(verifier, &auth.RequireBearerTokenOptions{AllowMissingExpiration: true})(handler)
	}
	if cfg.CORSEnabled {
		handler = allowCORS(handler)
	}
	if cfg.Logger != nil {
		handler = requestLogger(handler, cfg.Logger)
	}
	return handler, nil
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

func requestLogger(next http.Handler, logger *log.Logger) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		started := time.Now()
		method := requestMethod(request)
		sessionID := request.Header.Get("Mcp-Session-Id")
		if sessionID == "" {
			sessionID = "-"
		}
		logLine(logger, "[REQUEST] Session: %s | Method: %s", sessionID, method)

		response := &loggingResponseWriter{ResponseWriter: writer}
		next.ServeHTTP(response, request)
		if responseSession := writer.Header().Get("Mcp-Session-Id"); responseSession != "" {
			sessionID = responseSession
		}
		status := response.status
		if status == 0 {
			status = http.StatusOK
		}
		statusText := http.StatusText(status)
		if statusText == "" {
			statusText = fmt.Sprintf("HTTP %d", status)
		}
		logLine(logger, "[RESPONSE] Session: %s | Method: %s | Status: %s | Duration: %s", sessionID, method, statusText, time.Since(started))
	})
}

func requestMethod(request *http.Request) string {
	if request.Body == nil {
		return request.Method
	}
	body, err := io.ReadAll(io.LimitReader(request.Body, 1<<20))
	if err != nil {
		return request.Method
	}
	request.Body.Close()
	request.Body = io.NopCloser(bytes.NewReader(body))
	var message struct {
		Method string `json:"method"`
	}
	if json.Unmarshal(body, &message) == nil && message.Method != "" {
		return message.Method
	}
	return request.Method
}

type loggingResponseWriter struct {
	http.ResponseWriter
	status int
}

func (writer *loggingResponseWriter) WriteHeader(status int) {
	if writer.status != 0 {
		return
	}
	writer.status = status
	writer.ResponseWriter.WriteHeader(status)
}

func (writer *loggingResponseWriter) Write(body []byte) (int, error) {
	if writer.status == 0 {
		writer.WriteHeader(http.StatusOK)
	}
	return writer.ResponseWriter.Write(body)
}

func (writer *loggingResponseWriter) Flush() {
	if writer.status == 0 {
		writer.WriteHeader(http.StatusOK)
	}
	if flusher, ok := writer.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
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
	default:
		return fallback
	}
}

// allowCORS enables browser clients to call the MCP endpoint from any origin.
// It deliberately does not enable credentialed requests; authentication, when
// configured, is sent through the Authorization header instead of cookies.
func allowCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Origin") == "" {
			next.ServeHTTP(writer, request)
			return
		}

		header := writer.Header()
		header.Set("Access-Control-Allow-Origin", "*")
		header.Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		header.Set("Access-Control-Allow-Headers", "Authorization, Content-Type, Accept, Mcp-Session-Id, Last-Event-ID")
		header.Set("Access-Control-Expose-Headers", "Mcp-Session-Id, Last-Event-ID")
		header.Set("Access-Control-Max-Age", "600")
		header.Add("Vary", "Origin")

		if request.Method == http.MethodOptions && request.Header.Get("Access-Control-Request-Method") != "" {
			writer.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(writer, request)
	})
}
