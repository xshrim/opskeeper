package connector

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
)

type ApplicationCandidateRequest struct {
	RuntimeKind   string   `json:"runtime_kind"`
	ResourceIDs   []string `json:"resource_ids"`
	Keyword       string   `json:"keyword"`
	ExactIdentity string   `json:"exact_identity,omitempty"`
	Namespace     string   `json:"namespace"`
	Filters       string   `json:"filters"`
}

type ApplicationCandidateInstance struct {
	Name             string         `json:"name"`
	TargetResourceID string         `json:"target_resource_id"`
	Selector         map[string]any `json:"selector"`
	Status           string         `json:"status,omitempty"`
}

type ApplicationCandidate struct {
	Key           string                         `json:"key"`
	Identity      string                         `json:"identity"`
	RuntimeKind   string                         `json:"runtime_kind"`
	Name          string                         `json:"name"`
	Detail        string                         `json:"detail,omitempty"`
	Kind          string                         `json:"kind,omitempty"`
	Namespace     string                         `json:"namespace,omitempty"`
	Replicas      int                            `json:"replicas"`
	ReadyReplicas int                            `json:"ready_replicas,omitempty"`
	Instances     []ApplicationCandidateInstance `json:"instances"`
}

type ApplicationCandidateDiscovery struct {
	Items     []ApplicationCandidate `json:"items"`
	Count     int                    `json:"count"`
	Truncated bool                   `json:"truncated,omitempty"`
}

type candidateResourceMatch struct {
	candidate ApplicationCandidate
	count     int
}

// DiscoverApplicationCandidates returns only Host/Docker identities present
// exactly once in every selected runtime resource. Kubernetes is already
// grouped by workload, so it accepts one cluster resource.
func (s *Service) DiscoverApplicationCandidates(ctx context.Context, input ApplicationCandidateRequest) (ApplicationCandidateDiscovery, error) {
	input.RuntimeKind = strings.TrimSpace(input.RuntimeKind)
	input.Keyword = strings.TrimSpace(input.Keyword)
	input.ExactIdentity = strings.TrimSpace(input.ExactIdentity)
	input.Namespace = strings.TrimSpace(input.Namespace)
	input.Filters = strings.TrimSpace(input.Filters)
	resourceIDs := uniqueIDs(input.ResourceIDs)
	if len(resourceIDs) == 0 {
		return ApplicationCandidateDiscovery{}, invalid("at least one runtime resource is required")
	}
	if input.RuntimeKind == "cloud_native" {
		if len(resourceIDs) != 1 || input.Namespace == "" {
			return ApplicationCandidateDiscovery{}, invalid("Kubernetes requires one runtime resource and a namespace")
		}
		discovery, err := s.ListKubernetesWorkloads(ctx, resourceIDs[0], "", input.Namespace, input.Filters, 100)
		if err != nil {
			return ApplicationCandidateDiscovery{}, err
		}
		items := make([]ApplicationCandidate, 0, len(discovery.Items))
		for _, workload := range discovery.Items {
			identity := strings.Join([]string{workload.Namespace, workload.Kind, workload.Name}, "/")
			if input.ExactIdentity != "" && input.ExactIdentity != identity {
				continue
			}
			instances := make([]ApplicationCandidateInstance, 0, len(workload.Pods))
			for _, pod := range workload.Pods {
				instances = append(instances, ApplicationCandidateInstance{
					Name: pod.Name, TargetResourceID: resourceIDs[0],
					Status: workloadInstanceStatus(workload),
					Selector: map[string]any{
						"namespace": workload.Namespace, "workload_kind": workload.Kind,
						"workload_name": workload.Name, "pod_name": pod.Name,
					},
				})
			}
			if len(instances) == 0 {
				instances = append(instances, ApplicationCandidateInstance{
					Name: workload.Name, TargetResourceID: resourceIDs[0],
					Selector: map[string]any{
						"namespace": workload.Namespace, "workload_kind": workload.Kind,
						"workload_name": workload.Name,
					},
					Status: workloadInstanceStatus(workload),
				})
			}
			items = append(items, ApplicationCandidate{
				Key: candidateKey(input.RuntimeKind, identity), Identity: identity, RuntimeKind: input.RuntimeKind, Name: workload.Name,
				Detail: workload.Kind, Kind: workload.Kind, Namespace: workload.Namespace,
				Replicas: int(workload.Replicas), ReadyReplicas: int(workload.ReadyReplicas), Instances: instances,
			})
		}
		sort.Slice(items, func(i, j int) bool {
			if items[i].Name == items[j].Name {
				return items[i].Identity < items[j].Identity
			}
			return items[i].Name < items[j].Name
		})
		return ApplicationCandidateDiscovery{Items: items, Count: len(items), Truncated: discovery.Truncated}, nil
	}
	if input.RuntimeKind != "virtual_machine" && input.RuntimeKind != "containerized" {
		return ApplicationCandidateDiscovery{}, invalid("unsupported application runtime kind")
	}
	if input.RuntimeKind == "virtual_machine" && input.Keyword == "" && input.ExactIdentity == "" {
		return ApplicationCandidateDiscovery{}, invalid("Host process keyword is required")
	}

	byIdentity := make(map[string]map[string]*candidateResourceMatch)
	result := ApplicationCandidateDiscovery{}
	for _, resourceID := range resourceIDs {
		selectorKey := "container_name"
		if input.RuntimeKind == "virtual_machine" {
			selectorKey = "process_keyword"
		}
		selector := map[string]any{selectorKey: input.Keyword}
		if input.ExactIdentity != "" {
			if input.RuntimeKind == "virtual_machine" {
				selector = map[string]any{"command_line": input.ExactIdentity}
			} else {
				selector = map[string]any{"container_name": input.ExactIdentity, "exact": true}
			}
		}
		preview, err := s.PreviewApplicationBinding(ctx, input.RuntimeKind, resourceID, selector)
		if err != nil {
			return ApplicationCandidateDiscovery{}, err
		}
		result.Truncated = result.Truncated || preview.Truncated
		resource, err := s.resources.Get(ctx, resourceID)
		if err != nil {
			return ApplicationCandidateDiscovery{}, err
		}
		for _, item := range preview.Items {
			identity := strings.TrimSpace(item.Name)
			selector := map[string]any{"container_name": identity}
			if input.RuntimeKind == "virtual_machine" {
				identity = strings.TrimSpace(item.Detail)
				selector = map[string]any{"process_command_line": identity}
			}
			if identity == "" {
				continue
			}
			resourceMatches := byIdentity[identity]
			if resourceMatches == nil {
				resourceMatches = make(map[string]*candidateResourceMatch)
				byIdentity[identity] = resourceMatches
			}
			current := resourceMatches[resourceID]
			if current == nil {
				current = &candidateResourceMatch{candidate: ApplicationCandidate{
					Key: candidateKey(input.RuntimeKind, identity), Identity: identity, RuntimeKind: input.RuntimeKind, Name: item.Name,
					Detail: item.Detail, Instances: []ApplicationCandidateInstance{},
				}}
				resourceMatches[resourceID] = current
			}
			current.count++
			if current.count > 1 {
				continue
			}
			current.candidate.Instances = append(current.candidate.Instances, ApplicationCandidateInstance{
				Name: resource.Name, TargetResourceID: resourceID, Selector: selector, Status: candidateInstanceStatus(input.RuntimeKind, item.Status),
			})
		}
	}
	result.Items = finalizeApplicationCandidates(input.RuntimeKind, resourceIDs, byIdentity)
	result.Count = len(result.Items)
	return result, nil
}

func finalizeApplicationCandidates(runtimeKind string, resourceIDs []string, byIdentity map[string]map[string]*candidateResourceMatch) []ApplicationCandidate {
	items := make([]ApplicationCandidate, 0, len(byIdentity))
	for identity, byResource := range byIdentity {
		if len(byResource) != len(resourceIDs) {
			continue
		}
		candidate := byResource[resourceIDs[0]].candidate
		candidate.Instances = make([]ApplicationCandidateInstance, 0, len(resourceIDs))
		unique := true
		for _, resourceID := range resourceIDs {
			item := byResource[resourceID]
			if item == nil || item.count != 1 {
				unique = false
				break
			}
			instance := item.candidate.Instances[0]
			instance.Name = uniqueInstanceName(candidate.Instances, instance.Name)
			candidate.Instances = append(candidate.Instances, instance)
		}
		candidate.Key = candidateKey(runtimeKind, identity)
		candidate.Replicas = len(resourceIDs)
		if !unique {
			continue
		}
		items = append(items, candidate)
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Name == items[j].Name {
			return items[i].Identity < items[j].Identity
		}
		return items[i].Name < items[j].Name
	})
	return items
}

func uniqueInstanceName(instances []ApplicationCandidateInstance, base string) string {
	base = strings.TrimSpace(base)
	if base == "" {
		base = "instance"
	}
	name := base
	for suffix := 2; ; suffix++ {
		used := false
		for _, instance := range instances {
			if instance.Name == name {
				used = true
				break
			}
		}
		if !used {
			return name
		}
		name = fmt.Sprintf("%s-%d", base, suffix)
	}
}

func uniqueIDs(ids []string) []string {
	seen := make(map[string]struct{}, len(ids))
	result := make([]string, 0, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		result = append(result, id)
	}
	return result
}

func candidateKey(runtimeKind, identity string) string {
	hash := sha256.Sum256([]byte(runtimeKind + "\x00" + identity))
	return hex.EncodeToString(hash[:])
}

func candidateInstanceStatus(runtimeKind, observed string) string {
	observed = strings.ToLower(strings.TrimSpace(observed))
	switch runtimeKind {
	case "virtual_machine":
		if observed == "" || observed == "z" || observed == "x" {
			return "unknown"
		}
		return "active"
	case "containerized":
		if strings.HasPrefix(observed, "up") {
			return "active"
		}
		return "unknown"
	default:
		return "active"
	}
}

func workloadInstanceStatus(workload DiscoveredWorkload) string {
	if workload.Replicas > 0 && workload.ReadyReplicas >= workload.Replicas {
		return "active"
	}
	return "unknown"
}
