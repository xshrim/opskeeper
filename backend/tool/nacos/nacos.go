package nacos

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type ConnectionInput struct {
	Host           string `json:"host,omitempty"`
	Port           int    `json:"port,omitempty"`
	Scheme         string `json:"scheme,omitempty"`
	ContextPath    string `json:"context_path,omitempty"`
	Username       string `json:"username,omitempty"`
	Password       string `json:"password,omitempty"`
	AccessToken    string `json:"access_token,omitempty"`
	TimeoutSeconds int    `json:"timeout_seconds,omitempty"`
}
type ServicesInput struct {
	NamespaceID string `json:"namespace_id,omitempty"`
	GroupName   string `json:"group_name,omitempty"`
	Page        int    `json:"page,omitempty"`
	PageSize    int    `json:"page_size,omitempty"`
}
type ServiceInstancesInput struct {
	ServiceName string `json:"service_name"`
	GroupName   string `json:"group_name,omitempty"`
	NamespaceID string `json:"namespace_id,omitempty"`
	HealthyOnly bool   `json:"healthy_only,omitempty"`
}
type ConfigsInput struct {
	NamespaceID string `json:"namespace_id,omitempty"`
	GroupName   string `json:"group_name,omitempty"`
	Search      string `json:"search,omitempty"`
	Page        int    `json:"page,omitempty"`
	PageSize    int    `json:"page_size,omitempty"`
}
type ConfigDetailInput struct {
	DataID      string `json:"data_id"`
	GroupName   string `json:"group_name,omitempty"`
	NamespaceID string `json:"namespace_id,omitempty"`
}
type JSONOutput struct {
	Data any `json:"data"`
}
type ToolInfo struct{ Name, Description string }

var tools = []ToolInfo{{"nacos_server_state", "Read Nacos server state and metrics."}, {"nacos_namespaces", "List Nacos namespaces."}, {"nacos_services", "List registered Nacos services."}, {"nacos_service_instances", "List instances of one Nacos service."}, {"nacos_configs", "List Nacos configuration metadata."}, {"nacos_config_detail", "Read one Nacos configuration value."}}

func ListTools() []ToolInfo { return append([]ToolInfo(nil), tools...) }
func InputSchema(extra map[string]any) map[string]any {
	p := map[string]any{"host": map[string]any{"type": "string"}, "port": map[string]any{"type": "integer", "minimum": 1, "maximum": 65535}, "scheme": map[string]any{"type": "string", "enum": []string{"http", "https"}}, "context_path": map[string]any{"type": "string"}, "username": map[string]any{"type": "string"}, "password": map[string]any{"type": "string"}, "access_token": map[string]any{"type": "string"}, "timeout_seconds": map[string]any{"type": "integer", "minimum": 1, "maximum": 300}}
	for k, v := range extra {
		p[k] = v
	}
	return map[string]any{"type": "object", "properties": p, "additionalProperties": false}
}

type Client struct {
	in    ConnectionInput
	http  *http.Client
	token string
}

func New(ctx context.Context, in ConnectionInput) (*Client, error) {
	if strings.TrimSpace(in.Host) == "" {
		return nil, errors.New("host is required")
	}
	if in.Port <= 0 {
		in.Port = 8848
	}
	if in.Scheme == "" {
		in.Scheme = "http"
	}
	if in.Scheme != "http" && in.Scheme != "https" {
		return nil, errors.New("scheme must be http or https")
	}
	if in.ContextPath == "" {
		in.ContextPath = "/nacos"
	}
	timeout := 10 * time.Second
	if in.TimeoutSeconds > 0 {
		timeout = time.Duration(in.TimeoutSeconds) * time.Second
	}
	c := &Client{in: in, http: &http.Client{Timeout: timeout}, token: in.AccessToken}
	if c.token == "" && in.Username != "" && in.Password != "" {
		if err := c.login(ctx); err != nil {
			return nil, err
		}
	}
	return c, nil
}
func (c *Client) base() string {
	return strings.TrimRight(fmt.Sprintf("%s://%s:%d%s", c.in.Scheme, c.in.Host, c.in.Port, strings.TrimRight(c.in.ContextPath, "/")), "/")
}
func (c *Client) login(ctx context.Context) error {
	v := url.Values{"username": {c.in.Username}, "password": {c.in.Password}}
	req, e := http.NewRequestWithContext(ctx, http.MethodPost, c.base()+"/v1/auth/login", strings.NewReader(v.Encode()))
	if e != nil {
		return e
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, e := c.http.Do(req)
	if e != nil {
		return e
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("Nacos login failed: %s", strings.TrimSpace(string(b)))
	}
	var out struct {
		AccessToken string `json:"accessToken"`
	}
	if json.Unmarshal(b, &out) != nil || out.AccessToken == "" {
		return errors.New("Nacos login response did not contain accessToken")
	}
	c.token = out.AccessToken
	return nil
}
func (c *Client) get(ctx context.Context, path string, q url.Values) (any, error) {
	if q == nil {
		q = url.Values{}
	}
	if c.token != "" {
		q.Set("accessToken", c.token)
	}
	req, e := http.NewRequestWithContext(ctx, http.MethodGet, c.base()+path+"?"+q.Encode(), nil)
	if e != nil {
		return nil, e
	}
	resp, e := c.http.Do(req)
	if e != nil {
		return nil, e
	}
	defer resp.Body.Close()
	b, e := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if e != nil {
		return nil, e
	}
	if resp.StatusCode/100 != 2 {
		return nil, fmt.Errorf("Nacos API %s returned %s: %s", path, resp.Status, string(b))
	}
	var out any
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, err
	}
	return out, nil
}
func (c *Client) Health(ctx context.Context) (any, error) {
	return c.get(ctx, "/v1/ns/operator/metrics", nil)
}
func (c *Client) Namespaces(ctx context.Context) (any, error) {
	return c.get(ctx, "/v1/console/namespaces", nil)
}
func (c *Client) Services(ctx context.Context, in ServicesInput) (any, error) {
	q := url.Values{"namespaceId": {in.NamespaceID}, "groupName": {in.GroupName}, "pageNo": {strconv.Itoa(page(in.Page))}, "pageSize": {strconv.Itoa(pageSize(in.PageSize))}}
	return c.get(ctx, "/v1/ns/service/list", q)
}
func (c *Client) ServiceInstances(ctx context.Context, in ServiceInstancesInput) (any, error) {
	if strings.TrimSpace(in.ServiceName) == "" {
		return nil, errors.New("service_name is required")
	}
	q := url.Values{"serviceName": {in.ServiceName}, "groupName": {in.GroupName}, "namespaceId": {in.NamespaceID}, "healthyOnly": {strconv.FormatBool(in.HealthyOnly)}}
	return c.get(ctx, "/v1/ns/instance/list", q)
}
func (c *Client) Configs(ctx context.Context, in ConfigsInput) (any, error) {
	q := url.Values{"tenant": {in.NamespaceID}, "group": {in.GroupName}, "search": {in.Search}, "pageNo": {strconv.Itoa(page(in.Page))}, "pageSize": {strconv.Itoa(pageSize(in.PageSize))}}
	return c.get(ctx, "/v1/cs/configs", q)
}
func (c *Client) ConfigDetail(ctx context.Context, in ConfigDetailInput) (any, error) {
	if strings.TrimSpace(in.DataID) == "" {
		return nil, errors.New("data_id is required")
	}
	q := url.Values{"dataId": {in.DataID}, "group": {in.GroupName}, "tenant": {in.NamespaceID}}
	return c.get(ctx, "/v1/cs/configs", q)
}
func page(v int) int {
	if v < 1 {
		return 1
	}
	if v > 1000 {
		return 1000
	}
	return v
}
func pageSize(v int) int {
	if v < 1 {
		return 100
	}
	if v > 100 {
		return 100
	}
	return v
}
func Health(ctx context.Context, in ConnectionInput) (JSONOutput, error) {
	c, e := New(ctx, in)
	if e != nil {
		return JSONOutput{}, e
	}
	v, e := c.Health(ctx)
	return JSONOutput{Data: v}, e
}
func Namespaces(ctx context.Context, in ConnectionInput) (JSONOutput, error) {
	c, e := New(ctx, in)
	if e != nil {
		return JSONOutput{}, e
	}
	v, e := c.Namespaces(ctx)
	return JSONOutput{Data: v}, e
}
func Services(ctx context.Context, in ConnectionInput, i ServicesInput) (JSONOutput, error) {
	c, e := New(ctx, in)
	if e != nil {
		return JSONOutput{}, e
	}
	v, e := c.Services(ctx, i)
	return JSONOutput{Data: v}, e
}
func ServiceInstances(ctx context.Context, in ConnectionInput, i ServiceInstancesInput) (JSONOutput, error) {
	c, e := New(ctx, in)
	if e != nil {
		return JSONOutput{}, e
	}
	v, e := c.ServiceInstances(ctx, i)
	return JSONOutput{Data: v}, e
}
func Configs(ctx context.Context, in ConnectionInput, i ConfigsInput) (JSONOutput, error) {
	c, e := New(ctx, in)
	if e != nil {
		return JSONOutput{}, e
	}
	v, e := c.Configs(ctx, i)
	return JSONOutput{Data: v}, e
}
func ConfigDetail(ctx context.Context, in ConnectionInput, i ConfigDetailInput) (JSONOutput, error) {
	c, e := New(ctx, in)
	if e != nil {
		return JSONOutput{}, e
	}
	v, e := c.ConfigDetail(ctx, i)
	return JSONOutput{Data: v}, e
}
