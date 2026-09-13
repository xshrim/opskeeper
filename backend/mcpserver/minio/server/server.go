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
	mt "opskeeper/backend/tool/minio"
)

type Config struct {
	Address, BearerToken string
	Logger               *log.Logger
}

func ConfigFromEnv() Config {
	a := os.Getenv("MINIO_MCP_HTTP_ADDRESS")
	if strings.TrimSpace(a) == "" {
		a = "0.0.0.0:8820"
	}
	return Config{Address: a, BearerToken: os.Getenv("MINIO_MCP_BEARER_TOKEN")}
}
func New(cfg Config) (http.Handler, error) {
	if strings.TrimSpace(cfg.Address) == "" {
		return nil, errors.New("MinIO MCP HTTP address must not be empty")
	}
	s := mcp.NewServer(&mcp.Implementation{Name: "opskeeper-minio-mcp", Title: "OpsKeeper MinIO MCP", Description: "Read-only MinIO tools.", Version: "dev"}, nil)
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
	if strings.TrimSpace(cfg.BearerToken) != "" {
		h = auth.RequireBearerToken(func(_ context.Context, g string, _ *http.Request) (*auth.TokenInfo, error) {
			if g != cfg.BearerToken {
				return nil, auth.ErrInvalidToken
			}
			return &auth.TokenInfo{UserID: "minio-mcp-static-token"}, nil
		}, &auth.RequireBearerTokenOptions{AllowMissingExpiration: true})(h)
	}
	return h, nil
}

type input struct{ mt.ConnectionInput }
type bucketInput struct {
	mt.ConnectionInput
	Bucket string `json:"bucket"`
	Prefix string `json:"prefix,omitempty"`
	Limit  int    `json:"limit,omitempty"`
}
type objectInput struct {
	mt.ConnectionInput
	Bucket string `json:"bucket"`
	Object string `json:"object"`
}

func RegisterTools(s *mcp.Server) {
	schema := func(extra map[string]any) map[string]any { return mt.InputSchema(extra) }
	mcp.AddTool(s, &mcp.Tool{Name: "minio_health", Description: "Check MinIO connectivity and return a bounded bucket summary.", InputSchema: schema(nil)}, func(c context.Context, _ *mcp.CallToolRequest, in input) (*mcp.CallToolResult, any, error) {
		o, e := mt.Health(c, in.ConnectionInput)
		return nil, o, e
	})
	mcp.AddTool(s, &mcp.Tool{Name: "minio_buckets", Description: "List MinIO buckets visible to the configured credentials.", InputSchema: schema(nil)}, func(c context.Context, _ *mcp.CallToolRequest, in input) (*mcp.CallToolResult, any, error) {
		o, e := mt.Buckets(c, in.ConnectionInput)
		return nil, o, e
	})
	mcp.AddTool(s, &mcp.Tool{Name: "minio_bucket_objects", Description: "List a bounded set of objects in a MinIO bucket.", InputSchema: schema(map[string]any{"bucket": map[string]any{"type": "string"}, "prefix": map[string]any{"type": "string"}, "limit": map[string]any{"type": "integer"}, "__required": []string{"bucket"}})}, func(c context.Context, _ *mcp.CallToolRequest, in bucketInput) (*mcp.CallToolResult, any, error) {
		o, e := mt.BucketObjects(c, in.ConnectionInput, in.Bucket, in.Prefix, in.Limit)
		return nil, o, e
	})
	mcp.AddTool(s, &mcp.Tool{Name: "minio_object_stat", Description: "Read metadata for one MinIO object.", InputSchema: schema(map[string]any{"bucket": map[string]any{"type": "string"}, "object": map[string]any{"type": "string"}, "__required": []string{"bucket", "object"}})}, func(c context.Context, _ *mcp.CallToolRequest, in objectInput) (*mcp.CallToolResult, any, error) {
		o, e := mt.ObjectStat(c, in.ConnectionInput, in.Bucket, in.Object)
		return nil, o, e
	})
	mcp.AddTool(s, &mcp.Tool{Name: "minio_bucket_versioning", Description: "Read versioning configuration for one bucket.", InputSchema: schema(map[string]any{"bucket": map[string]any{"type": "string"}, "__required": []string{"bucket"}})}, func(c context.Context, _ *mcp.CallToolRequest, in bucketInput) (*mcp.CallToolResult, any, error) {
		o, e := mt.BucketVersioning(c, in.ConnectionInput, in.Bucket)
		return nil, o, e
	})
	mcp.AddTool(s, &mcp.Tool{Name: "minio_bucket_lifecycle", Description: "Read lifecycle configuration for one bucket.", InputSchema: schema(map[string]any{"bucket": map[string]any{"type": "string"}, "__required": []string{"bucket"}})}, func(c context.Context, _ *mcp.CallToolRequest, in bucketInput) (*mcp.CallToolResult, any, error) {
		o, e := mt.BucketLifecycle(c, in.ConnectionInput, in.Bucket)
		return nil, o, e
	})
}
