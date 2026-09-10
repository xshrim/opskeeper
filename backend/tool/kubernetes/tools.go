// Package kubernetes exposes the read-only Kubernetes operations shared by
// Direct resources and the Kubernetes MCP server.
package kubernetes

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	apiresource "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"opskeeper/backend/mcpserver/kubernetes/client"
)

const maxListLimit = 500
const maxLogBytes = 256 * 1024

type ToolInfo struct {
	Name        string
	Description string
}

type ListTool struct {
	ToolInfo
	Resource string
}

var listTools = []ListTool{
	{ToolInfo: ToolInfo{Name: "kubernetes_namespaces", Description: "List Kubernetes namespaces."}, Resource: "namespaces"},
	{ToolInfo: ToolInfo{Name: "kubernetes_nodes", Description: "List Kubernetes nodes."}, Resource: "nodes"},
	{ToolInfo: ToolInfo{Name: "kubernetes_pods", Description: "List Kubernetes pods."}, Resource: "pods"},
	{ToolInfo: ToolInfo{Name: "kubernetes_services", Description: "List Kubernetes services."}, Resource: "services"},
	{ToolInfo: ToolInfo{Name: "kubernetes_configmaps", Description: "List Kubernetes ConfigMaps."}, Resource: "configmaps"},
	{ToolInfo: ToolInfo{Name: "kubernetes_ingresses", Description: "List Kubernetes ingresses."}, Resource: "ingresses"},
	{ToolInfo: ToolInfo{Name: "kubernetes_endpoint_slices", Description: "List Kubernetes EndpointSlices."}, Resource: "endpointslices"},
	{ToolInfo: ToolInfo{Name: "kubernetes_events", Description: "List Kubernetes events."}, Resource: "events"},
}

func ListTools() []ListTool { return append([]ListTool(nil), listTools...) }

func AvailableTools() []ToolInfo {
	items := []ToolInfo{
		{Name: "kubernetes_cluster_info", Description: "Read Kubernetes version and connection information."},
		{Name: "kubernetes_api_resources", Description: "List API resources supported by the connected cluster."},
	}
	for _, item := range listTools {
		items = append(items, item.ToolInfo)
	}
	items = append(items,
		ToolInfo{Name: "kubernetes_workloads", Description: "List Kubernetes workloads."},
		ToolInfo{Name: "kubernetes_pod_stat", Description: "Read current pod CPU and memory usage from the Kubernetes Metrics API."},
		ToolInfo{Name: "kubernetes_node_stat", Description: "Read current node CPU and memory usage from the Kubernetes Metrics API."},
		ToolInfo{Name: "kubernetes_pod_logs", Description: "Read bounded, non-following pod logs."},
		ToolInfo{Name: "kubernetes_resource_get", Description: "Get an allowlisted Kubernetes resource by name."},
		ToolInfo{Name: "kubernetes_health", Description: "Check Kubernetes API health."})
	return items
}

type ResourceItem struct {
	Namespace string            `json:"namespace,omitempty"`
	Name      string            `json:"name"`
	Kind      string            `json:"kind,omitempty"`
	Phase     string            `json:"phase,omitempty"`
	Ready     bool              `json:"ready,omitempty"`
	Restarts  int32             `json:"restarts,omitempty"`
	Node      string            `json:"node,omitempty"`
	Age       string            `json:"age,omitempty"`
	Labels    map[string]string `json:"labels,omitempty"`
}

type ListOutput struct {
	Items     []ResourceItem `json:"items"`
	Count     int            `json:"count"`
	Truncated bool           `json:"truncated"`
	Continue  string         `json:"continue,omitempty"`
}

type LogsOutput struct {
	Namespace string `json:"namespace"`
	Pod       string `json:"pod"`
	Container string `json:"container,omitempty"`
	Logs      string `json:"logs"`
	Truncated bool   `json:"truncated"`
}

type ContainerUsage struct {
	Name  string            `json:"name"`
	Usage map[string]string `json:"usage"`
}

type MetricStat struct {
	Namespace  string            `json:"namespace,omitempty"`
	Name       string            `json:"name"`
	Timestamp  string            `json:"timestamp,omitempty"`
	Window     string            `json:"window,omitempty"`
	Usage      map[string]string `json:"usage"`
	Containers []ContainerUsage  `json:"containers,omitempty"`
}

type StatsOutput struct {
	Items     []MetricStat `json:"items"`
	Count     int          `json:"count"`
	Truncated bool         `json:"truncated"`
	Continue  string       `json:"continue,omitempty"`
}

type resourceDef struct {
	gvr        schema.GroupVersionResource
	namespaced bool
	kind       string
}

var resources = map[string]resourceDef{
	"namespaces":     {gvr: schema.GroupVersionResource{Version: "v1", Resource: "namespaces"}, kind: "Namespace"},
	"nodes":          {gvr: schema.GroupVersionResource{Version: "v1", Resource: "nodes"}, kind: "Node"},
	"pods":           {gvr: schema.GroupVersionResource{Version: "v1", Resource: "pods"}, namespaced: true, kind: "Pod"},
	"configmaps":     {gvr: schema.GroupVersionResource{Version: "v1", Resource: "configmaps"}, namespaced: true, kind: "ConfigMap"},
	"services":       {gvr: schema.GroupVersionResource{Version: "v1", Resource: "services"}, namespaced: true, kind: "Service"},
	"events":         {gvr: schema.GroupVersionResource{Version: "v1", Resource: "events"}, namespaced: true, kind: "Event"},
	"deployments":    {gvr: schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}, namespaced: true, kind: "Deployment"},
	"statefulsets":   {gvr: schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "statefulsets"}, namespaced: true, kind: "StatefulSet"},
	"daemonsets":     {gvr: schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "daemonsets"}, namespaced: true, kind: "DaemonSet"},
	"jobs":           {gvr: schema.GroupVersionResource{Group: "batch", Version: "v1", Resource: "jobs"}, namespaced: true, kind: "Job"},
	"cronjobs":       {gvr: schema.GroupVersionResource{Group: "batch", Version: "v1", Resource: "cronjobs"}, namespaced: true, kind: "CronJob"},
	"ingresses":      {gvr: schema.GroupVersionResource{Group: "networking.k8s.io", Version: "v1", Resource: "ingresses"}, namespaced: true, kind: "Ingress"},
	"endpointslices": {gvr: schema.GroupVersionResource{Group: "discovery.k8s.io", Version: "v1", Resource: "endpointslices"}, namespaced: true, kind: "EndpointSlice"},
}

var metricsResources = map[string]struct {
	gvr        schema.GroupVersionResource
	namespaced bool
	container  bool
}{
	"pods":  {gvr: schema.GroupVersionResource{Group: "metrics.k8s.io", Version: "v1beta1", Resource: "pods"}, namespaced: true, container: true},
	"nodes": {gvr: schema.GroupVersionResource{Group: "metrics.k8s.io", Version: "v1beta1", Resource: "nodes"}},
}

func InputSchema(extra map[string]any) map[string]any {
	p := map[string]any{
		"kubeconfig": map[string]any{"type": "string"}, "connection_mode": map[string]any{"type": "string"},
		"context": map[string]any{"type": "string"}, "profile": map[string]any{"type": "string"},
		"server": map[string]any{"type": "string"}, "ca": map[string]any{"type": "string"}, "token": map[string]any{"type": "string"},
		"client_cert": map[string]any{"type": "string"}, "client_key": map[string]any{"type": "string"}, "skip_tls_verify": map[string]any{"type": "boolean", "default": false},
	}
	for k, v := range extra {
		p[k] = v
	}
	return map[string]any{"type": "object", "properties": p}
}

func ListInputProperties() map[string]any {
	return map[string]any{
		"namespace": map[string]any{"type": "string"},
		"filters":   map[string]any{"type": "string"},
		"limit":     map[string]any{"type": "integer"},
		"continue":  map[string]any{"type": "string"},
	}
}

func PodStatsInputProperties() map[string]any {
	properties := ListInputProperties()
	properties["pod"] = map[string]any{"type": "string"}
	return properties
}

func NodeStatsInputProperties() map[string]any {
	properties := ListInputProperties()
	delete(properties, "namespace")
	properties["node"] = map[string]any{"type": "string"}
	return properties
}

func PodLogsInputProperties() map[string]any {
	return map[string]any{
		"namespace":      map[string]any{"type": "string"},
		"pod":            map[string]any{"type": "string"},
		"container_name": map[string]any{"type": "string"},
		"tail":           map[string]any{"type": "integer"},
		"timestamps":     map[string]any{"type": "boolean"},
	}
}

func GetInputProperties() map[string]any {
	return map[string]any{
		"resource":  map[string]any{"type": "string"},
		"namespace": map[string]any{"type": "string"},
		"name":      map[string]any{"type": "string"},
	}
}

func ClusterInfo(ctx context.Context, input client.ConnectionInput) (map[string]any, error) {
	c, err := client.Open(ctx, input)
	if err != nil {
		return nil, err
	}
	v, err := c.Discovery.ServerVersion()
	if err != nil {
		return nil, err
	}
	return map[string]any{"version": map[string]any{"major": v.Major, "minor": v.Minor, "git_version": v.GitVersion, "platform": v.Platform}, "profile": c.Profile, "server": c.RESTConfig.Host}, nil
}

func APIResources(ctx context.Context, input client.ConnectionInput) (map[string]any, error) {
	c, err := client.Open(ctx, input)
	if err != nil {
		return nil, err
	}
	lists, err := c.Discovery.ServerPreferredResources()
	if err != nil && len(lists) == 0 {
		return nil, err
	}
	out := make([]map[string]any, 0)
	for _, list := range lists {
		for _, item := range list.APIResources {
			if _, ok := resources[strings.ToLower(item.Name)]; ok {
				out = append(out, map[string]any{"group_version": list.GroupVersion, "name": item.Name, "kind": item.Kind, "namespaced": item.Namespaced})
			}
		}
	}
	return map[string]any{"resources": out}, nil
}

func List(ctx context.Context, input client.ConnectionInput, resource, namespace, filters, continuation string, limit int) (ListOutput, error) {
	def, ok := resources[strings.ToLower(strings.TrimSpace(resource))]
	if !ok {
		return ListOutput{}, fmt.Errorf("resource %q is not allowed", resource)
	}
	if limit < 0 || limit > maxListLimit {
		return ListOutput{}, fmt.Errorf("limit must be between 0 and %d", maxListLimit)
	}
	c, err := client.Open(ctx, input)
	if err != nil {
		return ListOutput{}, err
	}
	selector, err := labelsFromString(filters)
	if err != nil {
		return ListOutput{}, err
	}
	opts := metav1.ListOptions{LabelSelector: selector, Limit: int64(limit), Continue: continuation}
	var list *unstructured.UnstructuredList
	if def.namespaced {
		list, err = c.Dynamic.Resource(def.gvr).Namespace(namespace).List(ctx, opts)
	} else {
		list, err = c.Dynamic.Resource(def.gvr).List(ctx, opts)
	}
	if err != nil {
		return ListOutput{}, err
	}
	items := make([]ResourceItem, 0, len(list.Items))
	for _, item := range list.Items {
		items = append(items, toResourceItem(def.kind, item))
	}
	return ListOutput{Items: items, Count: len(items), Truncated: list.GetContinue() != "", Continue: list.GetContinue()}, nil
}

func labelsFromString(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}
	parts := strings.Split(raw, ",")
	labels := make([]string, 0, len(parts))
	for _, item := range parts {
		item = strings.TrimSpace(item)
		if strings.Contains(item, ":") {
			key, value, _ := strings.Cut(item, ":")
			key, value = strings.TrimSpace(key), strings.TrimSpace(value)
			if key == "" || value == "" {
				return "", fmt.Errorf("invalid filter %q: key and value are required", item)
			}
			labels = append(labels, key+"="+value)
		} else if strings.Contains(item, "=") {
			labels = append(labels, item)
		} else {
			return "", fmt.Errorf("invalid filter %q: expected key:value or key=value", item)
		}
	}
	return strings.Join(labels, ","), nil
}

func Workloads(ctx context.Context, input client.ConnectionInput, namespace, filters, continuation string, limit int) (ListOutput, error) {
	var all []ResourceItem
	var next string
	for _, name := range []string{"deployments", "statefulsets", "daemonsets", "jobs", "cronjobs"} {
		out, err := List(ctx, input, name, namespace, filters, continuation, limit)
		if err != nil {
			return ListOutput{}, err
		}
		all, next = append(all, out.Items...), out.Continue
	}
	return ListOutput{Items: all, Count: len(all), Continue: next}, nil
}

func Get(ctx context.Context, input client.ConnectionInput, resource, namespace, name string) (map[string]any, error) {
	def, ok := resources[strings.ToLower(strings.TrimSpace(resource))]
	if !ok {
		return nil, fmt.Errorf("resource %q is not allowed", resource)
	}
	if strings.TrimSpace(name) == "" {
		return nil, fmt.Errorf("name is required")
	}
	c, err := client.Open(ctx, input)
	if err != nil {
		return nil, err
	}
	var item *unstructured.Unstructured
	if def.namespaced {
		item, err = c.Dynamic.Resource(def.gvr).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	} else {
		item, err = c.Dynamic.Resource(def.gvr).Get(ctx, name, metav1.GetOptions{})
	}
	if err != nil {
		return nil, err
	}
	return map[string]any{"item": sanitize(item.Object)}, nil
}

func PodLogs(ctx context.Context, input client.ConnectionInput, namespace, pod, container string, tail int64, timestamps bool) (LogsOutput, error) {
	if strings.TrimSpace(namespace) == "" || strings.TrimSpace(pod) == "" {
		return LogsOutput{}, fmt.Errorf("namespace and pod are required")
	}
	if tail <= 0 {
		tail = 100
	}
	if tail > 10000 {
		tail = 10000
	}
	c, err := client.Open(ctx, input)
	if err != nil {
		return LogsOutput{}, err
	}
	raw, err := c.Kubernetes.CoreV1().Pods(namespace).GetLogs(pod, &corev1.PodLogOptions{Container: container, TailLines: &tail, Timestamps: timestamps}).Stream(ctx)
	if err != nil {
		return LogsOutput{}, err
	}
	defer raw.Close()
	data, err := io.ReadAll(io.LimitReader(raw, maxLogBytes+1))
	if err != nil {
		return LogsOutput{}, err
	}
	truncated := len(data) > maxLogBytes
	if truncated {
		data = data[:maxLogBytes]
	}
	return LogsOutput{Namespace: namespace, Pod: pod, Container: container, Logs: string(data), Truncated: truncated}, nil
}

// PodStats reads current usage from the Kubernetes Metrics API. The API is
// commonly provided by metrics-server and is separate from the core API.
func PodStats(ctx context.Context, input client.ConnectionInput, namespace, pod, filters, continuation string, limit int) (StatsOutput, error) {
	return metricStats(ctx, input, "pods", namespace, pod, filters, continuation, limit)
}

// NodeStats reads current node usage from the Kubernetes Metrics API.
func NodeStats(ctx context.Context, input client.ConnectionInput, node, filters, continuation string, limit int) (StatsOutput, error) {
	return metricStats(ctx, input, "nodes", "", node, filters, continuation, limit)
}

func metricStats(ctx context.Context, input client.ConnectionInput, kind, namespace, name, filters, continuation string, limit int) (StatsOutput, error) {
	def, ok := metricsResources[kind]
	if !ok {
		return StatsOutput{}, fmt.Errorf("metrics resource %q is not allowed", kind)
	}
	if limit < 0 || limit > maxListLimit {
		return StatsOutput{}, fmt.Errorf("limit must be between 0 and %d", maxListLimit)
	}
	selector, err := labelsFromString(filters)
	if err != nil {
		return StatsOutput{}, err
	}
	c, err := client.Open(ctx, input)
	if err != nil {
		return StatsOutput{}, err
	}
	resourceClient := c.Dynamic.Resource(def.gvr)
	var items []unstructured.Unstructured
	var next string
	if strings.TrimSpace(name) != "" {
		var item *unstructured.Unstructured
		if def.namespaced {
			item, err = resourceClient.Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
		} else {
			item, err = resourceClient.Get(ctx, name, metav1.GetOptions{})
		}
		if err != nil {
			return StatsOutput{}, fmt.Errorf("read Kubernetes %s metrics: %w", kind, err)
		}
		items = []unstructured.Unstructured{*item}
	} else {
		opts := metav1.ListOptions{LabelSelector: selector, Limit: int64(limit), Continue: continuation}
		var list *unstructured.UnstructuredList
		if def.namespaced {
			list, err = resourceClient.Namespace(namespace).List(ctx, opts)
		} else {
			list, err = resourceClient.List(ctx, opts)
		}
		if err != nil {
			return StatsOutput{}, fmt.Errorf("read Kubernetes %s metrics: %w", kind, err)
		}
		items, next = list.Items, list.GetContinue()
	}
	out := make([]MetricStat, 0, len(items))
	for _, item := range items {
		out = append(out, metricStat(item, def.container))
	}
	return StatsOutput{Items: out, Count: len(out), Truncated: next != "", Continue: next}, nil
}

func metricStat(item unstructured.Unstructured, withContainers bool) MetricStat {
	stat := MetricStat{
		Namespace: item.GetNamespace(),
		Name:      item.GetName(),
		Usage:     nestedUsage(item.Object, "usage"),
	}
	stat.Timestamp, _, _ = unstructured.NestedString(item.Object, "timestamp")
	stat.Window, _, _ = unstructured.NestedString(item.Object, "window")
	if !withContainers {
		return stat
	}
	containers, _, _ := unstructured.NestedSlice(item.Object, "containers")
	for _, raw := range containers {
		container, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		name, _, _ := unstructured.NestedString(container, "name")
		usage := nestedUsage(container, "usage")
		stat.Containers = append(stat.Containers, ContainerUsage{Name: name, Usage: usage})
		addUsage(stat.Usage, usage)
	}
	return stat
}

func nestedUsage(object map[string]any, field string) map[string]string {
	values, _, _ := unstructured.NestedStringMap(object, field)
	if values == nil {
		return map[string]string{}
	}
	return values
}

func addUsage(total map[string]string, current map[string]string) {
	for name, value := range current {
		quantity, err := apiresource.ParseQuantity(value)
		if err != nil {
			if _, exists := total[name]; !exists {
				total[name] = value
			}
			continue
		}
		if existing, ok := total[name]; ok {
			if previous, parseErr := apiresource.ParseQuantity(existing); parseErr == nil {
				previous.Add(quantity)
				total[name] = previous.String()
				continue
			}
		}
		total[name] = quantity.String()
	}
}

func Health(ctx context.Context, input client.ConnectionInput) (map[string]any, error) {
	_, err := ClusterInfo(ctx, input)
	if err != nil {
		return map[string]any{"healthy": false, "checks": map[string]string{"api": err.Error()}}, nil
	}
	return map[string]any{"healthy": true, "checks": map[string]string{"api": "ok"}}, nil
}

func toResourceItem(kind string, o unstructured.Unstructured) ResourceItem {
	item := ResourceItem{Namespace: o.GetNamespace(), Name: o.GetName(), Kind: kind, Labels: o.GetLabels(), Age: o.GetCreationTimestamp().Time.Format(time.RFC3339)}
	item.Phase, _, _ = unstructured.NestedString(o.Object, "status", "phase")
	item.Node, _, _ = unstructured.NestedString(o.Object, "spec", "nodeName")
	if kind == "Pod" {
		conditions, _, _ := unstructured.NestedSlice(o.Object, "status", "conditions")
		for _, raw := range conditions {
			value, _ := raw.(map[string]any)
			if value["type"] == "Ready" && value["status"] == "True" {
				item.Ready = true
			}
		}
		statuses, _, _ := unstructured.NestedSlice(o.Object, "status", "containerStatuses")
		for _, raw := range statuses {
			value, _ := raw.(map[string]any)
			if count, ok := value["restartCount"].(int64); ok {
				item.Restarts += int32(count)
			}
		}
	}
	return item
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
