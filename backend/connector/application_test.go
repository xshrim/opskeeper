package connector

import (
	"context"
	"fmt"
	"testing"

	"opskeeper/backend/aiengine"
	"opskeeper/backend/resource"
	dockertool "opskeeper/backend/tool/docker"
	hosttool "opskeeper/backend/tool/host"
	kt "opskeeper/backend/tool/kubernetes"
)

type applicationResourceReader struct{ items map[string]resource.Resource }

func (r applicationResourceReader) Get(_ context.Context, id string) (resource.Resource, error) {
	item, ok := r.items[id]
	if !ok {
		return resource.Resource{}, resource.ErrNotFound
	}
	return item, nil
}

type applicationToolCall struct {
	resourceID string
	tool       string
	arguments  map[string]any
}

type applicationToolInvoker struct{ calls []applicationToolCall }

func (i *applicationToolInvoker) InvokeFixed(_ context.Context, source aiengine.ContextResource, tool string, arguments map[string]any) (aiengine.ToolResult, error) {
	i.calls = append(i.calls, applicationToolCall{resourceID: source.ID, tool: tool, arguments: arguments})
	switch tool {
	case "host_processes":
		return aiengine.ToolResult{Output: hosttool.ProcessesOutput{MatchCount: 1, Processes: []hosttool.ProcessInfo{{PID: 42, Name: "orders"}}}}, nil
	case "docker_containers":
		return aiengine.ToolResult{Output: dockertool.ContainersOutput{Containers: []dockertool.ContainerDTO{{ID: "container-1", Names: []string{"/orders"}}}}}, nil
	case "kubernetes_workload_pods":
		return aiengine.ToolResult{Output: kt.WorkloadPodsOutput{Count: 1, Pods: []kt.PodTarget{{Namespace: "orders", Name: "orders-1", Containers: []string{"app"}}}}}, nil
	case "host_file_logs", "docker_container_logs", "docker_container_file", "kubernetes_pod_logs", "kubernetes_pod_file", "query_logs":
		return aiengine.ToolResult{Output: map[string]any{"ok": true}}, nil
	default:
		return aiengine.ToolResult{}, fmt.Errorf("unexpected tool %q", tool)
	}
}

func TestApplicationUsesSameFixedToolsForDirectAndAgentSources(t *testing.T) {
	for _, subtype := range []string{resource.AccessModeDirect, resource.AccessModeAgent} {
		t.Run(subtype, func(t *testing.T) {
			resources := applicationResourceReader{items: map[string]resource.Resource{
				"host":   {ID: "host", Kind: "Host", Subtype: subtype, Status: resource.StatusActive},
				"docker": {ID: "docker", Kind: "Docker", Subtype: subtype, Status: resource.StatusActive},
				"kube":   {ID: "kube", Kind: "Kubernetes", Subtype: subtype, Status: resource.StatusActive},
			}}
			invoker := &applicationToolInvoker{}
			service := NewService(NewRegistry(), resources, nil, nil, DefaultLimits())
			service.SetApplicationToolInvoker(invoker)
			cases := []struct {
				mode, sourceID, wantTool string
				instance                 map[string]any
			}{
				{mode: "virtual_machine", sourceID: "host", wantTool: "host_processes", instance: map[string]any{"host_resource_id": "host", "process_keyword": "orders", "log_source": map[string]any{"type": "path", "path": "/var/log/orders.log"}}},
				{mode: "containerized", sourceID: "docker", wantTool: "docker_containers", instance: map[string]any{"docker_resource_id": "docker", "container_name": "orders", "log_source": map[string]any{"type": "stdout"}}},
				{mode: "cloud_native", sourceID: "kube", wantTool: "kubernetes_workload_pods", instance: map[string]any{"kubernetes_resource_id": "kube", "namespace": "orders", "workload_kind": "Deployment", "workload_name": "orders", "log_source": map[string]any{"type": "stdout"}}},
			}
			for _, test := range cases {
				invoker.calls = nil
				err := service.testApplication(context.Background(), resource.Resource{ID: "application", Kind: "Application", Status: resource.StatusActive, Config: map[string]any{"access_mode": test.mode, "instances": []any{test.instance}}})
				if err != nil {
					t.Fatalf("%s validation error = %v", test.mode, err)
				}
				if len(invoker.calls) != 1 || invoker.calls[0].resourceID != test.sourceID || invoker.calls[0].tool != test.wantTool {
					t.Fatalf("%s calls = %#v", test.mode, invoker.calls)
				}
			}
		})
	}
}

func TestApplicationUsesAgentLokiThroughTheSameFixedQueryTool(t *testing.T) {
	resources := applicationResourceReader{items: map[string]resource.Resource{
		"host": {ID: "host", Kind: "Host", Subtype: resource.AccessModeAgent, Status: resource.StatusActive},
		"loki": {ID: "loki", Kind: "Loki", Subtype: resource.AccessModeAgent, Status: resource.StatusActive},
	}}
	invoker := &applicationToolInvoker{}
	service := NewService(NewRegistry(), resources, nil, nil, DefaultLimits())
	service.SetApplicationToolInvoker(invoker)
	_, err := service.applicationLogs(context.Background(), aiengine.ContextResource{ID: "application", Config: map[string]any{
		"access_mode": "virtual_machine",
		"instances":   []any{map[string]any{"host_resource_id": "host", "process_keyword": "orders", "log_source": map[string]any{"type": "query", "resource_id": "loki", "query": "{app=\"orders\"}"}}},
	}}, 0, 10, false)
	if err != nil || len(invoker.calls) != 1 || invoker.calls[0].resourceID != "loki" || invoker.calls[0].tool != "query_logs" {
		t.Fatalf("result error=%v calls=%#v", err, invoker.calls)
	}
}
