package discovery

import (
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
)

func TestApplicationItemAggregatesKubernetesDetails(t *testing.T) {
	workload := kubeObject("apps/v1", "Deployment", "orders", "payments", "deployment-uid", map[string]string{"app": "orders"})
	workload.Object["spec"] = map[string]any{
		"template": map[string]any{"metadata": map[string]any{"labels": map[string]any{"app": "orders"}}},
	}
	service := kubeObject("v1", "Service", "orders-http", "payments", "service-uid", nil)
	service.Object["spec"] = map[string]any{
		"selector":  map[string]any{"app": "orders"},
		"type":      "ClusterIP",
		"clusterIP": "10.96.0.10",
		"ports":     []any{map[string]any{"port": int64(8080)}},
	}
	ingress := kubeObject("networking.k8s.io/v1", "Ingress", "orders", "payments", "ingress-uid", nil)
	ingress.Object["spec"] = map[string]any{
		"rules": []any{map[string]any{
			"host": "orders.example.com",
			"http": map[string]any{"paths": []any{map[string]any{
				"backend": map[string]any{"service": map[string]any{"name": "orders-http"}},
			}}},
		}},
	}
	endpointSlice := kubeObject("discovery.k8s.io/v1", "EndpointSlice", "orders-http-abc", "payments", "endpoint-uid", map[string]string{"kubernetes.io/service-name": "orders-http"})
	endpointSlice.Object["endpoints"] = []any{map[string]any{"addresses": []any{"10.244.1.20"}}}
	endpointSlice.Object["ports"] = []any{map[string]any{"port": int64(8080)}}
	pod := kubeObject("v1", "Pod", "orders-7d9c", "payments", "pod-uid", map[string]string{"app": "orders"})
	pod.Object["spec"] = map[string]any{"nodeName": "node-a"}
	pod.Object["status"] = map[string]any{
		"phase":     "Running",
		"podIP":     "10.244.1.20",
		"startTime": "2026-08-16T02:00:00Z",
		"conditions": []any{map[string]any{
			"type": "Ready", "status": "True",
		}},
	}
	unrelatedPod := kubeObject("v1", "Pod", "other", "payments", "other-pod-uid", map[string]string{"app": "other"})

	item := applicationItem(
		"Deployment",
		schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"},
		workload,
		[]unstructured.Unstructured{service},
		[]unstructured.Unstructured{ingress},
		[]unstructured.Unstructured{endpointSlice},
		[]unstructured.Unstructured{pod, unrelatedPod},
	)

	if item.Kind != "Application" || item.Namespace != "payments" || item.ExternalUID != "deployment-uid" {
		t.Fatalf("application identity = %#v", item)
	}
	assertPayloadLength(t, item.Payload, "services", 1)
	assertPayloadLength(t, item.Payload, "ingresses", 1)
	assertPayloadLength(t, item.Payload, "endpoints", 1)
	assertPayloadLength(t, item.Payload, "instances", 1)
	instances := item.Payload["instances"].([]map[string]any)
	if instances[0]["name"] != "orders-7d9c" || instances[0]["uid"] != "pod-uid" || instances[0]["ready"] != true {
		t.Fatalf("application instances = %#v", instances)
	}
}

func TestCronJobUsesJobTemplateLabels(t *testing.T) {
	cronJob := kubeObject("batch/v1", "CronJob", "nightly", "payments", "cron-uid", nil)
	cronJob.Object["spec"] = map[string]any{
		"jobTemplate": map[string]any{
			"spec": map[string]any{
				"template": map[string]any{"metadata": map[string]any{"labels": map[string]any{"job": "nightly"}}},
			},
		},
	}
	labels := workloadPodLabels("CronJob", cronJob.Object)
	if labels["job"] != "nightly" {
		t.Fatalf("CronJob pod labels = %#v", labels)
	}
}

func kubeObject(apiVersion, kind, name, namespace, uid string, labels map[string]string) unstructured.Unstructured {
	item := unstructured.Unstructured{Object: map[string]any{
		"apiVersion": apiVersion,
		"kind":       kind,
		"metadata": map[string]any{
			"name":            name,
			"namespace":       namespace,
			"uid":             uid,
			"resourceVersion": "1",
		},
	}}
	item.SetUID(types.UID(uid))
	item.SetLabels(labels)
	return item
}

func assertPayloadLength(t *testing.T, payload map[string]any, key string, want int) {
	t.Helper()
	items, ok := payload[key].([]map[string]any)
	if !ok || len(items) != want {
		t.Fatalf("payload[%q] = %#v, want %d items", key, payload[key], want)
	}
}
