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
	RepositoryPath string `json:"repository_path,omitempty"`
	FilePath       string `json:"file_path,omitempty"`
	StartOffset    int64  `json:"start_offset,omitempty"`
	ByteOffset     int64  `json:"byte_offset,omitempty"`
	MaxBytes       int64  `json:"max_bytes,omitempty"`
	PageSize       int    `json:"page_size,omitempty"`
	Revision       string `json:"revision,omitempty"`
}
type searchInput struct {
	rt.ConnectionInput
	SearchPattern     string `json:"search_pattern"`
	RepositoryPath    string `json:"repository_path,omitempty"`
	StartOffset       int    `json:"start_offset,omitempty"`
	PageSize          int    `json:"page_size,omitempty"`
	RegularExpression bool   `json:"regular_expression,omitempty"`
	IgnoreCase        bool   `json:"ignore_case,omitempty"`
	Revision          string `json:"revision,omitempty"`
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
	mcp.AddTool(s, &mcp.Tool{Name: "repository_tree", Description: "Inventory tracked files at the selected revision. Defaults to 1000 files per page; continue from next_offset until done is true.", InputSchema: rt.InputSchema(repositoryToolProperties(map[string]any{"repository_path": map[string]any{"type": "string", "description": "Directory relative to the repository root; omit for the whole repository."}, "start_offset": map[string]any{"type": "integer", "minimum": 0, "description": "Zero-based file offset; use the previous page's next_offset."}, "page_size": map[string]any{"type": "integer", "minimum": 1, "maximum": 1000, "description": "Maximum files to return, up to 1000; defaults to 1000."}, "revision": map[string]any{"type": "string", "description": "Branch, tag, or commit; omit to use the execution-pinned revision."}}))}, func(c context.Context, _ *mcp.CallToolRequest, in pathInput) (*mcp.CallToolResult, rt.TreePage, error) {
		o, e := rt.TreePageAt(c, in.ConnectionInput, in.RepositoryPath, int(in.StartOffset), in.PageSize, in.Revision)
		return nil, o, e
	})
	mcp.AddTool(s, &mcp.Tool{Name: "repository_file", Description: "Read a tracked UTF-8 text file by byte range; continue from next_offset until done is true.", InputSchema: rt.InputSchema(repositoryToolProperties(map[string]any{"file_path": map[string]any{"type": "string", "description": "File path relative to the repository root."}, "byte_offset": map[string]any{"type": "integer", "minimum": 0, "description": "Zero-based byte offset; use the previous range's next_offset."}, "max_bytes": map[string]any{"type": "integer", "minimum": 1, "maximum": 1048576, "description": "Maximum UTF-8 text bytes, up to 1 MiB."}, "revision": map[string]any{"type": "string", "description": "Branch, tag, or commit; omit to use the execution-pinned revision."}}))}, func(c context.Context, _ *mcp.CallToolRequest, in pathInput) (*mcp.CallToolResult, rt.FileRange, error) {
		o, e := rt.FileRangeAt(c, in.ConnectionInput, in.FilePath, in.ByteOffset, in.MaxBytes, in.Revision)
		return nil, o, e
	})
	mcp.AddTool(s, &mcp.Tool{Name: "repository_search", Description: "Search tracked text using fixed strings or grep -E extended regular expressions, with optional case-insensitive matching. Continue until done and complete are true.", InputSchema: rt.InputSchema(repositoryToolProperties(map[string]any{"search_pattern": map[string]any{"type": "string", "description": "Literal text or grep -E pattern, for example 'foo|bar' or 'func[[:space:]]+Name'."}, "repository_path": map[string]any{"type": "string", "description": "Optional repository-relative file or directory to restrict the search."}, "start_offset": map[string]any{"type": "integer", "minimum": 0, "description": "Zero-based result offset; use the previous page's next_offset."}, "page_size": map[string]any{"type": "integer", "minimum": 1, "maximum": 100, "description": "Maximum matches in this page, up to 100."}, "regular_expression": map[string]any{"type": "boolean", "description": "True interprets search_pattern as grep -E extended regular expression; false searches literal text."}, "ignore_case": map[string]any{"type": "boolean", "description": "True ignores letter case, matching grep -i."}, "revision": map[string]any{"type": "string", "description": "Branch, tag, or commit; omit to use the execution-pinned revision."}}))}, func(c context.Context, _ *mcp.CallToolRequest, in searchInput) (*mcp.CallToolResult, rt.SearchPage, error) {
		o, e := rt.SearchPageWithOptionsAt(c, in.ConnectionInput, rt.SearchOptions{Pattern: in.SearchPattern, Path: in.RepositoryPath, Offset: in.StartOffset, Limit: in.PageSize, Revision: in.Revision, RegularExpression: in.RegularExpression, IgnoreCase: in.IgnoreCase})
		return nil, o, e
	})
}

func repositoryToolProperties(operation map[string]any) map[string]any {
	properties := rt.InputSchema(nil)["properties"].(map[string]any)
	for _, key := range []string{"path", "url", "branch", "username", "password", "timeout_seconds", "storage_backend", "storage_key", "s3_endpoint", "s3_bucket", "s3_prefix"} {
		delete(properties, key)
	}
	for key, value := range operation {
		properties[key] = value
	}
	return properties
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
