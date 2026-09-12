package connector

import (
	"context"
	"encoding/json"
	"fmt"
	a "opskeeper/backend/aiengine"
	rt "opskeeper/backend/tool/repository"
	"strings"
)

func (s *Service) resolveRepositoryTools(ctx context.Context, item a.ContextResource, add func(string, string, json.RawMessage, func(context.Context, map[string]any) (a.ToolResult, error))) error {
	if strings.EqualFold(strings.TrimSpace(item.Subtype), "agent") {
		return fmt.Errorf("Repository agent resources must use the MCP provider")
	}
	in := rt.ConnectionInput{Path: stringValue(item.Config, "path"), URL: stringValue(item.Config, "url"), Branch: stringValue(item.Config, "default_branch"), TimeoutSeconds: intValue(item.Config, "timeout_seconds"), StorageBackend: stringValue(item.Config, "storage_backend"), StorageKey: stringValue(item.Config, "storage_key"), S3Endpoint: stringValue(item.Config, "s3_endpoint"), S3Bucket: stringValue(item.Config, "s3_bucket"), S3Prefix: stringValue(item.Config, "s3_prefix")}
	register := func(name, desc string, schema json.RawMessage, fn func(context.Context, map[string]any) (any, error)) {
		add(name, desc, schema, func(c context.Context, args map[string]any) (a.ToolResult, error) {
			v, e := fn(c, args)
			if e != nil {
				return a.ToolResult{}, fmt.Errorf("%s: %w", name, e)
			}
			return a.ToolResult{Output: v, Untrusted: true}, nil
		})
	}
	for _, t := range rt.ListTools() {
		switch t.Name {
		case "repository_branches":
			register(t.Name, t.Description, repoSchema(nil), func(c context.Context, _ map[string]any) (any, error) { return rt.Branches(c, in) })
		case "repository_checkout":
			register(t.Name, t.Description, repoSchema(map[string]any{"branch": map[string]any{"type": "string"}}), func(c context.Context, a map[string]any) (any, error) {
				return rt.Checkout(c, in, stringArg(a, "branch"))
			})
		case "repository_status":
			register(t.Name, t.Description, repoSchema(nil), func(c context.Context, _ map[string]any) (any, error) { return rt.Status(c, in) })
		case "repository_tree":
			register(t.Name, t.Description, repoSchema(map[string]any{"path": map[string]any{"type": "string"}, "limit": map[string]any{"type": "integer"}}), func(c context.Context, a map[string]any) (any, error) {
				return rt.Tree(c, in, stringArg(a, "path"), int(int64Arg(a, "limit")))
			})
		case "repository_file":
			register(t.Name, t.Description, repoSchema(map[string]any{"path": map[string]any{"type": "string"}, "max_bytes": map[string]any{"type": "integer"}}), func(c context.Context, a map[string]any) (any, error) {
				return rt.File(c, in, stringArg(a, "path"), int64Arg(a, "max_bytes"))
			})
		case "repository_search":
			register(t.Name, t.Description, repoSchema(map[string]any{"query": map[string]any{"type": "string"}, "limit": map[string]any{"type": "integer"}}), func(c context.Context, a map[string]any) (any, error) {
				return rt.Search(c, in, stringArg(a, "query"), int(int64Arg(a, "limit")))
			})
		case "repository_metadata":
			register(t.Name, t.Description, repoSchema(nil), func(c context.Context, _ map[string]any) (any, error) { return rt.Metadata(c, in) })
		}
	}
	return nil
}
func repoSchema(extra map[string]any) json.RawMessage {
	b, _ := json.Marshal(rt.InputSchema(extra))
	return b
}
