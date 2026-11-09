package connector

import (
	"context"
	"encoding/json"
	"fmt"
	a "opskeeper/backend/engine"
	rt "opskeeper/backend/tool/repository"
	"path/filepath"
	"strings"
)

func (s *Service) resolveRepositoryTools(ctx context.Context, item a.ContextResource, add func(string, string, json.RawMessage, func(context.Context, map[string]any) (a.ToolResult, error)), setCleanup func(func() error)) error {
	if strings.EqualFold(strings.TrimSpace(item.Subtype), "agent") {
		return fmt.Errorf("Repository agent resources must use the MCP provider")
	}
	backend := ""
	if strings.EqualFold(strings.TrimSpace(item.Subtype), "bundle") {
		backend = s.repositoryStorage.Backend
		if backend == "" {
			backend = "postgres"
		}
	}
	storageKey := stringValue(item.Config, "storage_key")
	path, s3Endpoint, s3Bucket, s3Prefix := "", "", "", ""
	url, branch := "", ""
	if strings.EqualFold(strings.TrimSpace(item.Subtype), "git") {
		url = stringValue(item.Config, "url")
		branch = stringValue(item.Config, "default_branch")
	}
	if strings.EqualFold(strings.TrimSpace(item.Subtype), "bundle") {
		if s.repositoryStorage.Root != "" && backend == "local" && storageKey != "" {
			path = filepath.Join(s.repositoryStorage.Root, storageKey)
		}
		if s.repositoryStorage.Endpoint != "" {
			s3Endpoint = s.repositoryStorage.Endpoint
		}
		if s.repositoryStorage.Bucket != "" {
			s3Bucket = s.repositoryStorage.Bucket
		}
		if s.repositoryStorage.Prefix != "" {
			s3Prefix = s.repositoryStorage.Prefix
		}
	}
	in := rt.ConnectionInput{Path: path, URL: url, Branch: branch, TimeoutSeconds: intValue(item.Config, "timeout_seconds"), StorageBackend: backend, StorageKey: storageKey, S3Endpoint: s3Endpoint, S3Bucket: s3Bucket, S3Prefix: s3Prefix, S3AccessKey: s.repositoryStorage.AccessKey, S3SecretKey: s.repositoryStorage.SecretKey, S3UseSSL: s.repositoryStorage.UseSSL, Postgres: s.postgresPool}
	if item.Binding != nil {
		in.Directory = stringValue(item.Binding, "directory")
		if strings.TrimSpace(in.Directory) == "" {
			in.Directory = stringValue(item.Binding, "path")
		}
		if revision := stringValue(item.Binding, "revision"); revision != "" {
			in.PinnedRevision = revision
		} else if branch := stringValue(item.Binding, "branch"); branch != "" {
			in.Branch = branch
		}
	}
	secret, configured, err := s.resourceSecret(ctx, item.ID)
	if err != nil {
		return fmt.Errorf("read Repository credential: %w", err)
	}
	if configured {
		var credential struct {
			Username string `json:"username"`
			Password string `json:"password"`
		}
		if err := json.Unmarshal(secret, &credential); err != nil {
			return fmt.Errorf("decode Repository credential: %w", err)
		}
		in.Username, in.Password = credential.Username, credential.Password
	}
	snapshot := rt.NewSnapshot(in)
	if setCleanup != nil {
		setCleanup(snapshot.Close)
	}
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
			register(t.Name, t.Description, repoSchema(nil), func(c context.Context, _ map[string]any) (any, error) { return snapshot.Branches(c) })
		case "repository_status":
			register(t.Name, t.Description, repoSchema(nil), func(c context.Context, _ map[string]any) (any, error) { return snapshot.Status(c) })
		case "repository_tree":
			register(t.Name, t.Description, repoSchema(map[string]any{"path": map[string]any{"type": "string"}, "limit": map[string]any{"type": "integer"}, "revision": map[string]any{"type": "string"}}), func(c context.Context, a map[string]any) (any, error) {
				return snapshot.Tree(c, stringArg(a, "path"), int(int64Arg(a, "limit")), stringArg(a, "revision"))
			})
		case "repository_file":
			register(t.Name, t.Description, repoSchema(map[string]any{"path": map[string]any{"type": "string"}, "max_bytes": map[string]any{"type": "integer"}, "revision": map[string]any{"type": "string"}}), func(c context.Context, a map[string]any) (any, error) {
				return snapshot.File(c, stringArg(a, "path"), int64Arg(a, "max_bytes"), stringArg(a, "revision"))
			})
		case "repository_search":
			register(t.Name, t.Description, repoSchema(map[string]any{"query": map[string]any{"type": "string"}, "limit": map[string]any{"type": "integer"}, "revision": map[string]any{"type": "string"}}), func(c context.Context, a map[string]any) (any, error) {
				return snapshot.Search(c, stringArg(a, "query"), int(int64Arg(a, "limit")), stringArg(a, "revision"))
			})
		case "repository_metadata":
			register(t.Name, t.Description, repoSchema(nil), func(c context.Context, _ map[string]any) (any, error) { return snapshot.Metadata(c) })
		}
	}
	return nil
}
func repoSchema(extra map[string]any) json.RawMessage {
	schema := rt.InputSchema(extra)
	properties := schema["properties"].(map[string]any)
	for _, key := range []string{"path", "url", "branch", "username", "password", "timeout_seconds", "storage_backend", "storage_key", "s3_endpoint", "s3_bucket", "s3_prefix"} {
		delete(properties, key)
	}
	b, _ := json.Marshal(schema)
	return b
}
