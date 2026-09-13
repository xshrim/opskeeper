package server

import (
	"context"
	"errors"
	"github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"log"
	"net/http"
	rmq "opskeeper/backend/tool/rabbitmq"
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
type ToolInfo = rmq.ToolInfo

func AvailableTools() []ToolInfo { return rmq.ListTools() }
func ConfigFromEnv() Config {
	a := os.Getenv("RABBITMQ_MCP_HTTP_ADDRESS")
	if strings.TrimSpace(a) == "" {
		a = "0.0.0.0:8818"
	}
	return Config{Address: a, BearerToken: os.Getenv("RABBITMQ_MCP_BEARER_TOKEN")}
}
func New(cfg Config) (http.Handler, error) {
	if strings.TrimSpace(cfg.Address) == "" {
		return nil, errors.New("RabbitMQ MCP HTTP address must not be empty")
	}
	s := mcp.NewServer(&mcp.Implementation{Name: "opskeeper-rabbitmq-mcp", Title: "OpsKeeper RabbitMQ MCP", Description: "Read-only RabbitMQ Management API tools.", Version: "dev"}, nil)
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
		h = auth.RequireBearerToken(func(_ context.Context, g string, _ *http.Request) (*auth.TokenInfo, error) {
			if equal(g, token) == false {
				return nil, auth.ErrInvalidToken
			}
			return &auth.TokenInfo{UserID: "rabbitmq-mcp-static-token"}, nil
		}, &auth.RequireBearerTokenOptions{AllowMissingExpiration: true})(h)
	}
	return h, nil
}
func equal(g string, w []byte) bool {
	if len(g) != len(w) {
		return false
	}
	var x byte
	for i := range w {
		x |= g[i] ^ w[i]
	}
	return x == 0
}
