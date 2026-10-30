package connector

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"opskeeper/backend/engine"
	kclient "opskeeper/backend/mcpserver/kubernetes/client"
	kt "opskeeper/backend/tool/kubernetes"
)

type DiscoveredWorkload struct {
	kt.ResourceItem
	Pods []kt.PodTarget `json:"pods"`
}

type WorkloadDiscovery struct {
	Items     []DiscoveredWorkload `json:"items"`
	Count     int                  `json:"count"`
	Truncated bool                 `json:"truncated,omitempty"`
}

type KubernetesNamespace struct {
	Name string `json:"name"`
}

// ListKubernetesWorkloads is the narrow read operation used by the application
// wizard. It deliberately exposes only allowlisted workload metadata.
func (s *Service) ListKubernetesContexts(ctx context.Context, resourceID string) ([]kclient.ContextOption, error) {
	connection, err := s.kubernetesResourceConnection(ctx, resourceID)
	if err != nil {
		return nil, err
	}
	return kclient.Contexts(connection)
}

func (s *Service) ListKubernetesNamespaces(ctx context.Context, resourceID, contextName string) ([]KubernetesNamespace, error) {
	connection, err := s.kubernetesResourceConnection(ctx, resourceID)
	if err != nil {
		return nil, err
	}
	if contextName = strings.TrimSpace(contextName); contextName != "" {
		connection.Context = contextName
	}
	listed, err := kt.List(ctx, connection, "namespaces", "", "", "", 500)
	if err != nil {
		return nil, err
	}
	items := make([]KubernetesNamespace, 0, len(listed.Items))
	for _, item := range listed.Items {
		items = append(items, KubernetesNamespace{Name: item.Name})
	}
	return items, nil
}

func (s *Service) ListKubernetesWorkloads(ctx context.Context, resourceID, contextName, namespace, filters string, limit int) (WorkloadDiscovery, error) {
	connection, err := s.kubernetesResourceConnection(ctx, resourceID)
	if err != nil {
		return WorkloadDiscovery{}, err
	}
	if contextName = strings.TrimSpace(contextName); contextName != "" {
		connection.Context = contextName
	}
	workloads, err := kt.Workloads(ctx, connection, namespace, filters, "", limit)
	if err != nil {
		return WorkloadDiscovery{}, err
	}
	result := WorkloadDiscovery{Items: make([]DiscoveredWorkload, 0, len(workloads.Items)), Truncated: workloads.Truncated}
	for _, workload := range workloads.Items {
		pods, err := kt.WorkloadPods(ctx, connection, workload.Kind, workload.Namespace, workload.Name, 50)
		if err != nil {
			return WorkloadDiscovery{}, err
		}
		result.Truncated = result.Truncated || pods.Truncated
		result.Items = append(result.Items, DiscoveredWorkload{ResourceItem: workload, Pods: pods.Pods})
	}
	result.Count = len(result.Items)
	return result, nil
}

func (s *Service) kubernetesResourceConnection(ctx context.Context, resourceID string) (kclient.ConnectionInput, error) {
	if s.resources == nil {
		return kclient.ConnectionInput{}, errors.New("resource service is unavailable")
	}
	item, err := s.resources.Get(ctx, resourceID)
	if err != nil {
		return kclient.ConnectionInput{}, err
	}
	if item.Kind != "Kubernetes" {
		return kclient.ConnectionInput{}, fmt.Errorf("resource %q is not a Kubernetes resource", item.Name)
	}
	connection, err := s.kubernetesConnection(ctx, engine.ContextResource{
		ID: item.ID, ScopeID: item.ScopeID, Kind: item.Kind, Name: item.Name,
		Subtype: item.Subtype, AgentRef: item.AgentRef, Config: item.Config,
	})
	if err != nil {
		return kclient.ConnectionInput{}, err
	}
	return connection, nil
}
