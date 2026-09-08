package server

import (
	"context"
	"encoding/json"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"opskeeper/backend/mcpserver/kubernetes/client"
	kt "opskeeper/backend/tool/kubernetes"
)

const maxListLimit = 500

type baseInput struct{ client.ConnectionInput }
type listInput struct {
	client.ConnectionInput
	Namespace string `json:"namespace,omitempty"`
	Filters   string `json:"filters,omitempty"`
	Limit     int    `json:"limit,omitempty"`
	Continue  string `json:"continue,omitempty"`
}
type getInput struct {
	client.ConnectionInput
	Resource  string `json:"resource"`
	Namespace string `json:"namespace,omitempty"`
	Name      string `json:"name,omitempty"`
}
type logsInput struct {
	client.ConnectionInput
	Namespace     string `json:"namespace"`
	Pod           string `json:"pod,omitempty"`
	ContainerName string `json:"container_name,omitempty"`
	Tail          int64  `json:"tail,omitempty"`
	Timestamps    bool   `json:"timestamps,omitempty"`
}

func clusterInfoTool(ctx context.Context, _ *mcp.CallToolRequest, in baseInput) (*mcp.CallToolResult, any, error) {
	out, err := kt.ClusterInfo(ctx, in.ConnectionInput)
	return nil, out, err
}

func apiResourcesTool(ctx context.Context, _ *mcp.CallToolRequest, in baseInput) (*mcp.CallToolResult, any, error) {
	out, err := kt.APIResources(ctx, in.ConnectionInput)
	return nil, out, err
}

func RegisterTools(s *mcp.Server) {
	add := func(name, description string, extra map[string]any, handler any) {
		switch h := handler.(type) {
		case func(context.Context, *mcp.CallToolRequest, baseInput) (*mcp.CallToolResult, any, error):
			mcp.AddTool(s, &mcp.Tool{Name: name, Description: description, InputSchema: kt.InputSchema(extra)}, h)
		case func(context.Context, *mcp.CallToolRequest, listInput) (*mcp.CallToolResult, any, error):
			mcp.AddTool(s, &mcp.Tool{Name: name, Description: description, InputSchema: kt.InputSchema(extra)}, h)
		case func(context.Context, *mcp.CallToolRequest, getInput) (*mcp.CallToolResult, any, error):
			mcp.AddTool(s, &mcp.Tool{Name: name, Description: description, InputSchema: kt.InputSchema(extra)}, h)
		case func(context.Context, *mcp.CallToolRequest, logsInput) (*mcp.CallToolResult, any, error):
			mcp.AddTool(s, &mcp.Tool{Name: name, Description: description, InputSchema: kt.InputSchema(extra)}, h)
		}
	}
	add("kubernetes_cluster_info", "Read Kubernetes version and connection information.", nil, clusterInfoTool)
	add("kubernetes_api_resources", "List API resources supported by the connected cluster.", nil, apiResourcesTool)
	for _, item := range []struct{ name, description, resource string }{
		{"kubernetes_namespaces", "List Kubernetes namespaces.", "namespaces"}, {"kubernetes_nodes", "List Kubernetes nodes.", "nodes"}, {"kubernetes_pods", "List Kubernetes pods.", "pods"}, {"kubernetes_services", "List Kubernetes services.", "services"}, {"kubernetes_configmaps", "List Kubernetes ConfigMaps.", "configmaps"}, {"kubernetes_ingresses", "List Kubernetes ingresses.", "ingresses"}, {"kubernetes_events", "List Kubernetes events.", "events"},
	} {
		resource := item.resource
		add(item.name, item.description, listExtras(), func(ctx context.Context, _ *mcp.CallToolRequest, in listInput) (*mcp.CallToolResult, any, error) {
			out, err := kt.List(ctx, in.ConnectionInput, resource, in.Namespace, in.Filters, in.Continue, in.Limit)
			return nil, out, err
		})
	}
	add("kubernetes_workloads", "List Kubernetes workloads.", listExtras(), func(ctx context.Context, _ *mcp.CallToolRequest, in listInput) (*mcp.CallToolResult, any, error) {
		out, err := kt.Workloads(ctx, in.ConnectionInput, in.Namespace, in.Filters, in.Continue, in.Limit)
		return nil, out, err
	})
	add("kubernetes_pod_logs", "Read bounded, non-following pod logs.", map[string]any{"namespace": map[string]any{"type": "string"}, "pod": map[string]any{"type": "string"}, "container_name": map[string]any{"type": "string"}, "tail": map[string]any{"type": "integer"}, "timestamps": map[string]any{"type": "boolean"}}, func(ctx context.Context, _ *mcp.CallToolRequest, in logsInput) (*mcp.CallToolResult, any, error) {
		out, err := kt.PodLogs(ctx, in.ConnectionInput, in.Namespace, in.Pod, in.ContainerName, in.Tail, in.Timestamps)
		return nil, out, err
	})
	add("kubernetes_resource_get", "Get an allowlisted Kubernetes resource by name.", map[string]any{"resource": map[string]any{"type": "string"}, "namespace": map[string]any{"type": "string"}, "name": map[string]any{"type": "string"}}, func(ctx context.Context, _ *mcp.CallToolRequest, in getInput) (*mcp.CallToolResult, any, error) {
		out, err := kt.Get(ctx, in.ConnectionInput, in.Resource, in.Namespace, in.Name)
		return nil, out, err
	})
	add("kubernetes_health", "Check Kubernetes API health.", nil, func(ctx context.Context, _ *mcp.CallToolRequest, in baseInput) (*mcp.CallToolResult, any, error) {
		out, err := kt.Health(ctx, in.ConnectionInput)
		return nil, out, err
	})
}

func listExtras() map[string]any {
	return map[string]any{"namespace": map[string]any{"type": "string"}, "filters": map[string]any{"type": "string"}, "limit": map[string]any{"type": "integer"}, "continue": map[string]any{"type": "string"}}
}

// Keep schema output deterministic for consumers that inspect the generated JSON.
func marshalSchema(extra map[string]any) json.RawMessage {
	encoded, _ := json.Marshal(kt.InputSchema(extra))
	return encoded
}
