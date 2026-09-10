package server

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"opskeeper/backend/mcpserver/kubernetes/client"
	kt "opskeeper/backend/tool/kubernetes"
)

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
type podStatInput struct {
	client.ConnectionInput
	Namespace string `json:"namespace,omitempty"`
	Pod       string `json:"pod,omitempty"`
	Filters   string `json:"filters,omitempty"`
	Limit     int    `json:"limit,omitempty"`
	Continue  string `json:"continue,omitempty"`
}
type nodeStatInput struct {
	client.ConnectionInput
	Node     string `json:"node,omitempty"`
	Filters  string `json:"filters,omitempty"`
	Limit    int    `json:"limit,omitempty"`
	Continue string `json:"continue,omitempty"`
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
		case func(context.Context, *mcp.CallToolRequest, podStatInput) (*mcp.CallToolResult, any, error):
			mcp.AddTool(s, &mcp.Tool{Name: name, Description: description, InputSchema: kt.InputSchema(extra)}, h)
		case func(context.Context, *mcp.CallToolRequest, nodeStatInput) (*mcp.CallToolResult, any, error):
			mcp.AddTool(s, &mcp.Tool{Name: name, Description: description, InputSchema: kt.InputSchema(extra)}, h)
		}
	}
	add("kubernetes_cluster_info", "Read Kubernetes version and connection information.", nil, clusterInfoTool)
	add("kubernetes_api_resources", "List API resources supported by the connected cluster.", nil, apiResourcesTool)
	for _, tool := range kt.ListTools() {
		item := tool
		add(item.Name, item.Description, kt.ListInputProperties(), func(ctx context.Context, _ *mcp.CallToolRequest, in listInput) (*mcp.CallToolResult, any, error) {
			out, err := kt.List(ctx, in.ConnectionInput, item.Resource, in.Namespace, in.Filters, in.Continue, in.Limit)
			return nil, out, err
		})
	}
	add("kubernetes_workloads", "List Kubernetes workloads.", kt.ListInputProperties(), func(ctx context.Context, _ *mcp.CallToolRequest, in listInput) (*mcp.CallToolResult, any, error) {
		out, err := kt.Workloads(ctx, in.ConnectionInput, in.Namespace, in.Filters, in.Continue, in.Limit)
		return nil, out, err
	})
	add("kubernetes_pod_stat", "Read current pod CPU and memory usage from the Kubernetes Metrics API.", kt.PodStatsInputProperties(), func(ctx context.Context, _ *mcp.CallToolRequest, in podStatInput) (*mcp.CallToolResult, any, error) {
		out, err := kt.PodStats(ctx, in.ConnectionInput, in.Namespace, in.Pod, in.Filters, in.Continue, in.Limit)
		return nil, out, err
	})
	add("kubernetes_node_stat", "Read current node CPU and memory usage from the Kubernetes Metrics API.", kt.NodeStatsInputProperties(), func(ctx context.Context, _ *mcp.CallToolRequest, in nodeStatInput) (*mcp.CallToolResult, any, error) {
		out, err := kt.NodeStats(ctx, in.ConnectionInput, in.Node, in.Filters, in.Continue, in.Limit)
		return nil, out, err
	})
	add("kubernetes_pod_logs", "Read bounded, non-following pod logs.", kt.PodLogsInputProperties(), func(ctx context.Context, _ *mcp.CallToolRequest, in logsInput) (*mcp.CallToolResult, any, error) {
		out, err := kt.PodLogs(ctx, in.ConnectionInput, in.Namespace, in.Pod, in.ContainerName, in.Tail, in.Timestamps)
		return nil, out, err
	})
	add("kubernetes_resource_get", "Get an allowlisted Kubernetes resource by name.", kt.GetInputProperties(), func(ctx context.Context, _ *mcp.CallToolRequest, in getInput) (*mcp.CallToolResult, any, error) {
		out, err := kt.Get(ctx, in.ConnectionInput, in.Resource, in.Namespace, in.Name)
		return nil, out, err
	})
	add("kubernetes_health", "Check Kubernetes API health.", nil, func(ctx context.Context, _ *mcp.CallToolRequest, in baseInput) (*mcp.CallToolResult, any, error) {
		out, err := kt.Health(ctx, in.ConnectionInput)
		return nil, out, err
	})
}
