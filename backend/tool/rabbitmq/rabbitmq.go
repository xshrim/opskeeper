package rabbitmq

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type ConnectionInput struct {
	URL            string `json:"url,omitempty"`
	Username       string `json:"username,omitempty"`
	Password       string `json:"password,omitempty"`
	TimeoutSeconds int    `json:"timeout_seconds,omitempty"`
	TLSInsecure    bool   `json:"tls_insecure,omitempty"`
}
type ToolInfo struct{ Name, Description string }

var tools = []ToolInfo{{"rabbitmq_health", "Check RabbitMQ Management API health and overview."}, {"rabbitmq_overview", "Read RabbitMQ cluster overview and message totals."}, {"rabbitmq_nodes", "List RabbitMQ nodes and resource usage."}, {"rabbitmq_queues", "List bounded RabbitMQ queues and message rates."}, {"rabbitmq_exchanges", "List RabbitMQ exchanges."}, {"rabbitmq_connections", "List RabbitMQ client connections."}, {"rabbitmq_channels", "List RabbitMQ channels and consumer counts."}, {"rabbitmq_consumers", "List RabbitMQ consumers."}, {"rabbitmq_vhosts", "List RabbitMQ virtual hosts."}}

func ListTools() []ToolInfo { return append([]ToolInfo(nil), tools...) }
func InputSchema(extra map[string]any) map[string]any {
	p := map[string]any{"url": map[string]any{"type": "string", "format": "uri"}, "username": map[string]any{"type": "string"}, "password": map[string]any{"type": "string"}, "timeout_seconds": map[string]any{"type": "integer", "minimum": 1, "maximum": 300}, "tls_insecure": map[string]any{"type": "boolean"}}
	for k, v := range extra {
		p[k] = v
	}
	return map[string]any{"type": "object", "properties": p, "additionalProperties": false}
}
func endpoint(in ConnectionInput) (string, error) {
	raw := strings.TrimSpace(in.URL)
	if raw == "" {
		return "", errors.New("url is required")
	}
	u, e := url.Parse(raw)
	if e != nil || u.Scheme != "http" && u.Scheme != "https" || u.Host == "" {
		return "", errors.New("url must be an absolute HTTP or HTTPS URL")
	}
	u.Path = strings.TrimRight(u.Path, "/")
	if !strings.HasSuffix(u.Path, "/api") {
		u.Path += "/api"
	}
	return strings.TrimRight(u.String(), "/"), nil
}
func client(in ConnectionInput) *http.Client {
	timeout := 10 * time.Second
	if in.TimeoutSeconds > 0 {
		timeout = time.Duration(in.TimeoutSeconds) * time.Second
	}
	return &http.Client{Timeout: timeout, Transport: &http.Transport{TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12, InsecureSkipVerify: in.TLSInsecure}}}
}
func get(ctx context.Context, in ConnectionInput, path string) (any, error) {
	base, e := endpoint(in)
	if e != nil {
		return nil, e
	}
	req, e := http.NewRequestWithContext(ctx, http.MethodGet, base+"/"+strings.TrimLeft(path, "/"), nil)
	if e != nil {
		return nil, e
	}
	if strings.TrimSpace(in.Username) != "" {
		req.SetBasicAuth(in.Username, in.Password)
	}
	resp, e := client(in).Do(req)
	if e != nil {
		return nil, e
	}
	defer resp.Body.Close()
	body, e := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if e != nil {
		return nil, e
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("RabbitMQ Management API returned HTTP %d", resp.StatusCode)
	}
	var out any
	if e = json.Unmarshal(body, &out); e != nil {
		return nil, e
	}
	return out, nil
}
func Health(ctx context.Context, in ConnectionInput) (map[string]any, error) {
	v, e := get(ctx, in, "overview")
	if e != nil {
		return nil, e
	}
	return map[string]any{"status": "ok", "overview": v}, nil
}
func Overview(ctx context.Context, in ConnectionInput) (any, error) { return get(ctx, in, "overview") }
func Nodes(ctx context.Context, in ConnectionInput) (any, error)    { return get(ctx, in, "nodes") }
func Queues(ctx context.Context, in ConnectionInput) (any, error) {
	return get(ctx, in, "queues?disable_stats=true")
}
func Exchanges(ctx context.Context, in ConnectionInput) (any, error) {
	return get(ctx, in, "exchanges")
}
func Connections(ctx context.Context, in ConnectionInput) (any, error) {
	return get(ctx, in, "connections")
}
func Channels(ctx context.Context, in ConnectionInput) (any, error) { return get(ctx, in, "channels") }
func Consumers(ctx context.Context, in ConnectionInput) (any, error) {
	return get(ctx, in, "consumers")
}
func Vhosts(ctx context.Context, in ConnectionInput) (any, error) { return get(ctx, in, "vhosts") }
