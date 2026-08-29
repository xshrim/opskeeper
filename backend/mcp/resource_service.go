package mcp

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"regexp"
	"strings"
	"time"

	"opskeeper/backend/authorization"
	"opskeeper/backend/resource"
)

var ErrInvalid = errors.New("invalid MCP server configuration")
var ErrNotFound = errors.New("MCP snapshot not found")

type ResourceReader interface {
	Get(context.Context, string) (resource.Resource, error)
}

type Service struct {
	resources        ResourceReader
	store            SnapshotStore
	enhancedSecurity bool
}

func NewService(resources ResourceReader, store SnapshotStore) *Service {
	return &Service{resources: resources, store: store}
}

func NewServiceWithSecurity(resources ResourceReader, store SnapshotStore, enhancedSecurity bool) *Service {
	return &Service{resources: resources, store: store, enhancedSecurity: enhancedSecurity}
}

func (s *Service) Discover(ctx context.Context, resourceID string) (Snapshot, error) {
	if s == nil || s.resources == nil || s.store == nil {
		return Snapshot{}, errors.New("MCP service dependencies are unavailable")
	}
	server, config, err := s.server(ctx, resourceID)
	if err != nil {
		return Snapshot{}, err
	}
	item, discoverErr := DiscoverWithSecurity(ctx, config.URL, time.Duration(config.TimeoutSeconds)*time.Second, s.enhancedSecurity)
	item.ServerResourceID, item.ScopeID, item.Status, item.Untrusted = server.ID, server.ScopeID, "succeeded", true
	if discoverErr != nil {
		item.Status = "failed"
		item.ErrorMessage = safeError(discoverErr)
		item.Hash = errorHash(item.ErrorMessage)
		if _, err := s.store.Save(ctx, item); err != nil {
			return Snapshot{}, err
		}
		return item, discoverErr
	}
	item.Tools = filterTools(item.Tools, config.ToolAllowlist)
	raw, _ := json.Marshal(item.Tools)
	sum := sha256.Sum256(raw)
	item.Hash = hex.EncodeToString(sum[:])
	return s.store.Save(ctx, item)
}

func (s *Service) ListSnapshots(ctx context.Context, resourceID string, limit int) ([]Snapshot, error) {
	if s == nil || s.resources == nil || s.store == nil {
		return nil, errors.New("MCP service dependencies are unavailable")
	}
	if _, _, err := s.server(ctx, resourceID); err != nil {
		return nil, err
	}
	return s.store.List(ctx, resourceID, limit)
}

// Call requires current resource scope and filter access before network I/O.
// The result stays raw JSON with untrusted=false impossible: callers must
// treat every textual field as data rather than instructions or HTML.
func (s *Service) Call(ctx context.Context, resourceID, name string, arguments map[string]any) (json.RawMessage, error) {
	if s == nil || s.resources == nil || s.store == nil {
		return nil, errors.New("MCP service dependencies are unavailable")
	}
	_, config, err := s.server(ctx, resourceID)
	if err != nil {
		return nil, err
	}
	allowed := make(map[string]bool, len(config.ToolAllowlist))
	for _, tool := range config.ToolAllowlist {
		allowed[tool] = true
	}
	return CallBoundedWithSecurity(ctx, config.URL, name, allowed, arguments, time.Duration(config.TimeoutSeconds)*time.Second, config.MaxResponseBytes, s.enhancedSecurity)
}

func (s *Service) server(ctx context.Context, resourceID string) (resource.Resource, Config, error) {
	if s == nil || s.resources == nil {
		return resource.Resource{}, Config{}, errors.New("MCP resource service is unavailable")
	}
	server, err := s.resources.Get(ctx, resourceID)
	if err != nil {
		return resource.Resource{}, Config{}, err
	}
	if server.Kind != "MCPServer" || server.Status != resource.StatusActive || !allows(ctx, server.ScopeID, server.ID) {
		return resource.Resource{}, Config{}, authorization.ErrForbidden
	}
	config, err := configFromWithSecurity(server.Config, s.enhancedSecurity)
	if err != nil {
		return resource.Resource{}, Config{}, err
	}
	return server, config, nil
}

func configFrom(input map[string]any) (Config, error) {
	return configFromWithSecurity(input, false)
}

func configFromWithSecurity(input map[string]any, enhancedSecurity bool) (Config, error) {
	raw, err := json.Marshal(input)
	if err != nil {
		return Config{}, ErrInvalid
	}
	var config Config
	if err := json.Unmarshal(raw, &config); err != nil {
		return Config{}, ErrInvalid
	}
	if config.Transport != "streamable_http" || len(config.ToolAllowlist) == 0 {
		return Config{}, ErrInvalid
	}
	if _, err := endpointURL(config.URL, enhancedSecurity); err != nil {
		return Config{}, ErrInvalid
	}
	if config.TimeoutSeconds <= 0 || config.TimeoutSeconds > 60 {
		config.TimeoutSeconds = 10
	}
	if config.MaxResponseBytes <= 0 || config.MaxResponseBytes > defaultMaxResponseBytes {
		config.MaxResponseBytes = defaultMaxResponseBytes
	}
	seen := map[string]bool{}
	toolName := regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_.:-]{0,127}$`)
	for _, name := range config.ToolAllowlist {
		if !toolName.MatchString(name) || seen[name] {
			return Config{}, ErrInvalid
		}
		seen[name] = true
	}
	return config, nil
}

func allows(ctx context.Context, scopeID, resourceID string) bool {
	if filter, ok := authorization.ResourceFilterFromContext(ctx); ok {
		return filter.Allows(scopeID, resourceID)
	}
	if filter, ok := authorization.ScopeFilterFromContext(ctx); ok {
		return filter.Allows(scopeID)
	}
	return true
}

func filterTools(tools []Tool, allowed []string) []Tool {
	set := map[string]bool{}
	for _, item := range allowed {
		set[item] = true
	}
	out := []Tool{}
	for _, item := range tools {
		if set[item.Name] {
			out = append(out, item)
		}
	}
	return out
}
func errorHash(message string) string {
	sum := sha256.Sum256([]byte(message))
	return hex.EncodeToString(sum[:])
}
func safeError(err error) string { return strings.TrimSpace(err.Error()) }
