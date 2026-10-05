package connector

import (
	"context"
	"errors"
	"fmt"

	"opskeeper/backend/engine"
	kt "opskeeper/backend/tool/kubernetes"
)

type DiscoveredWorkload struct {
	kt.ResourceItem
	Pods []kt.PodTarget `json:"pods"`
}

type WorkloadDiscovery struct {
	Items []DiscoveredWorkload `json:"items"`
	Count int                  `json:"count"`
}

// ListKubernetesWorkloads is the narrow read operation used by the application
// wizard. It deliberately exposes only allowlisted workload metadata.
func (s *Service) ListKubernetesWorkloads(ctx context.Context, resourceID, namespace, filters string, limit int) (WorkloadDiscovery, error) {
	if s.resources == nil {
		return WorkloadDiscovery{}, errors.New("resource service is unavailable")
	}
	item, err := s.resources.Get(ctx, resourceID)
	if err != nil {
		return WorkloadDiscovery{}, err
	}
	if item.Kind != "Kubernetes" {
		return WorkloadDiscovery{}, fmt.Errorf("resource %q is not a Kubernetes resource", item.Name)
	}
	connection, err := s.kubernetesConnection(ctx, engine.ContextResource{
		ID: item.ID, ScopeID: item.ScopeID, Kind: item.Kind, Name: item.Name,
		Status: item.Status, Subtype: item.Subtype, AgentRef: item.AgentRef, Config: item.Config,
	})
	if err != nil {
		return WorkloadDiscovery{}, err
	}
	workloads, err := kt.Workloads(ctx, connection, namespace, filters, "", limit)
	if err != nil {
		return WorkloadDiscovery{}, err
	}
	result := WorkloadDiscovery{Items: make([]DiscoveredWorkload, 0, len(workloads.Items))}
	for _, workload := range workloads.Items {
		pods, err := kt.WorkloadPods(ctx, connection, workload.Kind, workload.Namespace, workload.Name, 50)
		if err != nil {
			return WorkloadDiscovery{}, err
		}
		result.Items = append(result.Items, DiscoveredWorkload{ResourceItem: workload, Pods: pods.Pods})
	}
	result.Count = len(result.Items)
	return result, nil
}
