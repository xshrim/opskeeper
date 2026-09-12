package connector

import (
	"context"
	"fmt"
	"strings"

	"opskeeper/backend/resource"
	dockertool "opskeeper/backend/tool/docker"
	hosttool "opskeeper/backend/tool/host"
	kt "opskeeper/backend/tool/kubernetes"
)

func (s *Service) testApplication(ctx context.Context, item resource.Resource) error {
	mode, _ := item.Config["access_mode"].(string)
	mode = strings.ToLower(strings.TrimSpace(mode))
	raw, ok := item.Config["instances"].([]any)
	if !ok || len(raw) == 0 {
		return connectorError(CategoryConfiguration, "validate Application instances", false, fmt.Errorf("Application instances are required"))
	}
	for index, value := range raw {
		instance, ok := value.(map[string]any)
		if !ok {
			return connectorError(CategoryConfiguration, "validate Application instance", false, fmt.Errorf("instance %d is invalid", index))
		}
		id, kind := applicationSource(instance, mode)
		if id == "" {
			return connectorError(CategoryConfiguration, "validate Application instance", false, fmt.Errorf("instance %d source resource is missing", index))
		}
		source, err := s.resources.Get(ctx, id)
		if err != nil {
			return fmt.Errorf("Application instance %d source resource: %w", index, err)
		}
		if source.Kind != kind || source.Status != resource.StatusActive {
			return fmt.Errorf("Application instance %d source resource is unavailable", index)
		}
		switch mode {
		case "virtual_machine":
			if err := s.validateHostInstance(ctx, source, instance); err != nil {
				return fmt.Errorf("Application instance %d: %w", index, err)
			}
		case "containerized":
			if err := s.validateDockerInstance(ctx, source, instance); err != nil {
				return fmt.Errorf("Application instance %d: %w", index, err)
			}
		case "cloud_native":
			if err := s.validateKubernetesInstance(ctx, source, instance); err != nil {
				return fmt.Errorf("Application instance %d: %w", index, err)
			}
		default:
			return connectorError(CategoryConfiguration, "validate Application", false, fmt.Errorf("unsupported access mode %q", mode))
		}
	}
	return nil
}

func applicationSource(instance map[string]any, mode string) (string, string) {
	key, kind := map[string]string{"virtual_machine": "host_resource_id", "containerized": "docker_resource_id", "cloud_native": "kubernetes_resource_id"}[mode], map[string]string{"virtual_machine": "Host", "containerized": "Docker", "cloud_native": "Kubernetes"}[mode]
	value, _ := instance[key].(string)
	return strings.TrimSpace(value), kind
}

func (s *Service) validateHostInstance(ctx context.Context, source resource.Resource, instance map[string]any) error {
	keyword := strings.TrimSpace(applicationStringValue(instance["process_keyword"]))
	if keyword == "" {
		return fmt.Errorf("process_keyword is required")
	}
	result, err := s.invokeApplicationSource(ctx, source, "host_processes", map[string]any{"keyword": keyword, "limit": hosttool.MaxProcessLimit})
	if err != nil {
		return err
	}
	out, err := decodeApplicationTool[hosttool.ProcessesOutput](result)
	if err != nil {
		return err
	}
	if out.Truncated {
		return fmt.Errorf("process keyword %q matches too many processes; refine the keyword", keyword)
	}
	if out.MatchCount != 1 {
		return fmt.Errorf("process keyword must uniquely identify one process, matched %d", out.MatchCount)
	}
	return nil
}

func (s *Service) validateDockerInstance(ctx context.Context, source resource.Resource, instance map[string]any) error {
	name, _ := instance["container_name"].(string)
	result, err := s.invokeApplicationSource(ctx, source, "docker_containers", map[string]any{"all": true, "filters": "name=" + strings.TrimSpace(name), "limit": 100})
	if err != nil {
		return err
	}
	out, err := decodeApplicationTool[dockertool.ContainersOutput](result)
	if err != nil {
		return err
	}
	count := 0
	for _, container := range out.Containers {
		for _, item := range container.Names {
			if strings.TrimPrefix(item, "/") == strings.TrimSpace(name) {
				count++
			}
		}
	}
	if count != 1 {
		return fmt.Errorf("container name must uniquely identify one container, matched %d", count)
	}
	return nil
}

func (s *Service) validateKubernetesInstance(ctx context.Context, source resource.Resource, instance map[string]any) error {
	kind, _ := instance["workload_kind"].(string)
	if !map[string]bool{"Deployment": true, "StatefulSet": true, "DaemonSet": true, "Job": true, "CronJob": true}[strings.TrimSpace(kind)] {
		return fmt.Errorf("workload_kind is invalid")
	}
	namespace, _ := instance["namespace"].(string)
	name, _ := instance["workload_name"].(string)
	result, err := s.invokeApplicationSource(ctx, source, "kubernetes_workload_pods", map[string]any{"workload_kind": strings.TrimSpace(kind), "namespace": strings.TrimSpace(namespace), "workload_name": strings.TrimSpace(name), "limit": 1})
	if err != nil {
		return err
	}
	workload, err := decodeApplicationTool[kt.WorkloadPodsOutput](result)
	if err != nil {
		return err
	}
	if workload.Count == 0 {
		return fmt.Errorf("workload must resolve to at least one pod")
	}
	return nil
}
