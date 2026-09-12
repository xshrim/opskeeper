package server

import (
	"context"
	"crypto/subtle"
	"errors"
	"github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"log"
	"net/http"
	rt "opskeeper/backend/tool/redis"
	"os"
	"strings"
	"time"
)

const StreamableHTTPPath = "/mcp"
const SSEPath = "/sse"

type Config struct {
	Address, BearerToken string
	CORSEnabled          bool
	Logger               *log.Logger
}
type ToolInfo = rt.ToolInfo

func AvailableTools() []ToolInfo { return rt.ListTools() }
func ConfigFromEnv() Config {
	a := strings.TrimSpace(os.Getenv("REDIS_MCP_HTTP_ADDRESS"))
	if a == "" {
		a = "0.0.0.0:8815"
	}
	return Config{Address: a, BearerToken: os.Getenv("REDIS_MCP_BEARER_TOKEN"), CORSEnabled: envBool("REDIS_MCP_CORS_ENABLED", false)}
}
func New(cfg Config) (http.Handler, error) {
	if strings.TrimSpace(cfg.Address) == "" {
		return nil, errors.New("Redis MCP HTTP address must not be empty")
	}
	s := mcp.NewServer(&mcp.Implementation{Name: "opskeeper-redis-mcp", Title: "OpsKeeper Redis MCP", Description: "Read-only Redis inspection tools.", Version: "dev"}, nil)
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
		token := []byte(cfg.BearerToken)
		verify := func(_ context.Context, t string, _ *http.Request) (*auth.TokenInfo, error) {
			if subtle.ConstantTimeCompare([]byte(t), token) != 1 {
				return nil, auth.ErrInvalidToken
			}
			return &auth.TokenInfo{UserID: "redis-mcp-static-token"}, nil
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
		if r.Header.Get("Origin") != "" {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, Accept, Mcp-Session-Id, Last-Event-ID")
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}
func requestLogger(next http.Handler, logger *log.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		rw := &loggingResponseWriter{ResponseWriter: w}
		next.ServeHTTP(rw, r)
		logger.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(started))
	})
}

type loggingResponseWriter struct {
	http.ResponseWriter
	status int
}

func (w *loggingResponseWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}
