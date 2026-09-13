package server

import (
	"context"
	"errors"
	"github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"log"
	"net/http"
	ot "opskeeper/backend/tool/oracle"
	"os"
	"strings"
)

const StreamableHTTPPath = "/mcp"
const SSEPath = "/sse"

type Config struct {
	Address, BearerToken string
	CORSEnabled          bool
	Logger               *log.Logger
}
type ToolInfo = ot.ToolInfo

func AvailableTools() []ToolInfo { return ot.ListTools() }
func ConfigFromEnv() Config {
	a := strings.TrimSpace(os.Getenv("ORACLE_MCP_HTTP_ADDRESS"))
	if a == "" {
		a = "0.0.0.0:8819"
	}
	return Config{Address: a, BearerToken: os.Getenv("ORACLE_MCP_BEARER_TOKEN")}
}
func New(cfg Config) (http.Handler, error) {
	if strings.TrimSpace(cfg.Address) == "" {
		return nil, errors.New("Oracle MCP HTTP address must not be empty")
	}
	s := mcp.NewServer(&mcp.Implementation{Name: "opskeeper-oracle-mcp", Title: "OpsKeeper Oracle MCP", Description: "Read-only Oracle inspection tools.", Version: "dev"}, nil)
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
			return &auth.TokenInfo{UserID: "oracle-mcp-static-token"}, nil
		}, &auth.RequireBearerTokenOptions{AllowMissingExpiration: true})(h)
	}
	return h, nil
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
