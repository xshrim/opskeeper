package kubernetes

import (
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestValidatePodPath(t *testing.T) {
	for _, path := range []string{"", "relative.log", "/var/log/../etc/passwd", "/"} {
		if _, err := validatePodPath(path); err == nil {
			t.Errorf("validatePodPath(%q) succeeded", path)
		}
	}
	if got, err := validatePodPath("/var/log/app.log"); err != nil || got != "/var/log/app.log" {
		t.Fatalf("validatePodPath = %q, %v", got, err)
	}
}

func TestMetricStatAggregatesPodContainerUsage(t *testing.T) {
	item := unstructured.Unstructured{Object: map[string]any{
		"metadata":  map[string]any{"namespace": "platform", "name": "api-1"},
		"timestamp": "2026-09-10T03:00:00Z",
		"window":    "30s",
		"containers": []any{
			map[string]any{"name": "api", "usage": map[string]any{"cpu": "10m", "memory": "20Mi"}},
			map[string]any{"name": "sidecar", "usage": map[string]any{"cpu": "25m", "memory": "30Mi"}},
		},
	}}

	stat := metricStat(item, true)
	if stat.Namespace != "platform" || stat.Name != "api-1" || stat.Timestamp == "" {
		t.Fatalf("stat identity = %#v", stat)
	}
	if stat.Usage["cpu"] != "35m" || stat.Usage["memory"] != "50Mi" {
		t.Fatalf("aggregated usage = %#v", stat.Usage)
	}
	if len(stat.Containers) != 2 || stat.Containers[1].Name != "sidecar" {
		t.Fatalf("containers = %#v", stat.Containers)
	}
}

func TestMetricStatKeepsNodeUsage(t *testing.T) {
	item := unstructured.Unstructured{Object: map[string]any{
		"metadata": map[string]any{"name": "node-1"},
		"usage":    map[string]any{"cpu": "1200m", "memory": "2Gi"},
	}}
	stat := metricStat(item, false)
	if stat.Name != "node-1" || stat.Usage["cpu"] != "1200m" || len(stat.Containers) != 0 {
		t.Fatalf("node stat = %#v", stat)
	}
}
