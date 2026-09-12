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
	pt "opskeeper/backend/tool/postgresql"
)

const (
	StreamableHTTPPath = "/mcp"
	SSEPath            = "/sse"
	LogTimeLayout      = "2006-01-02T15:04:05.000"
)

type Config struct {
	Address, BearerToken string
	CORSEnabled          bool
	Logger               *log.Logger
}
type ToolInfo = pt.ToolInfo

func AvailableTools() []ToolInfo { return pt.ListTools() }
func ConfigFromEnv() Config {
	return Config{Address: envOrDefault("POSTGRESQL_MCP_HTTP_ADDRESS", "0.0.0.0:8814"), BearerToken: os.Getenv("POSTGRESQL_MCP_BEARER_TOKEN"), CORSEnabled: envBool("POSTGRESQL_MCP_CORS_ENABLED", false)}
}
func New(cfg Config) (http.Handler, error) {
	if strings.TrimSpace(cfg.Address) == "" {
		return nil, errors.New("PostgreSQL MCP HTTP address must not be empty")
	}
	s := mcp.NewServer(&mcp.Implementation{Name: "opskeeper-postgresql-mcp", Title: "OpsKeeper PostgreSQL MCP", Description: "Read-only PostgreSQL inspection tools.", Version: "dev"}, nil)
	RegisterTools(s)
	get := func(*http.Request) *mcp.Server { return s }
	stream, sse := mcp.NewStreamableHTTPHandler(get, nil), mcp.NewSSEHandler(get, nil)
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
			return &auth.TokenInfo{UserID: "postgresql-mcp-static-token"}, nil
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
	for _, tool := range AvailableTools() {
		logLine(logger, "Available tool: %s (%s)", tool.Name, tool.Description)
	}
}
func logLine(logger *log.Logger, format string, args ...any) {
	logger.Printf("%s %s", time.Now().Format(LogTimeLayout), fmt.Sprintf(format, args...))
}
func envOrDefault(name, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(name)); v != "" {
		return v
	}
	return fallback
}
func envBool(name string, fallback bool) bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(name))) {
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
		method := r.Method
		if r.Body != nil {
			body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
			if err == nil {
				_ = r.Body.Close()
				r.Body = io.NopCloser(bytes.NewReader(body))
				var msg struct {
					Method string `json:"method"`
				}
				if json.Unmarshal(body, &msg) == nil && msg.Method != "" {
					method = msg.Method
				}
			}
		}
		logLine(logger, "[REQUEST] Method: %s", method)
		rw := &loggingResponseWriter{ResponseWriter: w}
		next.ServeHTTP(rw, r)
		status := rw.status
		if status == 0 {
			status = http.StatusOK
		}
		logLine(logger, "[RESPONSE] Method: %s | Status: %s | Duration: %s", method, http.StatusText(status), time.Since(started))
	})
}

type loggingResponseWriter struct {
	http.ResponseWriter
	status int
}

func (w *loggingResponseWriter) WriteHeader(status int) {
	if w.status != 0 {
		return
	}
	w.status = status
	w.ResponseWriter.WriteHeader(status)
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
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}
