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
	Offset   int64  `json:"offset,omitempty"`
	MaxBytes int64  `json:"max_bytes,omitempty"`
	Limit    int    `json:"limit,omitempty"`
	Revision string `json:"revision,omitempty"`
}
type searchInput struct {
	rt.ConnectionInput
	Query    string `json:"query"`
	Path     string `json:"path,omitempty"`
	Offset   int    `json:"offset,omitempty"`
	Limit    int    `json:"limit,omitempty"`
	Revision string `json:"revision,omitempty"`
}

func RegisterTools(s *mcp.Server) {
	schema := rt.InputSchema(nil)
	mcp.AddTool(s, &mcp.Tool{Name: "repository_branches", Description: "List repository branches.", InputSchema: schema}, func(c context.Context, _ *mcp.CallToolRequest, in input) (*mcp.CallToolResult, []string, error) {
		o, e := rt.Branches(c, in.ConnectionInput)
		return nil, o, e
	})
	mcp.AddTool(s, &mcp.Tool{Name: "repository_status", Description: "Read the resolved revision and worktree status when applicable; bare Bundle snapshots have no worktree.", InputSchema: schema}, func(c context.Context, _ *mcp.CallToolRequest, in input) (*mcp.CallToolResult, map[string]any, error) {
		o, e := rt.Status(c, in.ConnectionInput)
		return nil, o, e
	})
	mcp.AddTool(s, &mcp.Tool{Name: "repository_metadata", Description: "Read remotes and recent commits at the resolved revision; Bundle repositories may have no remote.", InputSchema: schema}, func(c context.Context, _ *mcp.CallToolRequest, in input) (*mcp.CallToolResult, map[string]any, error) {
		o, e := rt.Metadata(c, in.ConnectionInput)
		return nil, o, e
	})
	mcp.AddTool(s, &mcp.Tool{Name: "repository_tree", Description: "Page through every repository file; continue until done is true.", InputSchema: rt.InputSchema(map[string]any{"path": map[string]any{"type": "string"}, "offset": map[string]any{"type": "integer", "minimum": 0}, "limit": map[string]any{"type": "integer", "minimum": 1, "maximum": 500}, "revision": map[string]any{"type": "string"}})}, func(c context.Context, _ *mcp.CallToolRequest, in pathInput) (*mcp.CallToolResult, rt.TreePage, error) {
		o, e := rt.TreePageAt(c, in.ConnectionInput, in.Path, int(in.Offset), in.Limit, in.Revision)
		return nil, o, e
	})
	mcp.AddTool(s, &mcp.Tool{Name: "repository_file", Description: "Read a text file in byte ranges; continue from next_offset until done is true.", InputSchema: rt.InputSchema(map[string]any{"path": map[string]any{"type": "string"}, "offset": map[string]any{"type": "integer", "minimum": 0}, "max_bytes": map[string]any{"type": "integer", "minimum": 1, "maximum": 1048576}, "revision": map[string]any{"type": "string"}})}, func(c context.Context, _ *mcp.CallToolRequest, in pathInput) (*mcp.CallToolResult, rt.FileRange, error) {
		o, e := rt.FileRangeAt(c, in.ConnectionInput, in.Path, in.Offset, in.MaxBytes, in.Revision)
		return nil, o, e
	})
	mcp.AddTool(s, &mcp.Tool{Name: "repository_search", Description: "Page through literal text matches; continue until done is true and confirm complete is true. observed_matches is a lower bound when incomplete; narrow path if truncated.", InputSchema: rt.InputSchema(map[string]any{"query": map[string]any{"type": "string"}, "path": map[string]any{"type": "string"}, "offset": map[string]any{"type": "integer", "minimum": 0}, "limit": map[string]any{"type": "integer", "minimum": 1, "maximum": 100}, "revision": map[string]any{"type": "string"}})}, func(c context.Context, _ *mcp.CallToolRequest, in searchInput) (*mcp.CallToolResult, rt.SearchPage, error) {
		o, e := rt.SearchPageAt(c, in.ConnectionInput, in.Query, in.Path, in.Offset, in.Limit, in.Revision)
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
