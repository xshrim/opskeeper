package connector

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"opskeeper/backend/aiengine"
	mt "opskeeper/backend/tool/minio"
)

func (s *Service) resolveMinIOTools(ctx context.Context, item aiengine.ContextResource, add func(string, string, json.RawMessage, func(context.Context, map[string]any) (aiengine.ToolResult, error))) error {
	if strings.EqualFold(strings.TrimSpace(item.Subtype), "agent") {
		return fmt.Errorf("MinIO agent resources must use the MCP provider")
	}
	in := mt.ConnectionInput{Endpoint: stringValue(item.Config, "endpoint"), Region: stringValue(item.Config, "region"), Secure: configBool(item.Config, "secure"), TimeoutSeconds: intValue(item.Config, "timeout_seconds")}
	raw, configured, err := s.resourceSecret(ctx, item.ID)
	if err != nil {
		return err
	}
	if configured {
		var v map[string]any
		if err := json.Unmarshal(raw, &v); err != nil {
			return err
		}
		in.AccessKey, in.SecretKey, in.SessionToken = stringValue(v, "access_key"), stringValue(v, "secret_key"), stringValue(v, "session_token")
	}
	schema := func(extra map[string]any) json.RawMessage { return minioSchema(extra) }
	register := func(name, desc string, extra map[string]any, fn func(context.Context, map[string]any) (any, error)) {
		add(name, desc, schema(extra), func(c context.Context, args map[string]any) (aiengine.ToolResult, error) {
			out, err := fn(c, args)
			if err != nil {
				return aiengine.ToolResult{}, mt.Error(name, err)
			}
			return aiengine.ToolResult{Output: out, Untrusted: true}, nil
		})
	}
	for _, tool := range mt.ListTools() {
		switch tool.Name {
		case "minio_health":
			register(tool.Name, tool.Description, nil, func(c context.Context, _ map[string]any) (any, error) { return mt.Health(c, in) })
		case "minio_buckets":
			register(tool.Name, tool.Description, nil, func(c context.Context, _ map[string]any) (any, error) { return mt.Buckets(c, in) })
		case "minio_bucket_objects":
			register(tool.Name, tool.Description, map[string]any{"bucket": map[string]any{"type": "string"}, "prefix": map[string]any{"type": "string"}, "limit": map[string]any{"type": "integer"}, "__required": []string{"bucket"}}, func(c context.Context, a map[string]any) (any, error) {
				return mt.BucketObjects(c, in, stringValue(a, "bucket"), stringValue(a, "prefix"), intValue(a, "limit"))
			})
		case "minio_object_stat":
			register(tool.Name, tool.Description, map[string]any{"bucket": map[string]any{"type": "string"}, "object": map[string]any{"type": "string"}, "__required": []string{"bucket", "object"}}, func(c context.Context, a map[string]any) (any, error) {
				return mt.ObjectStat(c, in, stringValue(a, "bucket"), stringValue(a, "object"))
			})
		case "minio_bucket_versioning":
			register(tool.Name, tool.Description, map[string]any{"bucket": map[string]any{"type": "string"}, "__required": []string{"bucket"}}, func(c context.Context, a map[string]any) (any, error) {
				return mt.BucketVersioning(c, in, stringValue(a, "bucket"))
			})
		case "minio_bucket_lifecycle":
			register(tool.Name, tool.Description, map[string]any{"bucket": map[string]any{"type": "string"}, "__required": []string{"bucket"}}, func(c context.Context, a map[string]any) (any, error) {
				return mt.BucketLifecycle(c, in, stringValue(a, "bucket"))
			})
		}
	}
	return nil
}

func minioSchema(extra map[string]any) json.RawMessage {
	schema := mt.InputSchema(extra)
	if properties, ok := schema["properties"].(map[string]any); ok {
		for _, key := range []string{"endpoint", "access_key", "secret_key", "session_token", "region", "secure", "timeout_seconds", "__required"} {
			delete(properties, key)
		}
	}
	if required, ok := extra["__required"]; ok {
		schema["required"] = required
	}
	encoded, _ := json.Marshal(schema)
	return encoded
}
