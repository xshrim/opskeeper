// Package mcp provides a deliberately narrow wrapper over the official MCP Go
// SDK. It never implements JSON-RPC or calls a server without a resource,
// scope, tool whitelist and bounded transport.
package mcp

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"time"

	gomcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

const defaultMaxResponseBytes int64 = 1 << 20

type Tool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"input_schema"`
}
type Snapshot struct {
	ID               string    `json:"id,omitempty"`
	ServerResourceID string    `json:"server_resource_id,omitempty"`
	ScopeID          string    `json:"scope_id,omitempty"`
	ProtocolVersion  string    `json:"protocol_version"`
	ServerName       string    `json:"server_name"`
	ServerVersion    string    `json:"server_version"`
	Hash             string    `json:"content_hash"`
	Tools            []Tool    `json:"tools"`
	Status           string    `json:"status,omitempty"`
	ErrorMessage     string    `json:"error_message,omitempty"`
	CreatedAt        time.Time `json:"created_at,omitempty"`
	Untrusted        bool      `json:"untrusted"`
}

// Config is deliberately resource-owned. It describes only the transport and
// explicit tools available from one MCPServer resource; credentials remain in
// the normal credential store and are never copied into MCP snapshots.
type Config struct {
	Transport        string   `json:"transport"`
	URL              string   `json:"url"`
	ToolAllowlist    []string `json:"tool_allowlist"`
	TimeoutSeconds   int      `json:"timeout_seconds"`
	MaxResponseBytes int64    `json:"max_response_bytes"`
}

// Discover uses the official SDK's initialization and tools/list calls. Only
// HTTPS streamable HTTP is allowed for the first release; stdio commands are
// reserved for the Kubernetes Job sandbox rather than API/Worker processes.
func Discover(ctx context.Context, endpoint string, timeout time.Duration) (Snapshot, error) {
	return discoverWithSecurity(ctx, endpoint, timeout, false)
}

func DiscoverWithSecurity(ctx context.Context, endpoint string, timeout time.Duration, enhancedSecurity bool) (Snapshot, error) {
	return discoverWithSecurity(ctx, endpoint, timeout, enhancedSecurity)
}

func discoverWithSecurity(ctx context.Context, endpoint string, timeout time.Duration, enhancedSecurity bool) (Snapshot, error) {
	u, err := endpointURL(endpoint, enhancedSecurity)
	if err != nil {
		return Snapshot{}, err
	}
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	client := gomcp.NewClient(&gomcp.Implementation{Name: "opskeeper", Version: "t14"}, nil)
	session, err := client.Connect(ctx, &gomcp.StreamableClientTransport{Endpoint: u.String(), HTTPClient: httpClient(timeout, enhancedSecurity), DisableStandaloneSSE: true, MaxRetries: 0}, nil)
	if err != nil {
		return Snapshot{}, fmt.Errorf("connect MCP server: %w", err)
	}
	defer session.Close()
	result, err := session.ListTools(ctx, nil)
	if err != nil {
		return Snapshot{}, fmt.Errorf("list MCP tools: %w", err)
	}
	tools := make([]Tool, 0, len(result.Tools))
	for _, tool := range result.Tools {
		raw, _ := json.Marshal(tool.InputSchema)
		tools = append(tools, Tool{Name: tool.Name, Description: tool.Description, InputSchema: raw})
	}
	raw, _ := json.Marshal(tools)
	sum := sha256.Sum256(raw)
	init := session.InitializeResult()
	return Snapshot{ProtocolVersion: init.ProtocolVersion, ServerName: init.ServerInfo.Name, ServerVersion: init.ServerInfo.Version, Hash: hex.EncodeToString(sum[:]), Tools: tools, Untrusted: true}, nil
}

// Call invokes only a previously discovered, explicit whitelist entry and
// returns a response bounded before persistence or presentation to a model.
func Call(ctx context.Context, endpoint, name string, allowed map[string]bool, arguments map[string]any, timeout time.Duration) (json.RawMessage, error) {
	return CallBoundedWithSecurity(ctx, endpoint, name, allowed, arguments, timeout, defaultMaxResponseBytes, false)
}

func CallBounded(ctx context.Context, endpoint, name string, allowed map[string]bool, arguments map[string]any, timeout time.Duration, limit int64) (json.RawMessage, error) {
	return CallBoundedWithSecurity(ctx, endpoint, name, allowed, arguments, timeout, limit, false)
}

func CallBoundedWithSecurity(ctx context.Context, endpoint, name string, allowed map[string]bool, arguments map[string]any, timeout time.Duration, limit int64, enhancedSecurity bool) (json.RawMessage, error) {
	name = strings.TrimSpace(name)
	if name == "" || !allowed[name] {
		return nil, errors.New("MCP tool is not allowlisted")
	}
	normalized, err := endpointURL(endpoint, enhancedSecurity)
	if err != nil {
		return nil, err
	}
	discovered, err := DiscoverWithSecurity(ctx, normalized.String(), timeout, enhancedSecurity)
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
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	client := gomcp.NewClient(&gomcp.Implementation{Name: "opskeeper", Version: "t14"}, nil)
	session, err := client.Connect(ctx, &gomcp.StreamableClientTransport{Endpoint: normalized.String(), HTTPClient: httpClient(timeout, enhancedSecurity), DisableStandaloneSSE: true, MaxRetries: 0}, nil)
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
	if limit <= 0 || limit > defaultMaxResponseBytes {
		limit = defaultMaxResponseBytes
	}
	if int64(len(raw)) > limit {
		return nil, errors.New("MCP response exceeds configured size limit")
	}
	return raw, nil
}

func endpointURL(value string, enhancedSecurity bool) (*url.URL, error) {
	u, err := url.Parse(strings.TrimSpace(value))
	if err != nil || u.Scheme == "" || u.Host == "" || u.User != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return nil, errors.New("MCP endpoint must be an absolute HTTP(S) URL without user info")
	}
	if enhancedSecurity {
		if u.Scheme != "https" {
			return nil, errors.New("MCP endpoint must use HTTPS when enhanced security is enabled")
		}
		host := u.Hostname()
		if host == "" || host == "localhost" {
			return nil, errors.New("MCP endpoint host is not permitted by enhanced security")
		}
		if addr, err := netip.ParseAddr(host); err == nil && !isPublicAddress(addr) {
			return nil, errors.New("MCP endpoint address is not permitted by enhanced security")
		}
	}
	return u, nil
}

func safeEndpoint(value string) (*url.URL, error) { return endpointURL(value, true) }

func httpClient(timeout time.Duration, enhancedSecurity bool) *http.Client {
	if enhancedSecurity {
		return restrictedClient(timeout)
	}
	return &http.Client{Timeout: timeout}
}

func restrictedClient(timeout time.Duration) *http.Client {
	dialer := &net.Dialer{Timeout: timeout}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		host, _, err := net.SplitHostPort(address)
		if err != nil {
			return nil, err
		}
		ips, err := net.DefaultResolver.LookupNetIP(ctx, "ip", host)
		if err != nil {
			return nil, err
		}
		if len(ips) == 0 {
			return nil, errors.New("MCP endpoint has no resolved address")
		}
		for _, ip := range ips {
			if !isPublicAddress(ip) {
				return nil, errors.New("MCP endpoint resolves to a non-public address")
			}
		}
		return dialer.DialContext(ctx, network, address)
	}
	return &http.Client{Timeout: timeout, Transport: transport}
}

func isPublicAddress(addr netip.Addr) bool {
	addr = addr.Unmap()
	return addr.IsValid() && !addr.IsLoopback() && !addr.IsPrivate() && !addr.IsLinkLocalUnicast() && !addr.IsLinkLocalMulticast() && !addr.IsUnspecified() && !addr.IsMulticast()
}
