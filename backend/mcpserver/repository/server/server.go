package server

import (
	"context"
	"errors"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"log"
	"net/http"
	rt "opskeeper/backend/tool/repository"
	"os"
	"strings"
)

const StreamableHTTPPath = "/mcp"
const SSEPath = "/sse"

type Config struct {
	Address, BearerToken string
	Logger               *log.Logger
}

func ConfigFromEnv() Config {
	a := os.Getenv("REPOSITORY_MCP_HTTP_ADDRESS")
	if strings.TrimSpace(a) == "" {
		a = "0.0.0.0:8820"
	}
	return Config{Address: a, BearerToken: os.Getenv("REPOSITORY_MCP_BEARER_TOKEN")}
}

type input struct{ rt.ConnectionInput }
type pathInput struct {
	rt.ConnectionInput
	Path     string `json:"path,omitempty"`
	MaxBytes int64  `json:"max_bytes,omitempty"`
	Limit    int    `json:"limit,omitempty"`
}
type searchInput struct {
	rt.ConnectionInput
	Query string `json:"query"`
	Limit int    `json:"limit,omitempty"`
}
type checkoutInput struct {
	rt.ConnectionInput
	Branch string `json:"branch"`
}

func RegisterTools(s *mcp.Server) {
	schema := rt.InputSchema(nil)
	mcp.AddTool(s, &mcp.Tool{Name: "repository_branches", Description: "List repository branches.", InputSchema: schema}, func(c context.Context, _ *mcp.CallToolRequest, in input) (*mcp.CallToolResult, []string, error) {
		o, e := rt.Branches(c, in.ConnectionInput)
		return nil, o, e
	})
	mcp.AddTool(s, &mcp.Tool{Name: "repository_status", Description: "Read repository status and HEAD.", InputSchema: schema}, func(c context.Context, _ *mcp.CallToolRequest, in input) (*mcp.CallToolResult, map[string]any, error) {
		o, e := rt.Status(c, in.ConnectionInput)
		return nil, o, e
	})
	mcp.AddTool(s, &mcp.Tool{Name: "repository_metadata", Description: "Read repository metadata.", InputSchema: schema}, func(c context.Context, _ *mcp.CallToolRequest, in input) (*mcp.CallToolResult, map[string]any, error) {
		o, e := rt.Metadata(c, in.ConnectionInput)
		return nil, o, e
	})
	mcp.AddTool(s, &mcp.Tool{Name: "repository_checkout", Description: "Switch the repository read branch.", InputSchema: rt.InputSchema(map[string]any{"branch": map[string]any{"type": "string"}})}, func(c context.Context, _ *mcp.CallToolRequest, in checkoutInput) (*mcp.CallToolResult, map[string]any, error) {
		o, e := rt.Checkout(c, in.ConnectionInput, in.Branch)
		return nil, o, e
	})
	mcp.AddTool(s, &mcp.Tool{Name: "repository_tree", Description: "Read a bounded repository directory tree.", InputSchema: rt.InputSchema(map[string]any{"path": map[string]any{"type": "string"}, "limit": map[string]any{"type": "integer"}})}, func(c context.Context, _ *mcp.CallToolRequest, in pathInput) (*mcp.CallToolResult, []rt.TreeEntry, error) {
		o, e := rt.Tree(c, in.ConnectionInput, in.Path, in.Limit)
		return nil, o, e
	})
	mcp.AddTool(s, &mcp.Tool{Name: "repository_file", Description: "Read a bounded repository file.", InputSchema: rt.InputSchema(map[string]any{"path": map[string]any{"type": "string"}, "max_bytes": map[string]any{"type": "integer"}})}, func(c context.Context, _ *mcp.CallToolRequest, in pathInput) (*mcp.CallToolResult, string, error) {
		o, e := rt.File(c, in.ConnectionInput, in.Path, in.MaxBytes)
		return nil, o, e
	})
	mcp.AddTool(s, &mcp.Tool{Name: "repository_search", Description: "Search text in repository files.", InputSchema: rt.InputSchema(map[string]any{"query": map[string]any{"type": "string"}, "limit": map[string]any{"type": "integer"}})}, func(c context.Context, _ *mcp.CallToolRequest, in searchInput) (*mcp.CallToolResult, []map[string]any, error) {
		o, e := rt.Search(c, in.ConnectionInput, in.Query, in.Limit)
		return nil, o, e
	})
}
func New(cfg Config) (http.Handler, error) {
	if strings.TrimSpace(cfg.Address) == "" {
		return nil, errors.New("Repository MCP HTTP address must not be empty")
	}
	s := mcp.NewServer(&mcp.Implementation{Name: "opskeeper-repository-mcp", Title: "OpsKeeper Repository MCP", Description: "Read-only repository inspection tools.", Version: "dev"}, nil)
	RegisterTools(s)
	get := func(*http.Request) *mcp.Server { return s }
	stream, sse := mcp.NewStreamableHTTPHandler(get, nil), mcp.NewSSEHandler(get, nil)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case StreamableHTTPPath, StreamableHTTPPath + "/":
			stream.ServeHTTP(w, r)
		case SSEPath, SSEPath + "/":
			sse.ServeHTTP(w, r)
		default:
			http.NotFound(w, r)
		}
	}), nil
}
