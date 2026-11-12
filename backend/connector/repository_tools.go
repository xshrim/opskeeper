package connector

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	a "opskeeper/backend/engine"
	rt "opskeeper/backend/tool/repository"
	"path/filepath"
	"strings"
)

func (s *Service) testRepositoryBundle(ctx context.Context, config map[string]any) error {
	backend := strings.ToLower(strings.TrimSpace(s.repositoryStorage.Backend))
	if backend == "" {
		backend = "postgres"
	}
	if backend == "postgresql" {
		backend = "postgres"
	}
	if backend != "local" && backend != "postgres" && backend != "s3" {
		return fmt.Errorf("unsupported repository storage backend %q", backend)
	}
	storageKey := stringValue(config, "storage_key")
	if storageKey == "" {
		return connectorError(CategoryConfiguration, "read Repository Bundle", false, errors.New("Bundle storage key is missing"))
	}
	path := ""
	if backend == "local" && s.repositoryStorage.Root != "" {
		path = filepath.Join(s.repositoryStorage.Root, storageKey)
	}
	in := rt.ConnectionInput{
		Path: path, StorageBackend: backend, StorageKey: storageKey,
		S3Endpoint: s.repositoryStorage.Endpoint, S3Bucket: s.repositoryStorage.Bucket,
		S3Prefix: s.repositoryStorage.Prefix, S3AccessKey: s.repositoryStorage.AccessKey,
		S3SecretKey: s.repositoryStorage.SecretKey, S3UseSSL: s.repositoryStorage.UseSSL,
		Postgres: s.postgresPool,
	}
	_, err := rt.Branches(ctx, in)
	return err
}

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
			register(t.Name, "Inventory tracked files at the selected revision. Defaults to 1000 entries per call; continue from next_offset until done=true.", repoSchema(map[string]any{"repository_path": map[string]any{"type": "string", "description": "Directory path relative to the repository root; omit or use an empty string for the whole repository."}, "start_offset": map[string]any{"type": "integer", "minimum": 0, "description": "Zero-based index of the first file to return; use the previous page's next_offset."}, "page_size": map[string]any{"type": "integer", "minimum": 1, "maximum": 1000, "description": "Maximum files in this page; defaults to 1000."}, "revision": map[string]any{"type": "string", "description": "Resolved branch, tag, or commit to inspect; omit to use the execution-pinned revision."}}), func(c context.Context, a map[string]any) (any, error) {
				limit := int(int64Arg(a, "page_size"))
				if limit <= 0 {
					limit = 1000
				}
				return snapshot.TreePage(c, stringArg(a, "repository_path"), int(int64Arg(a, "start_offset")), limit, stringArg(a, "revision"))
			})
		case "repository_file":
			register(t.Name, "Read a tracked text file by byte range; continue from next_offset until done=true.", repoSchema(map[string]any{"file_path": map[string]any{"type": "string", "description": "File path relative to the repository root."}, "byte_offset": map[string]any{"type": "integer", "minimum": 0, "description": "Zero-based byte offset to continue reading this file; use the previous range's next_offset."}, "max_bytes": map[string]any{"type": "integer", "minimum": 1, "maximum": 1048576, "description": "Maximum UTF-8 text bytes to return, up to 1 MiB."}, "revision": map[string]any{"type": "string", "description": "Resolved branch, tag, or commit; omit to use the execution-pinned revision."}}), func(c context.Context, a map[string]any) (any, error) {
				return snapshot.FileRange(c, stringArg(a, "file_path"), int64Arg(a, "byte_offset"), int64Arg(a, "max_bytes"), stringArg(a, "revision"))
			})
		case "repository_search":
			register(t.Name, "Search tracked text files. Set regular_expression=true for grep -E syntax, or use fixed-string matching by default. Results are paginated; continue until done=true and complete=true.", repoSchema(map[string]any{"search_pattern": map[string]any{"type": "string", "description": "Text or extended regular expression to search for, such as 'foo|bar', 'func[[:space:]]+Name', or 'TODO'."}, "repository_path": map[string]any{"type": "string", "description": "Optional repository-relative directory or file path to limit the search."}, "start_offset": map[string]any{"type": "integer", "minimum": 0, "description": "Zero-based match offset; use the previous page's next_offset."}, "page_size": map[string]any{"type": "integer", "minimum": 1, "maximum": 100, "description": "Maximum matches in this page, up to 100."}, "regular_expression": map[string]any{"type": "boolean", "description": "When true, interpret search_pattern as a grep -E extended regular expression; false uses literal text."}, "ignore_case": map[string]any{"type": "boolean", "description": "When true, match letters case-insensitively."}, "revision": map[string]any{"type": "string", "description": "Resolved branch, tag, or commit; omit to use the execution-pinned revision."}}), func(c context.Context, a map[string]any) (any, error) {
				return snapshot.SearchPage(c, rt.SearchOptions{Pattern: stringArg(a, "search_pattern"), Path: stringArg(a, "repository_path"), Offset: int(int64Arg(a, "start_offset")), Limit: int(int64Arg(a, "page_size")), RegularExpression: boolValueArg(a, "regular_expression"), IgnoreCase: boolValueArg(a, "ignore_case"), Revision: stringArg(a, "revision")})
			})
		case "repository_metadata":
			register(t.Name, t.Description, repoSchema(nil), func(c context.Context, _ map[string]any) (any, error) { return snapshot.Metadata(c) })
		}
	}
	return nil
}

func boolValueArg(args map[string]any, key string) bool {
	value, _ := args[key].(bool)
	return value
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
