package discovery

import (
	"context"
	"fmt"
	"sort"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"opskeeper/backend/resource"
)

type KubernetesScanner struct {
	Factory func(*rest.Config) (dynamic.Interface, error)
}

func NewKubernetesScanner() *KubernetesScanner {
	return &KubernetesScanner{Factory: func(config *rest.Config) (dynamic.Interface, error) {
		return dynamic.NewForConfig(config)
	}}
}

func (s *KubernetesScanner) Scan(ctx context.Context, _ resource.Resource, kubeconfig string) ([]ScannedItem, error) {
	config, err := clientcmd.RESTConfigFromKubeConfig([]byte(kubeconfig))
	if err != nil {
		return nil, fmt.Errorf("load kubeconfig: %w", err)
	}
	client, err := s.Factory(config)
	if err != nil {
		return nil, fmt.Errorf("create Kubernetes client: %w", err)
	}

	namespaces, err := listAll(ctx, client, schema.GroupVersionResource{Version: "v1", Resource: "namespaces"}, false)
	if err != nil {
		return nil, fmt.Errorf("list Kubernetes namespaces: %w", err)
	}
	services, err := listAll(ctx, client, schema.GroupVersionResource{Version: "v1", Resource: "services"}, true)
	if err != nil {
		return nil, fmt.Errorf("list Kubernetes services: %w", err)
	}
	ingresses, err := listAll(ctx, client, schema.GroupVersionResource{Group: "networking.k8s.io", Version: "v1", Resource: "ingresses"}, true)
	if err != nil {
		return nil, fmt.Errorf("list Kubernetes ingresses: %w", err)
	}
	endpointSlices, err := listAll(ctx, client, schema.GroupVersionResource{Group: "discovery.k8s.io", Version: "v1", Resource: "endpointslices"}, true)
	if err != nil {
		return nil, fmt.Errorf("list Kubernetes endpoint slices: %w", err)
	}
	pods, err := listAll(ctx, client, schema.GroupVersionResource{Version: "v1", Resource: "pods"}, true)
	if err != nil {
		return nil, fmt.Errorf("list Kubernetes pods: %w", err)
	}

	items := make([]ScannedItem, 0, len(namespaces))
	for _, namespace := range namespaces {
		items = append(items, ScannedItem{
			Kind:            "Project",
			Namespace:       namespace.GetName(),
			Name:            namespace.GetName(),
			ExternalUID:     string(namespace.GetUID()),
			ResourceVersion: namespace.GetResourceVersion(),
			Labels:          labelsOrEmpty(namespace.GetLabels()),
			Payload: map[string]any{
				"kubernetes_kind": "Namespace",
				"namespace":       namespace.GetName(),
			},
		})
	}

	workloadQueries := []struct {
		objectKind string
		gvr        schema.GroupVersionResource
	}{
		{objectKind: "Deployment", gvr: schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}},
		{objectKind: "StatefulSet", gvr: schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "statefulsets"}},
		{objectKind: "DaemonSet", gvr: schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "daemonsets"}},
		{objectKind: "Job", gvr: schema.GroupVersionResource{Group: "batch", Version: "v1", Resource: "jobs"}},
		{objectKind: "CronJob", gvr: schema.GroupVersionResource{Group: "batch", Version: "v1", Resource: "cronjobs"}},
	}
	for _, query := range workloadQueries {
		workloads, err := listAll(ctx, client, query.gvr, true)
		if err != nil {
			return nil, fmt.Errorf("list Kubernetes %s: %w", query.gvr.Resource, err)
		}
		for _, workload := range workloads {
			items = append(items, applicationItem(query.objectKind, query.gvr, workload, services, ingresses, endpointSlices, pods))
		}
	}
	return items, nil
}

func applicationItem(objectKind string, gvr schema.GroupVersionResource, workload unstructured.Unstructured, services, ingresses, endpointSlices, pods []unstructured.Unstructured) ScannedItem {
	namespace := workload.GetNamespace()
	podLabels := workloadPodLabels(objectKind, workload.Object)
	matchedServices := make([]map[string]any, 0)
	serviceNames := make(map[string]struct{})
	for _, service := range services {
		if service.GetNamespace() != namespace {
			continue
		}
		selector, _, _ := unstructured.NestedStringMap(service.Object, "spec", "selector")
		if len(selector) == 0 || !selectorMatches(selector, podLabels) {
			continue
		}
		serviceNames[service.GetName()] = struct{}{}
		ports, _, _ := unstructured.NestedSlice(service.Object, "spec", "ports")
		matchedServices = append(matchedServices, map[string]any{
			"name":       service.GetName(),
			"type":       nestedString(service.Object, "spec", "type"),
			"cluster_ip": nestedString(service.Object, "spec", "clusterIP"),
			"ports":      ports,
		})
	}

	matchedIngresses := make([]map[string]any, 0)
	for _, ingress := range ingresses {
		if ingress.GetNamespace() != namespace || !ingressUsesService(ingress.Object, serviceNames) {
			continue
		}
		rules, _, _ := unstructured.NestedSlice(ingress.Object, "spec", "rules")
		tls, _, _ := unstructured.NestedSlice(ingress.Object, "spec", "tls")
		matchedIngresses = append(matchedIngresses, map[string]any{
			"name":  ingress.GetName(),
			"rules": rules,
			"tls":   tls,
		})
	}

	matchedEndpoints := make([]map[string]any, 0)
	for _, endpointSlice := range endpointSlices {
		if endpointSlice.GetNamespace() != namespace {
			continue
		}
		serviceName := endpointSlice.GetLabels()["kubernetes.io/service-name"]
		if _, matched := serviceNames[serviceName]; !matched {
			continue
		}
		endpoints, _, _ := unstructured.NestedSlice(endpointSlice.Object, "endpoints")
		ports, _, _ := unstructured.NestedSlice(endpointSlice.Object, "ports")
		matchedEndpoints = append(matchedEndpoints, map[string]any{
			"service":   serviceName,
			"addresses": endpointAddresses(endpoints),
			"ports":     ports,
		})
	}
	instances := make([]map[string]any, 0)
	for _, pod := range pods {
		if pod.GetNamespace() != namespace || len(podLabels) == 0 || !selectorMatches(podLabels, pod.GetLabels()) {
			continue
		}
		instances = append(instances, map[string]any{
			"name":       pod.GetName(),
			"uid":        string(pod.GetUID()),
			"phase":      nestedString(pod.Object, "status", "phase"),
			"pod_ip":     nestedString(pod.Object, "status", "podIP"),
			"node_name":  nestedString(pod.Object, "spec", "nodeName"),
			"ready":      podReady(pod.Object),
			"started_at": nestedString(pod.Object, "status", "startTime"),
		})
	}

	sort.Slice(matchedServices, func(i, j int) bool { return matchedServices[i]["name"].(string) < matchedServices[j]["name"].(string) })
	sort.Slice(matchedIngresses, func(i, j int) bool {
		return matchedIngresses[i]["name"].(string) < matchedIngresses[j]["name"].(string)
	})
	sort.Slice(instances, func(i, j int) bool {
		return instances[i]["name"].(string) < instances[j]["name"].(string)
	})
	payload := map[string]any{
		"kubernetes": map[string]any{
			"api_version":     gvr.GroupVersion().String(),
			"workload_kind":   objectKind,
			"workload_name":   workload.GetName(),
			"workload_uid":    string(workload.GetUID()),
			"namespace":       namespace,
			"selector_labels": podLabels,
		},
		"services":  matchedServices,
		"ingresses": matchedIngresses,
		"endpoints": matchedEndpoints,
		"instances": instances,
	}
	return ScannedItem{
		Kind:            "Application",
		Namespace:       namespace,
		Name:            workload.GetName(),
		ExternalUID:     string(workload.GetUID()),
		ResourceVersion: workload.GetResourceVersion(),
		Labels:          labelsOrEmpty(workload.GetLabels()),
		Payload:         payload,
	}
}

func workloadPodLabels(objectKind string, object map[string]any) map[string]string {
	fields := []string{"spec", "template", "metadata", "labels"}
	if objectKind == "CronJob" {
		fields = []string{"spec", "jobTemplate", "spec", "template", "metadata", "labels"}
	}
	labels, _, _ := unstructured.NestedStringMap(object, fields...)
	return labels
}

func podReady(object map[string]any) bool {
	conditions, _, _ := unstructured.NestedSlice(object, "status", "conditions")
	for _, rawCondition := range conditions {
		condition, _ := rawCondition.(map[string]any)
		if condition["type"] == "Ready" && condition["status"] == "True" {
			return true
		}
	}
	return false
}

func listAll(ctx context.Context, client dynamic.Interface, gvr schema.GroupVersionResource, namespaced bool) ([]unstructured.Unstructured, error) {
	var resourceClient dynamic.ResourceInterface
	if namespaced {
		resourceClient = client.Resource(gvr).Namespace(metav1.NamespaceAll)
	} else {
		resourceClient = client.Resource(gvr)
	}
	items := make([]unstructured.Unstructured, 0)
	continueToken := ""
	for {
		list, err := resourceClient.List(ctx, metav1.ListOptions{Limit: 500, Continue: continueToken})
		if err != nil {
			return nil, err
		}
		items = append(items, list.Items...)
		continueToken = list.GetContinue()
		if continueToken == "" {
			return items, nil
		}
	}
}

func selectorMatches(selector, labels map[string]string) bool {
	for key, value := range selector {
		if labels[key] != value {
			return false
		}
	}
	return true
}

func ingressUsesService(object map[string]any, services map[string]struct{}) bool {
	rules, _, _ := unstructured.NestedSlice(object, "spec", "rules")
	for _, rawRule := range rules {
		rule, _ := rawRule.(map[string]any)
		paths, _, _ := unstructured.NestedSlice(rule, "http", "paths")
		for _, rawPath := range paths {
			path, _ := rawPath.(map[string]any)
			name := nestedString(path, "backend", "service", "name")
			if _, ok := services[name]; ok {
				return true
			}
		}
	}
	defaultName := nestedString(object, "spec", "defaultBackend", "service", "name")
	_, ok := services[defaultName]
	return ok
}

func endpointAddresses(endpoints []any) []string {
	addresses := make([]string, 0)
	for _, rawEndpoint := range endpoints {
		endpoint, _ := rawEndpoint.(map[string]any)
		values, _, _ := unstructured.NestedStringSlice(endpoint, "addresses")
		addresses = append(addresses, values...)
	}
	sort.Strings(addresses)
	return addresses
}

func nestedString(object map[string]any, fields ...string) string {
	value, _, _ := unstructured.NestedString(object, fields...)
	return value
}

func labelsOrEmpty(labels map[string]string) map[string]string {
	if labels == nil {
		return map[string]string{}
	}
	return labels
}
