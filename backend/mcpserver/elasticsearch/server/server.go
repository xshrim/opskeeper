package server

import (
	"context"
	"crypto/subtle"
	"errors"
	"github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"log"
	"net/http"
	es "opskeeper/backend/tool/elasticsearch"
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
type ToolInfo = es.ToolInfo

func AvailableTools() []ToolInfo { return es.ListTools() }
func ConfigFromEnv() Config {
	a := os.Getenv("ELASTICSEARCH_MCP_HTTP_ADDRESS")
	if strings.TrimSpace(a) == "" {
		a = "0.0.0.0:8818"
	}
	return Config{Address: a, BearerToken: os.Getenv("ELASTICSEARCH_MCP_BEARER_TOKEN")}
}
func New(c Config) (http.Handler, error) {
	if strings.TrimSpace(c.Address) == "" {
		return nil, errors.New("Elasticsearch MCP HTTP address must not be empty")
	}
	s := mcp.NewServer(&mcp.Implementation{Name: "opskeeper-elasticsearch-mcp", Title: "OpsKeeper Elasticsearch MCP", Description: "Read-only Elasticsearch tools.", Version: "dev"}, nil)
	RegisterTools(s)
	get := func(*http.Request) *mcp.Server { return s }
	stream, sse := mcp.NewStreamableHTTPHandler(get, nil), mcp.NewSSEHandler(get, nil)
	var h http.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/mcp", "/mcp/":
			stream.ServeHTTP(w, r)
		case "/sse", "/sse/":
			sse.ServeHTTP(w, r)
		default:
			http.NotFound(w, r)
		}
	})
	if c.BearerToken != "" {
		tok := []byte(c.BearerToken)
		h = auth.RequireBearerToken(func(_ context.Context, g string, _ *http.Request) (*auth.TokenInfo, error) {
			if subtle.ConstantTimeCompare([]byte(g), tok) != 1 {
				return nil, auth.ErrInvalidToken
			}
			return &auth.TokenInfo{UserID: "elasticsearch-mcp-static-token"}, nil
		}, &auth.RequireBearerTokenOptions{AllowMissingExpiration: true})(h)
	}
	return h, nil
}

var _ = time.Second
