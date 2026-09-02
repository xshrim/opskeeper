package mcp

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"path"
	"regexp"
	"strings"
	"time"

	gomcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"opskeeper/backend/authorization"
	"opskeeper/backend/resource"
)

var ErrInvalid = errors.New("invalid MCP server configuration")
var ErrNotFound = errors.New("MCP snapshot not found")

type ResourceReader interface {
	Get(context.Context, string) (resource.Resource, error)
}
type CredentialReader interface {
	RevealLinked(context.Context, string) ([]byte, error)
}

type Service struct {
	resources        ResourceReader
	store            SnapshotStore
	enhancedSecurity bool
	credentials      CredentialReader
}

// DraftConfig is used by the creation wizard to verify an MCP endpoint before
// a resource or credential is persisted.
type DraftConfig struct {
	Transport        string            `json:"transport"`
	URL              string            `json:"url"`
	Token            string            `json:"token"`
	RequestHeaders   map[string]string `json:"request_headers"`
	ToolAllowlist    []string          `json:"tool_allowlist"`
	TimeoutSeconds   int               `json:"timeout_seconds"`
	MaxResponseBytes int64             `json:"max_response_bytes"`
}

func NewService(resources ResourceReader, store SnapshotStore) *Service {
	return &Service{resources: resources, store: store}
}

func NewServiceWithSecurity(resources ResourceReader, store SnapshotStore, enhancedSecurity bool, credentials ...CredentialReader) *Service {
	var reader CredentialReader
	if len(credentials) > 0 {
		reader = credentials[0]
	}
	return &Service{resources: resources, store: store, enhancedSecurity: enhancedSecurity, credentials: reader}
}

// TestDraft performs initialization and tools/list against an in-memory
// configuration. It deliberately does not write a snapshot or resource.
func (s *Service) TestDraft(ctx context.Context, draft DraftConfig) (Snapshot, error) {
	input := map[string]any{
		"transport":          draft.Transport,
		"url":                draft.URL,
		"tool_allowlist":     draft.ToolAllowlist,
		"timeout_seconds":    draft.TimeoutSeconds,
		"max_response_bytes": draft.MaxResponseBytes,
		"request_headers":    draft.RequestHeaders,
	}
	config, err := configFromWithSecurity(input, s != nil && s.enhancedSecurity)
	if err != nil {
		return Snapshot{}, err
	}
	headers := make(map[string]string, len(config.RequestHeaders)+1)
	for key, value := range config.RequestHeaders {
		headers[key] = value
	}
	if token := strings.TrimSpace(draft.Token); token != "" {
		headers["Authorization"] = "Bearer " + token
	}
	if err := validateHeaders(headers); err != nil {
		return Snapshot{}, ErrInvalid
	}
	started := time.Now()
	item, discoverErr := discoverWithSecurity(ctx, config.URL, time.Duration(config.TimeoutSeconds)*time.Second, s != nil && s.enhancedSecurity, config.Transport, headers)
	item.Status = "succeeded"
	item.Untrusted = true
	item.ErrorMessage = ""
	if discoverErr != nil {
		item.Status = "failed"
		item.ErrorMessage = safeError(discoverErr)
		item.Hash = errorHash(item.ErrorMessage)
	}
	item.LatencyMS = time.Since(started).Milliseconds()
	item.Tools = filterTools(item.Tools, config.ToolAllowlist)
	if discoverErr == nil {
		raw, _ := json.Marshal(item.Tools)
		sum := sha256.Sum256(raw)
		item.Hash = hex.EncodeToString(sum[:])
	}
	return item, nil
}

func (s *Service) Discover(ctx context.Context, resourceID string) (Snapshot, error) {
	if s == nil || s.resources == nil || s.store == nil {
		return Snapshot{}, errors.New("MCP service dependencies are unavailable")
	}
	server, config, err := s.server(ctx, resourceID)
	if err != nil {
		return Snapshot{}, err
	}
	headers := s.requestHeaders(ctx, server)
	item, discoverErr := discoverWithSecurity(ctx, config.URL, time.Duration(config.TimeoutSeconds)*time.Second, s.enhancedSecurity, config.Transport, headers)
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
	server, config, err := s.server(ctx, resourceID)
	if err != nil {
		return nil, err
	}
	return s.callConfigured(ctx, server, config, name, arguments)
}

func (s *Service) callConfigured(ctx context.Context, server resource.Resource, config Config, name string, arguments map[string]any) (json.RawMessage, error) {
	name = strings.TrimSpace(name)
	if !matchesTool(name, config.ToolAllowlist) {
		return nil, errors.New("MCP tool is not allowlisted")
	}
	headers := s.requestHeaders(ctx, server)
	normalized, err := endpointURL(config.URL, s.enhancedSecurity)
	if err != nil {
		return nil, err
	}
	discovered, err := discoverWithSecurity(ctx, normalized.String(), time.Duration(config.TimeoutSeconds)*time.Second, s.enhancedSecurity, config.Transport, headers)
	if err != nil {
		return nil, err
	}
	found := false
	for _, tool := range discovered.Tools {
		if tool.Name == name {
			found = true
			break
		}
	}
	if !found {
		return nil, fmt.Errorf("MCP tool %q is not available on the server", name)
	}
	timeout := time.Duration(config.TimeoutSeconds) * time.Second
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	client := gomcp.NewClient(&gomcp.Implementation{Name: "opskeeper", Version: "t14"}, nil)
	session, err := client.Connect(ctx, clientTransport(config.Transport, normalized.String(), httpClient(timeout, s.enhancedSecurity, headers)), nil)
	if err != nil {
		return nil, err
	}
	defer session.Close()
	result, err := session.CallTool(ctx, &gomcp.CallToolParams{Name: name, Arguments: arguments})
	if err != nil {
		return nil, err
	}
	raw, err := json.Marshal(result)
	if err != nil {
		return nil, err
	}
	raw = normalizeToolResult(raw)
	if int64(len(raw)) > config.MaxResponseBytes {
		return nil, errors.New("MCP response exceeds configured size limit")
	}
	return raw, nil
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
	if config.Transport != "streamable_http" && config.Transport != "sse" {
		return Config{}, ErrInvalid
	}
	if _, err := endpointURL(config.URL, enhancedSecurity); err != nil {
		return Config{}, ErrInvalid
	}
	if config.TimeoutSeconds <= 0 || config.TimeoutSeconds > 600 {
		config.TimeoutSeconds = 120
	}
	if config.MaxResponseBytes <= 0 || config.MaxResponseBytes > maxResponseBytesLimit {
		config.MaxResponseBytes = defaultMaxResponseBytes
	}
	if config.RequestHeaders != nil {
		if err := validateHeaders(config.RequestHeaders); err != nil {
			return Config{}, ErrInvalid
		}
	}
	seen := map[string]bool{}
	toolName := regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_.:*?\[\]-]{0,127}$`)
	for _, name := range config.ToolAllowlist {
		if !toolName.MatchString(name) || seen[name] {
			return Config{}, ErrInvalid
		}
		seen[name] = true
	}
	return config, nil
}

func (s *Service) requestHeaders(ctx context.Context, server resource.Resource) map[string]string {
	headers := map[string]string{}
	for key, value := range configHeaders(server.Config) {
		headers[key] = value
	}
	if server.CredentialID != nil && s.credentials != nil {
		if raw, err := s.credentials.RevealLinked(ctx, *server.CredentialID); err == nil {
			var secret struct {
				Token   string            `json:"token"`
				Headers map[string]string `json:"headers"`
			}
			if json.Unmarshal(raw, &secret) == nil {
				for key, value := range secret.Headers {
					headers[key] = value
				}
				if strings.TrimSpace(secret.Token) != "" {
					headers["Authorization"] = "Bearer " + strings.TrimSpace(secret.Token)
				}
			} else if token := strings.TrimSpace(string(raw)); token != "" {
				headers["Authorization"] = "Bearer " + token
			}
		}
	}
	if err := validateHeaders(headers); err != nil {
		return map[string]string{}
	}
	return headers
}

func configHeaders(input map[string]any) map[string]string {
	config, ok := input["request_headers"].(map[string]any)
	if !ok {
		return nil
	}
	headers := make(map[string]string, len(config))
	for key, value := range config {
		if text, ok := value.(string); ok {
			headers[key] = text
		}
	}
	return headers
}

func validateHeaders(headers map[string]string) error {
	for key, value := range headers {
		if !validHeaderName(key) || strings.ContainsAny(value, "\r\n") {
			return errors.New("invalid MCP request header")
		}
		if http.CanonicalHeaderKey(key) == "" {
			return errors.New("invalid MCP request header")
		}
	}
	return nil
}

func validHeaderName(name string) bool {
	if strings.TrimSpace(name) != name || name == "" {
		return false
	}
	for _, ch := range name {
		if !(ch >= 'a' && ch <= 'z' || ch >= 'A' && ch <= 'Z' || ch >= '0' && ch <= '9' || strings.ContainsRune("!#$%&'*+-.^_`|~", ch)) {
			return false
		}
	}
	return true
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
	if len(allowed) == 0 {
		return tools
	}
	out := []Tool{}
	for _, item := range tools {
		if matchesTool(item.Name, allowed) {
			out = append(out, item)
		}
	}
	return out
}
func matchesTool(name string, patterns []string) bool {
	if len(patterns) == 0 {
		return true
	}
	for _, pattern := range patterns {
		matched, _ := path.Match(pattern, name)
		if matched || pattern == name {
			return true
		}
	}
	return false
}
func errorHash(message string) string {
	sum := sha256.Sum256([]byte(message))
	return hex.EncodeToString(sum[:])
}
func safeError(err error) string { return strings.TrimSpace(err.Error()) }
