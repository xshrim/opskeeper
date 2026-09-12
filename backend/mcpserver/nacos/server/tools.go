package server

import (
	"context"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	nt "opskeeper/backend/tool/nacos"
)

type input struct{ nt.ConnectionInput }
type servicesInput struct {
	nt.ConnectionInput
	NamespaceID string `json:"namespace_id,omitempty"`
	GroupName   string `json:"group_name,omitempty"`
	Page        int    `json:"page,omitempty"`
	PageSize    int    `json:"page_size,omitempty"`
}
type instancesInput struct {
	nt.ConnectionInput
	ServiceName string `json:"service_name"`
	GroupName   string `json:"group_name,omitempty"`
	NamespaceID string `json:"namespace_id,omitempty"`
	HealthyOnly bool   `json:"healthy_only,omitempty"`
}
type configsInput struct {
	nt.ConnectionInput
	NamespaceID string `json:"namespace_id,omitempty"`
	GroupName   string `json:"group_name,omitempty"`
	Search      string `json:"search,omitempty"`
	Page        int    `json:"page,omitempty"`
	PageSize    int    `json:"page_size,omitempty"`
}
type detailInput struct {
	nt.ConnectionInput
	DataID      string `json:"data_id"`
	GroupName   string `json:"group_name,omitempty"`
	NamespaceID string `json:"namespace_id,omitempty"`
}

func schema(extra map[string]any) map[string]any { s := nt.InputSchema(extra); return s }
func RegisterTools(s *mcp.Server) {
	mcp.AddTool(s, &mcp.Tool{Name: "nacos_server_state", Description: "Read Nacos server state and metrics.", InputSchema: schema(nil)}, func(ctx context.Context, _ *mcp.CallToolRequest, in input) (*mcp.CallToolResult, nt.JSONOutput, error) {
		o, e := nt.Health(ctx, in.ConnectionInput)
		return nil, o, e
	})
	mcp.AddTool(s, &mcp.Tool{Name: "nacos_namespaces", Description: "List Nacos namespaces.", InputSchema: schema(nil)}, func(ctx context.Context, _ *mcp.CallToolRequest, in input) (*mcp.CallToolResult, nt.JSONOutput, error) {
		o, e := nt.Namespaces(ctx, in.ConnectionInput)
		return nil, o, e
	})
	mcp.AddTool(s, &mcp.Tool{Name: "nacos_services", Description: "List registered Nacos services.", InputSchema: schema(map[string]any{"namespace_id": map[string]any{"type": "string"}, "group_name": map[string]any{"type": "string"}, "page": map[string]any{"type": "integer"}, "page_size": map[string]any{"type": "integer"}})}, func(ctx context.Context, _ *mcp.CallToolRequest, in servicesInput) (*mcp.CallToolResult, nt.JSONOutput, error) {
		o, e := nt.Services(ctx, in.ConnectionInput, nt.ServicesInput{NamespaceID: in.NamespaceID, GroupName: in.GroupName, Page: in.Page, PageSize: in.PageSize})
		return nil, o, e
	})
	mcp.AddTool(s, &mcp.Tool{Name: "nacos_service_instances", Description: "List instances of one Nacos service.", InputSchema: schema(map[string]any{"service_name": map[string]any{"type": "string"}, "group_name": map[string]any{"type": "string"}, "namespace_id": map[string]any{"type": "string"}, "healthy_only": map[string]any{"type": "boolean"}})}, func(ctx context.Context, _ *mcp.CallToolRequest, in instancesInput) (*mcp.CallToolResult, nt.JSONOutput, error) {
		o, e := nt.ServiceInstances(ctx, in.ConnectionInput, nt.ServiceInstancesInput{ServiceName: in.ServiceName, GroupName: in.GroupName, NamespaceID: in.NamespaceID, HealthyOnly: in.HealthyOnly})
		return nil, o, e
	})
	mcp.AddTool(s, &mcp.Tool{Name: "nacos_configs", Description: "List Nacos configuration metadata.", InputSchema: schema(map[string]any{"namespace_id": map[string]any{"type": "string"}, "group_name": map[string]any{"type": "string"}, "search": map[string]any{"type": "string"}, "page": map[string]any{"type": "integer"}, "page_size": map[string]any{"type": "integer"}})}, func(ctx context.Context, _ *mcp.CallToolRequest, in configsInput) (*mcp.CallToolResult, nt.JSONOutput, error) {
		o, e := nt.Configs(ctx, in.ConnectionInput, nt.ConfigsInput{NamespaceID: in.NamespaceID, GroupName: in.GroupName, Search: in.Search, Page: in.Page, PageSize: in.PageSize})
		return nil, o, e
	})
	mcp.AddTool(s, &mcp.Tool{Name: "nacos_config_detail", Description: "Read one Nacos configuration value.", InputSchema: schema(map[string]any{"data_id": map[string]any{"type": "string"}, "group_name": map[string]any{"type": "string"}, "namespace_id": map[string]any{"type": "string"}})}, func(ctx context.Context, _ *mcp.CallToolRequest, in detailInput) (*mcp.CallToolResult, nt.JSONOutput, error) {
		o, e := nt.ConfigDetail(ctx, in.ConnectionInput, nt.ConfigDetailInput{DataID: in.DataID, GroupName: in.GroupName, NamespaceID: in.NamespaceID})
		return nil, o, e
	})
}
