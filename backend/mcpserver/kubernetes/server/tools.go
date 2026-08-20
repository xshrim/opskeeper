package server

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"opskeeper/backend/mcpserver/kubernetes/client"
)

const maxListLimit = 500
const maxLogBytes = 256 * 1024

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

type resourceDef struct {
	GVR        schema.GroupVersionResource
	Namespaced bool
	Kind       string
}

var resources = map[string]resourceDef{
	"namespaces":     {GVR: schema.GroupVersionResource{Version: "v1", Resource: "namespaces"}, Kind: "Namespace"},
	"nodes":          {GVR: schema.GroupVersionResource{Version: "v1", Resource: "nodes"}, Kind: "Node"},
	"pods":           {GVR: schema.GroupVersionResource{Version: "v1", Resource: "pods"}, Namespaced: true, Kind: "Pod"},
	"configmaps":     {GVR: schema.GroupVersionResource{Version: "v1", Resource: "configmaps"}, Namespaced: true, Kind: "ConfigMap"},
	"services":       {GVR: schema.GroupVersionResource{Version: "v1", Resource: "services"}, Namespaced: true, Kind: "Service"},
	"events":         {GVR: schema.GroupVersionResource{Version: "v1", Resource: "events"}, Namespaced: true, Kind: "Event"},
	"deployments":    {GVR: schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}, Namespaced: true, Kind: "Deployment"},
	"statefulsets":   {GVR: schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "statefulsets"}, Namespaced: true, Kind: "StatefulSet"},
	"daemonsets":     {GVR: schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "daemonsets"}, Namespaced: true, Kind: "DaemonSet"},
	"jobs":           {GVR: schema.GroupVersionResource{Group: "batch", Version: "v1", Resource: "jobs"}, Namespaced: true, Kind: "Job"},
	"cronjobs":       {GVR: schema.GroupVersionResource{Group: "batch", Version: "v1", Resource: "cronjobs"}, Namespaced: true, Kind: "CronJob"},
	"ingresses":      {GVR: schema.GroupVersionResource{Group: "networking.k8s.io", Version: "v1", Resource: "ingresses"}, Namespaced: true, Kind: "Ingress"},
	"endpointslices": {GVR: schema.GroupVersionResource{Group: "discovery.k8s.io", Version: "v1", Resource: "endpointslices"}, Namespaced: true, Kind: "EndpointSlice"},
}

func RegisterTools(s *mcp.Server) {
	add := func(name, description string, extra map[string]any, handler any) {
		// Every output is registered as any: the SDK consequently omits outputSchema.
		switch h := handler.(type) {
		case func(context.Context, *mcp.CallToolRequest, baseInput) (*mcp.CallToolResult, any, error):
			mcp.AddTool(s, &mcp.Tool{Name: name, Description: description, InputSchema: inputSchema(extra)}, h)
		case func(context.Context, *mcp.CallToolRequest, listInput) (*mcp.CallToolResult, any, error):
			mcp.AddTool(s, &mcp.Tool{Name: name, Description: description, InputSchema: inputSchema(extra)}, h)
		case func(context.Context, *mcp.CallToolRequest, getInput) (*mcp.CallToolResult, any, error):
			mcp.AddTool(s, &mcp.Tool{Name: name, Description: description, InputSchema: inputSchema(extra)}, h)
		case func(context.Context, *mcp.CallToolRequest, logsInput) (*mcp.CallToolResult, any, error):
			mcp.AddTool(s, &mcp.Tool{Name: name, Description: description, InputSchema: inputSchema(extra)}, h)
		}
	}
	add("kubernetes_cluster_info", "Read Kubernetes version and connection information.", nil, clusterInfoTool)
	add("kubernetes_api_resources", "List API resources supported by the connected cluster.", nil, apiResourcesTool)
	add("kubernetes_namespaces", "List Kubernetes namespaces.", listExtras(), namespacesTool)
	add("kubernetes_nodes", "List Kubernetes nodes.", listExtras(), nodesTool)
	add("kubernetes_pods", "List Kubernetes pods.", listExtras(), podsTool)
	add("kubernetes_workloads", "List deployments, statefulsets, daemonsets, jobs, and cronjobs.", listExtras(), workloadsTool)
	add("kubernetes_services", "List Kubernetes services.", listExtras(), servicesTool)
	add("kubernetes_configmaps", "List Kubernetes ConfigMaps.", listExtras(), configMapsTool)
	add("kubernetes_ingresses", "List Kubernetes ingresses.", listExtras(), ingressesTool)
	add("kubernetes_events", "List Kubernetes events.", listExtras(), eventsTool)
	add("kubernetes_pod_logs", "Read bounded, non-following pod logs.", map[string]any{"namespace": map[string]any{"type": "string"}, "pod": map[string]any{"type": "string"}, "container_name": map[string]any{"type": "string"}, "tail": map[string]any{"type": "integer"}, "timestamps": map[string]any{"type": "boolean"}}, logsTool)
	add("kubernetes_resource_get", "Get an allowlisted Kubernetes resource by name.", map[string]any{"resource": map[string]any{"type": "string"}, "namespace": map[string]any{"type": "string"}, "name": map[string]any{"type": "string"}}, resourceGetTool)
	add("kubernetes_health", "Check Kubernetes API health.", nil, healthTool)
}

func listExtras() map[string]any {
	return map[string]any{"namespace": map[string]any{"type": "string"}, "filters": map[string]any{"type": "string"}, "limit": map[string]any{"type": "integer"}, "continue": map[string]any{"type": "string"}}
}
func inputSchema(extra map[string]any) map[string]any {
	p := map[string]any{"kubeconfig_base64": map[string]any{"type": "string"}, "connection_mode": map[string]any{"type": "string"}, "kubeconfig_path": map[string]any{"type": "string"}, "context": map[string]any{"type": "string"}, "profile": map[string]any{"type": "string"}, "server": map[string]any{"type": "string"}, "ca_file": map[string]any{"type": "string"}, "token_file": map[string]any{"type": "string"}, "client_cert_file": map[string]any{"type": "string"}, "client_key_file": map[string]any{"type": "string"}, "skip_tls_verify": map[string]any{"type": "boolean", "default": false}}
	for k, v := range extra {
		p[k] = v
	}
	return map[string]any{"type": "object", "properties": p}
}

func clusterInfoTool(ctx context.Context, _ *mcp.CallToolRequest, in baseInput) (*mcp.CallToolResult, any, error) {
	c, err := client.Open(ctx, in.ConnectionInput)
	if err != nil {
		return nil, nil, err
	}
	v, err := c.Discovery.ServerVersion()
	if err != nil {
		return nil, nil, err
	}
	return nil, ClusterInfoOutput{Version: map[string]any{"major": v.Major, "minor": v.Minor, "git_version": v.GitVersion, "platform": v.Platform}, Profile: c.Profile, Server: c.RESTConfig.Host}, nil
}
func apiResourcesTool(ctx context.Context, _ *mcp.CallToolRequest, in baseInput) (*mcp.CallToolResult, any, error) {
	c, err := client.Open(ctx, in.ConnectionInput)
	if err != nil {
		return nil, nil, err
	}
	lists, err := c.Discovery.ServerPreferredResources()
	if err != nil && len(lists) == 0 {
		return nil, nil, err
	}
	out := make([]map[string]any, 0)
	for _, list := range lists {
		for _, r := range list.APIResources {
			if _, ok := resources[strings.ToLower(r.Name)]; ok {
				out = append(out, map[string]any{"group_version": list.GroupVersion, "name": r.Name, "kind": r.Kind, "namespaced": r.Namespaced})
			}
		}
	}
	return nil, APIResourcesOutput{Resources: out}, nil
}
func namespacesTool(ctx context.Context, r *mcp.CallToolRequest, in listInput) (*mcp.CallToolResult, any, error) {
	return listResource(ctx, r, in, "namespaces")
}
func nodesTool(ctx context.Context, r *mcp.CallToolRequest, in listInput) (*mcp.CallToolResult, any, error) {
	return listResource(ctx, r, in, "nodes")
}
func podsTool(ctx context.Context, r *mcp.CallToolRequest, in listInput) (*mcp.CallToolResult, any, error) {
	return listResource(ctx, r, in, "pods")
}
func servicesTool(ctx context.Context, r *mcp.CallToolRequest, in listInput) (*mcp.CallToolResult, any, error) {
	return listResource(ctx, r, in, "services")
}
func configMapsTool(ctx context.Context, r *mcp.CallToolRequest, in listInput) (*mcp.CallToolResult, any, error) {
	return listResource(ctx, r, in, "configmaps")
}
func ingressesTool(ctx context.Context, r *mcp.CallToolRequest, in listInput) (*mcp.CallToolResult, any, error) {
	return listResource(ctx, r, in, "ingresses")
}
func eventsTool(ctx context.Context, r *mcp.CallToolRequest, in listInput) (*mcp.CallToolResult, any, error) {
	return listResource(ctx, r, in, "events")
}
func workloadsTool(ctx context.Context, r *mcp.CallToolRequest, in listInput) (*mcp.CallToolResult, any, error) {
	var all []ResourceItem
	var continuation string
	for _, name := range []string{"deployments", "statefulsets", "daemonsets", "jobs", "cronjobs"} {
		_, out, err := listResource(ctx, r, in, name)
		if err != nil {
			return nil, nil, err
		}
		value := out.(ListOutput)
		all = append(all, value.Items...)
		continuation = value.Continue
	}
	return nil, ListOutput{Items: all, Count: len(all), Continue: continuation}, nil
}
func listResource(ctx context.Context, _ *mcp.CallToolRequest, in listInput, name string) (*mcp.CallToolResult, any, error) {
	def := resources[name]
	opts, err := listOptions(in.Filters, in.Limit, in.Continue)
	if err != nil {
		return nil, nil, err
	}
	c, err := client.Open(ctx, in.ConnectionInput)
	if err != nil {
		return nil, nil, err
	}
	var ri interface {
		List(context.Context, metav1.ListOptions) (*unstructured.UnstructuredList, error)
	}
	if def.Namespaced {
		ri = c.Dynamic.Resource(def.GVR).Namespace(in.Namespace)
	} else {
		ri = c.Dynamic.Resource(def.GVR)
	}
	list, err := ri.List(ctx, opts)
	if err != nil {
		return nil, nil, err
	}
	items := make([]ResourceItem, 0, len(list.Items))
	for _, item := range list.Items {
		items = append(items, toResourceItem(def.Kind, item))
	}
	return nil, ListOutput{Items: items, Count: len(items), Truncated: list.GetContinue() != "", Continue: list.GetContinue()}, nil
}

func resourceGetTool(ctx context.Context, _ *mcp.CallToolRequest, in getInput) (*mcp.CallToolResult, any, error) {
	def, ok := resources[strings.ToLower(strings.TrimSpace(in.Resource))]
	if !ok {
		return nil, nil, fmt.Errorf("resource %q is not allowed", in.Resource)
	}
	if strings.TrimSpace(in.Name) == "" {
		return nil, nil, fmt.Errorf("name is required")
	}
	c, err := client.Open(ctx, in.ConnectionInput)
	if err != nil {
		return nil, nil, err
	}
	var ri interface {
		Get(context.Context, string, metav1.GetOptions, ...string) (*unstructured.Unstructured, error)
	}
	if def.Namespaced {
		ri = c.Dynamic.Resource(def.GVR).Namespace(in.Namespace)
	} else {
		ri = c.Dynamic.Resource(def.GVR)
	}
	item, err := ri.Get(ctx, in.Name, metav1.GetOptions{})
	if err != nil {
		return nil, nil, err
	}
	return nil, DetailOutput{Item: sanitize(item.Object)}, nil
}
func logsTool(ctx context.Context, _ *mcp.CallToolRequest, in logsInput) (*mcp.CallToolResult, any, error) {
	if strings.TrimSpace(in.Namespace) == "" || strings.TrimSpace(in.Pod) == "" {
		return nil, nil, fmt.Errorf("namespace and pod are required")
	}
	c, err := client.Open(ctx, in.ConnectionInput)
	if err != nil {
		return nil, nil, err
	}
	tail := in.Tail
	if tail <= 0 {
		tail = 100
	}
	if tail > 10000 {
		tail = 10000
	}
	opts := corev1.PodLogOptions{Container: in.ContainerName, TailLines: &tail, Timestamps: in.Timestamps}
	req := c.Kubernetes.CoreV1().Pods(in.Namespace).GetLogs(in.Pod, &opts)
	raw, err := req.Stream(ctx)
	if err != nil {
		return nil, nil, err
	}
	defer raw.Close()
	data, err := io.ReadAll(io.LimitReader(raw, maxLogBytes+1))
	if err != nil {
		return nil, nil, err
	}
	tr := len(data) > maxLogBytes
	if tr {
		data = data[:maxLogBytes]
	}
	return nil, LogsOutput{Namespace: in.Namespace, Pod: in.Pod, Container: in.ContainerName, Logs: string(data), Truncated: tr}, nil
}
func healthTool(ctx context.Context, _ *mcp.CallToolRequest, in baseInput) (*mcp.CallToolResult, any, error) {
	c, err := client.Open(ctx, in.ConnectionInput)
	if err != nil {
		return nil, nil, err
	}
	_, err = c.Discovery.ServerVersion()
	checks := map[string]string{"api": "ok"}
	if err != nil {
		checks["api"] = err.Error()
	}
	return nil, HealthOutput{Healthy: err == nil, Checks: checks}, nil
}

func toResourceItem(kind string, o unstructured.Unstructured) ResourceItem {
	item := ResourceItem{Namespace: o.GetNamespace(), Name: o.GetName(), Kind: kind, Labels: o.GetLabels(), Age: o.GetCreationTimestamp().Time.Format(time.RFC3339)}
	item.Phase, _ = nestedString(o.Object, "status", "phase")
	item.Node, _ = nestedString(o.Object, "spec", "nodeName")
	if kind == "Pod" {
		item.Ready = podReady(o.Object)
		item.Restarts = podRestarts(o.Object)
	}
	return item
}
func nestedString(obj map[string]any, fields ...string) (string, bool) {
	v, ok, _ := unstructured.NestedString(obj, fields...)
	return v, ok
}
func podReady(obj map[string]any) bool {
	conditions, _, _ := unstructured.NestedSlice(obj, "status", "conditions")
	for _, raw := range conditions {
		m, _ := raw.(map[string]any)
		if m["type"] == "Ready" && m["status"] == "True" {
			return true
		}
	}
	return false
}
func podRestarts(obj map[string]any) int32 {
	containers, _, _ := unstructured.NestedSlice(obj, "status", "containerStatuses")
	var n int32
	for _, raw := range containers {
		m, _ := raw.(map[string]any)
		if v, ok := m["restartCount"].(int64); ok {
			n += int32(v)
		}
	}
	return n
}
func sanitize(obj map[string]any) map[string]any {
	out := map[string]any{}
	for _, k := range []string{"apiVersion", "kind", "metadata", "data", "binaryData"} {
		if v, ok := obj[k]; ok {
			out[k] = v
		}
	}
	if metadata, ok := out["metadata"].(map[string]any); ok {
		delete(metadata, "managedFields")
		delete(metadata, "annotations")
	}
	return out
}
