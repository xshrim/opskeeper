package connector

import "testing"

func TestKubernetesWorkloadRulesCoverDaemonSetAndProbeFailures(t *testing.T) {
	summary := kubernetesSummary("daemonsets", "platform", 1, []byte(`{"metadata":{"name":"agent"},"spec":{"template":{}},"status":{"desiredNumberScheduled":3,"numberReady":1,"updatedNumberScheduled":1,"numberAvailable":1}}`))
	items := summary["findings"].([]Finding)
	if !containsFinding(items, "kubernetes.daemonset_not_ready") || !containsFinding(items, "kubernetes.daemonset_rollout_incomplete") {
		t.Fatalf("daemonset findings = %#v", items)
	}
	podSummary := kubernetesSummary("pods", "platform", 1, []byte(`{"metadata":{"name":"agent-1"},"status":{"containerStatuses":[{"name":"agent","ready":false}]},"spec":{"containers":[{"name":"agent","resources":{"limits":{"cpu":"10m"}},"readinessProbe":{}}]}}`))
	if !containsFinding(podSummary["findings"].([]Finding), "kubernetes.container_not_ready") {
		t.Fatalf("probe findings = %#v", podSummary["findings"])
	}
}

func containsFinding(items []Finding, code string) bool {
	for _, item := range items {
		if item.Code == code {
			return true
		}
	}
	return false
}
