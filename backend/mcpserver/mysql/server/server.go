package server

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	mt "opskeeper/backend/tool/mysql"
)

const StreamableHTTPPath = "/mcp"
const SSEPath = "/sse"

type Config struct {
	Address, BearerToken string
	CORSEnabled          bool
	Logger               *log.Logger
}
type ToolInfo = mt.ToolInfo

func AvailableTools() []ToolInfo { return mt.ListTools() }
func ConfigFromEnv() Config {
	address := strings.TrimSpace(os.Getenv("MYSQL_MCP_HTTP_ADDRESS"))
	if address == "" {
		address = "0.0.0.0:8816"
	}
	return Config{Address: address, BearerToken: os.Getenv("MYSQL_MCP_BEARER_TOKEN")}
}
func New(cfg Config) (http.Handler, error) {
	if strings.TrimSpace(cfg.Address) == "" {
		return nil, errors.New("MySQL MCP HTTP address must not be empty")
	}
	s := mcp.NewServer(&mcp.Implementation{Name: "opskeeper-mysql-mcp", Title: "OpsKeeper MySQL MCP", Description: "Read-only MySQL inspection tools.", Version: "dev"}, nil)
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
		h = auth.RequireBearerToken(func(_ context.Context, got string, _ *http.Request) (*auth.TokenInfo, error) {
			if !equal(got, token) {
				return nil, auth.ErrInvalidToken
			}
			return &auth.TokenInfo{UserID: "mysql-mcp-static-token"}, nil
		}, &auth.RequireBearerTokenOptions{AllowMissingExpiration: true})(h)
	}
	if cfg.CORSEnabled {
		h = allowCORS(h)
	}
	return h, nil
}

func allowCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Origin") != "" {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, Accept, Mcp-Session-Id, Last-Event-ID")
			w.Header().Set("Access-Control-Expose-Headers", "Mcp-Session-Id, Last-Event-ID")
		}
		if r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != "" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
func equal(got string, want []byte) bool {
	if len(got) != len(want) {
		return false
	}
	var x byte
	for i := range want {
		x |= got[i] ^ want[i]
	}
	return x == 0
}
