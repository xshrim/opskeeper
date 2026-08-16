package connector

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/clientcmd"
)

type kubernetesAdapter struct {
	target        Target
	dynamicClient dynamic.Interface
	serverVersion func(context.Context) (json.RawMessage, error)
}

type kubernetesResource struct {
	gvr        schema.GroupVersionResource
	namespaced bool
}

var allowedKubernetesResources = map[string]kubernetesResource{
	"namespaces":     {gvr: schema.GroupVersionResource{Version: "v1", Resource: "namespaces"}},
	"pods":           {gvr: schema.GroupVersionResource{Version: "v1", Resource: "pods"}, namespaced: true},
	"services":       {gvr: schema.GroupVersionResource{Version: "v1", Resource: "services"}, namespaced: true},
	"events":         {gvr: schema.GroupVersionResource{Version: "v1", Resource: "events"}, namespaced: true},
	"deployments":    {gvr: schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}, namespaced: true},
	"statefulsets":   {gvr: schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "statefulsets"}, namespaced: true},
	"daemonsets":     {gvr: schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "daemonsets"}, namespaced: true},
	"jobs":           {gvr: schema.GroupVersionResource{Group: "batch", Version: "v1", Resource: "jobs"}, namespaced: true},
	"cronjobs":       {gvr: schema.GroupVersionResource{Group: "batch", Version: "v1", Resource: "cronjobs"}, namespaced: true},
	"ingresses":      {gvr: schema.GroupVersionResource{Group: "networking.k8s.io", Version: "v1", Resource: "ingresses"}, namespaced: true},
	"endpointslices": {gvr: schema.GroupVersionResource{Group: "discovery.k8s.io", Version: "v1", Resource: "endpointslices"}, namespaced: true},
}

func newKubernetesAdapter(target Target, limits Limits) (Adapter, error) {
	kubeconfig, err := kubeconfigSecret(target.Secret)
	if err != nil {
		return nil, err
	}
	config, err := clientcmd.RESTConfigFromKubeConfig([]byte(kubeconfig))
	if err != nil {
		return nil, connectorError(CategoryConfiguration, "load kubeconfig", false, err)
	}
	config.Timeout = limits.Timeout
	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, connectorError(CategoryConfiguration, "create Kubernetes client", false, err)
	}
	discoveryClient, err := discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		return nil, connectorError(CategoryConfiguration, "create Kubernetes discovery client", false, err)
	}
	return &kubernetesAdapter{
		target: target, dynamicClient: dynamicClient,
		serverVersion: func(ctx context.Context) (json.RawMessage, error) {
			body, err := discoveryClient.RESTClient().Get().AbsPath("/version").Do(ctx).Raw()
			if err != nil {
				return nil, kubernetesError("read Kubernetes version", err)
			}
			return json.RawMessage(body), nil
		},
	}, nil
}

func (a *kubernetesAdapter) Kind() string { return "Kubernetes" }

func (a *kubernetesAdapter) Capabilities() []Capability {
	return []Capability{CapabilityKubernetesRead}
}

func (a *kubernetesAdapter) Test(ctx context.Context) error {
	_, err := a.serverVersion(ctx)
	return err
}

func (a *kubernetesAdapter) ReadKubernetes(ctx context.Context, query KubernetesQuery) (Evidence, error) {
	resourceName := strings.ToLower(strings.TrimSpace(query.Resource))
	definition, ok := allowedKubernetesResources[resourceName]
	if !ok {
		return Evidence{}, connectorError(CategoryUnsupported, "read Kubernetes", false, fmt.Errorf("resource %q is not allowed", resourceName))
	}
	var client dynamic.ResourceInterface
	if definition.namespaced {
		if strings.TrimSpace(query.Namespace) == "" {
			return Evidence{}, connectorError(CategoryConfiguration, "read Kubernetes", false, errors.New("namespace is required for namespaced resources"))
		}
		client = a.dynamicClient.Resource(definition.gvr).Namespace(query.Namespace)
	} else {
		client = a.dynamicClient.Resource(definition.gvr)
	}
	var payload []byte
	count := 1
	if strings.TrimSpace(query.Name) != "" {
		item, err := client.Get(ctx, query.Name, metav1.GetOptions{})
		if err != nil {
			return Evidence{}, kubernetesError("get Kubernetes object", err)
		}
		payload, err = json.Marshal(item.Object)
		if err != nil {
			return Evidence{}, connectorError(CategoryInternal, "encode Kubernetes object", false, err)
		}
	} else {
		list, err := client.List(ctx, metav1.ListOptions{Limit: query.Limit, LabelSelector: query.LabelSelector})
		if err != nil {
			return Evidence{}, kubernetesError("list Kubernetes objects", err)
		}
		count = len(list.Items)
		partial := list.GetContinue() != ""
		payload, err = json.Marshal(list.Object)
		if err != nil {
			return Evidence{}, connectorError(CategoryInternal, "encode Kubernetes objects", false, err)
		}
		return Evidence{
			CollectedAt: time.Now(), Data: json.RawMessage(payload), Partial: partial,
			Summary: map[string]any{"resource": resourceName, "namespace": query.Namespace, "item_count": count},
		}, nil
	}
	return Evidence{
		CollectedAt: time.Now(), Data: json.RawMessage(payload),
		Summary: map[string]any{"resource": resourceName, "namespace": query.Namespace, "item_count": count},
	}, nil
}

func kubernetesError(operation string, err error) error {
	switch {
	case apierrors.IsUnauthorized(err), apierrors.IsForbidden(err):
		return connectorError(CategoryAuthentication, operation, false, err)
	case apierrors.IsTimeout(err), apierrors.IsServerTimeout(err):
		return connectorError(CategoryTimeout, operation, true, err)
	case apierrors.IsTooManyRequests(err):
		return connectorError(CategoryRateLimited, operation, true, err)
	default:
		return connectorError(CategoryUpstream, operation, apierrors.IsInternalError(err) || apierrors.IsServiceUnavailable(err), err)
	}
}

func kubeconfigSecret(secret []byte) (string, error) {
	value := strings.TrimSpace(string(secret))
	if value == "" {
		return "", connectorError(CategoryConfiguration, "read kubeconfig", false, errors.New("kubeconfig credential is required"))
	}
	var fields map[string]string
	if json.Unmarshal(secret, &fields) == nil {
		value = strings.TrimSpace(fields["kubeconfig"])
	}
	if value == "" {
		return "", connectorError(CategoryConfiguration, "read kubeconfig", false, errors.New("credential does not contain kubeconfig"))
	}
	return value, nil
}
