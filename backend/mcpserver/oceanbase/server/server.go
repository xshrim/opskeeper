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
	ot "opskeeper/backend/tool/oceanbase"
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
	a := strings.TrimSpace(os.Getenv("OCEANBASE_MCP_HTTP_ADDRESS"))
	if a == "" {
		a = "0.0.0.0:8822"
	}
	return Config{Address: a, BearerToken: os.Getenv("OCEANBASE_MCP_BEARER_TOKEN")}
}
func New(cfg Config) (http.Handler, error) {
	if strings.TrimSpace(cfg.Address) == "" {
		return nil, errors.New("OceanBase MCP HTTP address must not be empty")
	}
	s := mcp.NewServer(&mcp.Implementation{Name: "opskeeper-oceanbase-mcp", Title: "OpsKeeper OceanBase MCP", Description: "Read-only OceanBase inspection tools.", Version: "dev"}, nil)
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
			return &auth.TokenInfo{UserID: "oceanbase-mcp-static-token"}, nil
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
