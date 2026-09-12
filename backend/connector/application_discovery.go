package connector

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"opskeeper/backend/resource"
	dockertool "opskeeper/backend/tool/docker"
	hosttool "opskeeper/backend/tool/host"
	kt "opskeeper/backend/tool/kubernetes"
)

const applicationDiscoveryMaxLimit = 100

type ApplicationHostProcess struct {
	PID         int    `json:"pid"`
	Name        string `json:"name,omitempty"`
	Executable  string `json:"executable,omitempty"`
	CommandLine string `json:"command_line,omitempty"`
	State       string `json:"state,omitempty"`
}

type ApplicationHostProcesses struct {
	Keyword    string                   `json:"keyword"`
	Processes  []ApplicationHostProcess `json:"processes"`
	MatchCount int                      `json:"match_count"`
	Truncated  bool                     `json:"truncated"`
}

type ApplicationHostProcessValidation struct {
	Keyword    string `json:"keyword"`
	PID        int    `json:"pid"`
	Valid      bool   `json:"valid"`
	MatchCount int    `json:"match_count"`
	Message    string `json:"message,omitempty"`
}

type ApplicationDockerContainer struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Image  string `json:"image,omitempty"`
	State  string `json:"state,omitempty"`
	Status string `json:"status,omitempty"`
}

type ApplicationDockerContainers struct {
	Containers []ApplicationDockerContainer `json:"containers"`
}

type ApplicationKubernetesNamespace struct {
	Name string `json:"name"`
}

type ApplicationKubernetesNamespaces struct {
	Namespaces []ApplicationKubernetesNamespace `json:"namespaces"`
}

type ApplicationKubernetesWorkload struct {
	Namespace string            `json:"namespace"`
	Kind      string            `json:"kind"`
	Name      string            `json:"name"`
	Ready     bool              `json:"ready,omitempty"`
	Phase     string            `json:"phase,omitempty"`
	Age       string            `json:"age,omitempty"`
	Labels    map[string]string `json:"labels,omitempty"`
}

type ApplicationKubernetesWorkloads struct {
	Namespace string                          `json:"namespace"`
	Workloads []ApplicationKubernetesWorkload `json:"workloads"`
}

func (s *Service) applicationDiscoverySource(ctx context.Context, resourceID, kind string) (resource.Resource, error) {
	resourceID = strings.TrimSpace(resourceID)
	if resourceID == "" {
		return resource.Resource{}, invalid("resource_id is required")
	}
	if s.resources == nil {
		return resource.Resource{}, connectorError(CategoryInternal, "discover Application target", false, fmt.Errorf("resource service is unavailable"))
	}
	item, err := s.resources.Get(ctx, resourceID)
	if err != nil {
		return resource.Resource{}, err
	}
	if item.Kind != kind {
		return resource.Resource{}, invalid(fmt.Sprintf("resource must be a %s resource", kind))
	}
	if item.Status != resource.StatusActive {
		return resource.Resource{}, invalid("resource must be active")
	}
	return item, nil
}

func applicationDiscoveryLimit(limit int) int {
	if limit <= 0 {
		return 50
	}
	if limit > applicationDiscoveryMaxLimit {
		return applicationDiscoveryMaxLimit
	}
	return limit
}

func (s *Service) DiscoverApplicationHostProcesses(ctx context.Context, resourceID, keyword string, limit int) (ApplicationHostProcesses, error) {
	keyword = strings.TrimSpace(keyword)
	if keyword == "" {
		return ApplicationHostProcesses{}, invalid("keyword is required to discover Host processes")
	}
	item, err := s.applicationDiscoverySource(ctx, resourceID, "Host")
	if err != nil {
		return ApplicationHostProcesses{}, err
	}
	result, err := s.invokeApplicationSource(ctx, item, "host_processes", map[string]any{"keyword": keyword, "limit": applicationDiscoveryLimit(limit)})
	if err != nil {
		if errors.Is(err, hosttool.ErrInvalidArgument) {
			return ApplicationHostProcesses{}, invalid(err.Error())
		}
		return ApplicationHostProcesses{}, err
	}
	out, err := decodeApplicationTool[hosttool.ProcessesOutput](result)
	if err != nil {
		return ApplicationHostProcesses{}, err
	}
	processes := make([]ApplicationHostProcess, 0, len(out.Processes))
	for _, process := range out.Processes {
		processes = append(processes, ApplicationHostProcess{PID: process.PID, Name: process.Name, Executable: process.Executable, CommandLine: process.CommandLine, State: process.State})
	}
	return ApplicationHostProcesses{Keyword: keyword, Processes: processes, MatchCount: out.MatchCount, Truncated: out.Truncated}, nil
}

func (s *Service) ValidateApplicationHostProcess(ctx context.Context, resourceID, keyword string, pid int) (ApplicationHostProcessValidation, error) {
	keyword = strings.TrimSpace(keyword)
	if keyword == "" {
		return ApplicationHostProcessValidation{}, invalid("keyword is required")
	}
	if pid <= 0 {
		return ApplicationHostProcessValidation{}, invalid("pid must be positive")
	}
	item, err := s.applicationDiscoverySource(ctx, resourceID, "Host")
	if err != nil {
		return ApplicationHostProcessValidation{}, err
	}
	toolResult, err := s.invokeApplicationSource(ctx, item, "host_processes", map[string]any{"keyword": keyword, "limit": 2})
	if err != nil {
		if errors.Is(err, hosttool.ErrInvalidArgument) {
			return ApplicationHostProcessValidation{}, invalid(err.Error())
		}
		return ApplicationHostProcessValidation{}, err
	}
	out, err := decodeApplicationTool[hosttool.ProcessesOutput](toolResult)
	if err != nil {
		return ApplicationHostProcessValidation{}, err
	}
	selected := false
	for _, process := range out.Processes {
		if process.PID == pid {
			selected = true
			break
		}
	}
	result := ApplicationHostProcessValidation{Keyword: keyword, PID: pid, MatchCount: out.MatchCount}
	switch {
	case out.MatchCount == 0:
		result.Message = "搜索关键字未匹配到运行中的进程"
	case out.MatchCount != 1:
		result.Message = fmt.Sprintf("搜索关键字匹配到 %d 个进程，必须唯一定位一个进程", out.MatchCount)
	case !selected:
		result.Message = "选中的进程已退出或搜索结果已变化，请重新搜索"
	default:
		result.Valid = true
	}
	return result, nil
}

func (s *Service) DiscoverApplicationDockerContainers(ctx context.Context, resourceID, keyword string, limit int) (ApplicationDockerContainers, error) {
	item, err := s.applicationDiscoverySource(ctx, resourceID, "Docker")
	if err != nil {
		return ApplicationDockerContainers{}, err
	}
	result, err := s.invokeApplicationSource(ctx, item, "docker_containers", map[string]any{"limit": applicationDiscoveryLimit(limit)})
	if err != nil {
		return ApplicationDockerContainers{}, err
	}
	out, err := decodeApplicationTool[dockertool.ContainersOutput](result)
	if err != nil {
		return ApplicationDockerContainers{}, err
	}
	keyword = strings.ToLower(strings.TrimSpace(keyword))
	containers := make([]ApplicationDockerContainer, 0, len(out.Containers))
	for _, container := range out.Containers {
		for _, rawName := range container.Names {
			name := strings.TrimPrefix(strings.TrimSpace(rawName), "/")
			if name == "" || (keyword != "" && !strings.Contains(strings.ToLower(name+" "+container.Image+" "+container.Status), keyword)) {
				continue
			}
			containers = append(containers, ApplicationDockerContainer{ID: container.ID, Name: name, Image: container.Image, State: container.State, Status: container.Status})
		}
	}
	sort.Slice(containers, func(i, j int) bool { return containers[i].Name < containers[j].Name })
	return ApplicationDockerContainers{Containers: containers}, nil
}

func (s *Service) DiscoverApplicationKubernetesNamespaces(ctx context.Context, resourceID string, includeSystem bool, limit int) (ApplicationKubernetesNamespaces, error) {
	item, err := s.applicationDiscoverySource(ctx, resourceID, "Kubernetes")
	if err != nil {
		return ApplicationKubernetesNamespaces{}, err
	}
	result, err := s.invokeApplicationSource(ctx, item, "kubernetes_namespaces", map[string]any{"limit": applicationDiscoveryMaxLimit})
	if err != nil {
		return ApplicationKubernetesNamespaces{}, err
	}
	out, err := decodeApplicationTool[kt.ListOutput](result)
	if err != nil {
		return ApplicationKubernetesNamespaces{}, err
	}
	namespaces := make([]ApplicationKubernetesNamespace, 0, len(out.Items))
	for _, namespace := range out.Items {
		if !includeSystem && applicationInfrastructureName(namespace.Name) {
			continue
		}
		namespaces = append(namespaces, ApplicationKubernetesNamespace{Name: namespace.Name})
		if len(namespaces) >= applicationDiscoveryLimit(limit) {
			break
		}
	}
	sort.Slice(namespaces, func(i, j int) bool { return namespaces[i].Name < namespaces[j].Name })
	return ApplicationKubernetesNamespaces{Namespaces: namespaces}, nil
}

func (s *Service) DiscoverApplicationKubernetesWorkloads(ctx context.Context, resourceID, namespace string, limit int) (ApplicationKubernetesWorkloads, error) {
	namespace = strings.TrimSpace(namespace)
	if namespace == "" {
		return ApplicationKubernetesWorkloads{}, invalid("namespace is required")
	}
	item, err := s.applicationDiscoverySource(ctx, resourceID, "Kubernetes")
	if err != nil {
		return ApplicationKubernetesWorkloads{}, err
	}
	result, err := s.invokeApplicationSource(ctx, item, "kubernetes_workloads", map[string]any{"namespace": namespace, "limit": applicationDiscoveryLimit(limit)})
	if err != nil {
		return ApplicationKubernetesWorkloads{}, err
	}
	out, err := decodeApplicationTool[kt.ListOutput](result)
	if err != nil {
		return ApplicationKubernetesWorkloads{}, err
	}
	workloads := make([]ApplicationKubernetesWorkload, 0, len(out.Items))
	for _, workload := range out.Items {
		workloads = append(workloads, ApplicationKubernetesWorkload{Namespace: namespace, Kind: workload.Kind, Name: workload.Name, Ready: workload.Ready, Phase: workload.Phase, Age: workload.Age, Labels: workload.Labels})
		if len(workloads) >= applicationDiscoveryLimit(limit) {
			break
		}
	}
	sort.Slice(workloads, func(i, j int) bool {
		if workloads[i].Kind == workloads[j].Kind {
			return workloads[i].Name < workloads[j].Name
		}
		return workloads[i].Kind < workloads[j].Kind
	})
	return ApplicationKubernetesWorkloads{Namespace: namespace, Workloads: workloads}, nil
}

func applicationInfrastructureName(name string) bool {
	lower := strings.ToLower(strings.TrimSpace(name))
	if lower == "kube-system" || lower == "kube-node-lease" || lower == "kube-public" {
		return true
	}
	for _, token := range []string{
		"calico", "cilium", "flannel", "metallb", "ingress-nginx", "traefik", "istio", "gateway",
		"longhorn", "openebs", "rook", "ceph", "csi", "cert-manager", "metrics-server", "prometheus", "rancher", "local-path",
	} {
		if strings.Contains(lower, token) {
			return true
		}
	}
	return false
}
