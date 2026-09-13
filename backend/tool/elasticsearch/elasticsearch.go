package elasticsearch

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
	URL            string `json:"url"`
	Username       string `json:"username,omitempty"`
	Password       string `json:"password,omitempty"`
	TLSInsecure    bool   `json:"tls_insecure,omitempty"`
	TimeoutSeconds int    `json:"timeout_seconds,omitempty"`
}
type ToolInfo struct{ Name, Description string }

var tools = []ToolInfo{{"elasticsearch_health", "Read Elasticsearch cluster health."}, {"elasticsearch_cluster_info", "Read Elasticsearch cluster metadata."}, {"elasticsearch_nodes", "List Elasticsearch nodes and roles."}, {"elasticsearch_indices", "List user indices and storage/document metrics."}, {"elasticsearch_shards", "Read shard allocation summary."}, {"elasticsearch_settings", "Read selected cluster settings."}, {"elasticsearch_index_mapping", "Read one index mapping."}}

func ListTools() []ToolInfo { return append([]ToolInfo(nil), tools...) }
func InputSchema(extra map[string]any) map[string]any {
	p := map[string]any{"url": map[string]any{"type": "string"}, "username": map[string]any{"type": "string"}, "password": map[string]any{"type": "string"}, "tls_insecure": map[string]any{"type": "boolean"}, "timeout_seconds": map[string]any{"type": "integer", "minimum": 1, "maximum": 300}}
	for k, v := range extra {
		p[k] = v
	}
	return map[string]any{"type": "object", "properties": p, "additionalProperties": false}
}

type Client struct {
	base               string
	http               *http.Client
	auth               bool
	username, password string
}

func New(in ConnectionInput) (*Client, error) {
	base := strings.TrimRight(strings.TrimSpace(in.URL), "/")
	if base == "" {
		return nil, errors.New("url is required")
	}
	parsed, err := url.Parse(base)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return nil, errors.New("url must be an HTTP or HTTPS endpoint")
	}
	timeout := 10 * time.Second
	if in.TimeoutSeconds > 0 {
		timeout = time.Duration(in.TimeoutSeconds) * time.Second
	}
	tr := &http.Transport{}
	if strings.HasPrefix(strings.ToLower(base), "https://") {
		tr.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12, InsecureSkipVerify: in.TLSInsecure}
	}
	return &Client{base: base, http: &http.Client{Timeout: timeout, Transport: tr}, auth: in.Username != "", username: in.Username, password: in.Password}, nil
}
func (c *Client) get(ctx context.Context, path string) (any, error) {
	req, e := http.NewRequestWithContext(ctx, http.MethodGet, c.base+path, nil)
	if e != nil {
		return nil, e
	}
	if c.auth {
		req.SetBasicAuth(c.username, c.password)
	}
	resp, e := c.http.Do(req)
	if e != nil {
		return nil, e
	}
	defer resp.Body.Close()
	var out any
	dec := json.NewDecoder(io.LimitReader(resp.Body, 8<<20))
	if e = dec.Decode(&out); e != nil {
		return nil, e
	}
	if resp.StatusCode/100 != 2 {
		return nil, fmt.Errorf("Elasticsearch API %s returned %s", path, resp.Status)
	}
	return out, nil
}
func (c *Client) Health(x context.Context) (any, error)      { return c.get(x, "/_cluster/health") }
func (c *Client) ClusterInfo(x context.Context) (any, error) { return c.get(x, "/") }
func (c *Client) Nodes(x context.Context) (any, error) {
	return c.get(x, "/_nodes/stats/jvm,process,os,fs,indices,transport")
}
func (c *Client) Indices(x context.Context) (any, error) {
	return c.get(x, "/_cat/indices?format=json&expand_wildcards=open")
}
func (c *Client) Shards(x context.Context) (any, error) {
	return c.get(x, "/_cluster/health?level=indices")
}
func (c *Client) Settings(x context.Context) (any, error) {
	return c.get(x, "/_cluster/settings?include_defaults=true&flat_settings=true")
}
func (c *Client) IndexMapping(x context.Context, index string) (any, error) {
	if strings.TrimSpace(index) == "" {
		return nil, errors.New("index is required")
	}
	return c.get(x, "/"+url.PathEscape(index)+"/_mapping")
}
