package connector

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"opskeeper/backend/aiengine"
	"opskeeper/backend/resource"
	kt "opskeeper/backend/tool/kubernetes"
)

var applicationToolSchema = json.RawMessage(`{"type":"object","properties":{"instance_index":{"type":"integer","minimum":0},"tail":{"type":"integer","minimum":1,"maximum":1000},"timestamps":{"type":"boolean"}},"additionalProperties":false}`)

const (
	maxApplicationLogOutputs = 40
	maxApplicationLogBytes   = 4 << 20
)

func (s *Service) resolveApplicationTools(ctx context.Context, item aiengine.ContextResource, add func(string, string, json.RawMessage, func(context.Context, map[string]any) (aiengine.ToolResult, error))) error {
	if item.Status != resource.StatusActive {
		return fmt.Errorf("Application resource is not active")
	}
	add("application_instances", "Read the configured Application instances and validate their current targets.", emptySchema, func(runCtx context.Context, _ map[string]any) (aiengine.ToolResult, error) {
		if err := s.testApplication(runCtx, resource.Resource{ID: item.ID, Kind: item.Kind, Status: item.Status, Config: item.Config}); err != nil {
			return aiengine.ToolResult{}, err
		}
		return aiengine.ToolResult{Output: map[string]any{"access_mode": item.Config["access_mode"], "instances": item.Config["instances"]}, Untrusted: true}, nil
	})
	add("application_logs", "Read bounded logs for one configured Application instance.", applicationToolSchema, func(runCtx context.Context, args map[string]any) (aiengine.ToolResult, error) {
		output, err := s.applicationLogs(runCtx, item, int(int64Arg(args, "instance_index")), int(int64Arg(args, "tail")), boolArg(args, "timestamps", false))
		if err != nil {
			return aiengine.ToolResult{}, err
		}
		return aiengine.ToolResult{Output: output, Untrusted: true}, nil
	})
	return nil
}

func (s *Service) applicationLogs(ctx context.Context, item aiengine.ContextResource, index, tail int, timestamps bool) (any, error) {
	if tail <= 0 {
		tail = 100
	}
	if tail > 1000 {
		tail = 1000
	}
	raw, ok := item.Config["instances"].([]any)
	if !ok || index < 0 || index >= len(raw) {
		return nil, fmt.Errorf("instance_index must identify a configured instance")
	}
	instance, ok := raw[index].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("Application instance is invalid")
	}
	sourceID, _ := applicationSource(instance, strings.ToLower(fmt.Sprint(item.Config["access_mode"])))
	source, err := s.resources.Get(ctx, sourceID)
	if err != nil {
		return nil, err
	}
	logSource, _ := instance["log_source"].(map[string]any)
	if logSource != nil && strings.EqualFold(applicationStringValue(logSource["type"]), "query") {
		resourceID := strings.TrimSpace(applicationStringValue(logSource["resource_id"]))
		query := strings.TrimSpace(applicationStringValue(logSource["query"]))
		if resourceID == "" {
			return nil, fmt.Errorf("log_source.resource_id is required for query logs")
		}
		logResource, err := s.resources.Get(ctx, resourceID)
		if err != nil {
			return nil, fmt.Errorf("log source resource: %w", err)
		}
		if logResource.Kind != "Loki" || logResource.Status != resource.StatusActive {
			return nil, fmt.Errorf("log source resource must be an active Loki resource")
		}
		end := time.Now().UTC()
		result, err := s.invokeApplicationSource(ctx, logResource, "query_logs", map[string]any{
			"query": query, "start": end.Add(-time.Hour).Format(time.RFC3339), "end": end.Format(time.RFC3339), "limit": max(1, tail),
		})
		return result.Output, err
	}
	path := applicationStringValue(logSource["path"])
	mode := strings.ToLower(fmt.Sprint(item.Config["access_mode"]))
	switch mode {
	case "virtual_machine":
		if path == "" {
			return nil, fmt.Errorf("Host Application log_source.path is required")
		}
		result, err := s.invokeApplicationSource(ctx, source, "host_file_logs", map[string]any{"path": path, "tail": fmt.Sprint(tail), "timestamps": timestamps})
		return result.Output, err
	case "containerized":
		name := applicationStringValue(instance["container_name"])
		if path != "" {
			result, err := s.invokeApplicationSource(ctx, source, "docker_container_file", map[string]any{"container_name": name, "path": path})
			return result.Output, err
		}
		result, err := s.invokeApplicationSource(ctx, source, "docker_container_logs", map[string]any{"container_name": name, "tail": fmt.Sprint(tail), "timestamps": timestamps})
		return result.Output, err
	case "cloud_native":
		return s.applicationKubernetesLogs(ctx, source, instance, path, tail, timestamps)
	default:
		return nil, fmt.Errorf("unsupported Application access mode %q", mode)
	}
	return nil, fmt.Errorf("unsupported Application access mode %q", mode)
}

func (s *Service) applicationKubernetesLogs(ctx context.Context, source resource.Resource, instance map[string]any, path string, tail int, timestamps bool) (any, error) {
	kind := applicationStringValue(instance["workload_kind"])
	namespace := applicationStringValue(instance["namespace"])
	name := applicationStringValue(instance["workload_name"])
	toolResult, err := s.invokeApplicationSource(ctx, source, "kubernetes_workload_pods", map[string]any{"workload_kind": kind, "namespace": namespace, "workload_name": name, "limit": 20})
	if err != nil {
		return nil, err
	}
	workload, err := decodeApplicationTool[kt.WorkloadPodsOutput](toolResult)
	if err != nil {
		return nil, err
	}
	outputs := make([]any, 0, len(workload.Pods))
	errorsSeen := make([]string, 0)
	partial := workload.Truncated
	outputBytes := 0
	stop := false
	for _, pod := range workload.Pods {
		containers := pod.Containers
		if len(containers) == 0 {
			containers = []string{""}
		}
		for _, container := range containers {
			if len(outputs) >= maxApplicationLogOutputs {
				partial = true
				stop = true
				break
			}
			var output any
			var callErr error
			if path == "" {
				toolResult, err := s.invokeApplicationSource(ctx, source, "kubernetes_pod_logs", map[string]any{"namespace": pod.Namespace, "pod": pod.Name, "container_name": container, "tail": tail, "timestamps": timestamps})
				output, callErr = toolResult.Output, err
			} else {
				toolResult, err := s.invokeApplicationSource(ctx, source, "kubernetes_pod_file", map[string]any{"namespace": pod.Namespace, "pod": pod.Name, "container_name": container, "path": path})
				output, callErr = toolResult.Output, err
			}
			if callErr != nil {
				partial = true
				errorsSeen = append(errorsSeen, fmt.Sprintf("%s/%s/%s: %v", pod.Namespace, pod.Name, container, callErr))
				continue
			}
			encoded, _ := json.Marshal(output)
			if outputBytes+len(encoded) > maxApplicationLogBytes {
				partial = true
				stop = true
				break
			}
			outputBytes += len(encoded)
			outputs = append(outputs, output)
		}
		if stop {
			break
		}
	}
	result := map[string]any{
		"namespace":     namespace,
		"workload_kind": kind,
		"workload_name": name,
		"pods":          workload.Pods,
		"outputs":       outputs,
		"count":         len(outputs),
		"partial":       partial,
	}
	if len(errorsSeen) > 0 {
		result["errors"] = errorsSeen
	}
	if len(outputs) == 0 && len(errorsSeen) > 0 {
		return nil, fmt.Errorf("read Kubernetes Application logs: %s", strings.Join(errorsSeen, "; "))
	}
	return result, nil
}

func applicationStringValue(value any) string {
	text, _ := value.(string)
	return strings.TrimSpace(text)
}
