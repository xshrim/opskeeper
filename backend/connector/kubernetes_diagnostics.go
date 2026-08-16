package connector

import (
	"encoding/json"
	"fmt"
	"strings"
)

// kubernetesSummary evaluates only documented Kubernetes status and spec
// fields from an already-authorized response. It deliberately ignores labels,
// annotations and container command text: those are evidence, never
// instructions for the diagnostic system.
func kubernetesSummary(resource, namespace string, count int, payload []byte) map[string]any {
	items := kubernetesItems(payload)
	findings := make([]Finding, 0)
	for _, item := range items {
		switch resource {
		case "pods":
			findings = append(findings, podFindings(item)...)
		case "events":
			findings = append(findings, eventFindings(item)...)
		case "deployments":
			findings = append(findings, deploymentFindings(item)...)
		case "statefulsets":
			findings = append(findings, statefulSetFindings(item)...)
		case "daemonsets":
			findings = append(findings, daemonSetFindings(item)...)
		case "jobs":
			findings = append(findings, jobFindings(item)...)
		}
	}
	return map[string]any{"resource": resource, "namespace": namespace, "item_count": count, "findings": uniqueFindings(findings)}
}

func kubernetesItems(payload []byte) []map[string]any {
	var object map[string]any
	if json.Unmarshal(payload, &object) != nil {
		return nil
	}
	if values, ok := object["items"].([]any); ok {
		result := make([]map[string]any, 0, len(values))
		for _, value := range values {
			if item, ok := value.(map[string]any); ok {
				result = append(result, item)
			}
		}
		return result
	}
	return []map[string]any{object}
}

func podFindings(pod map[string]any) []Finding {
	name := kubernetesName(pod)
	status := objectField(pod, "status")
	spec := objectField(pod, "spec")
	result := make([]Finding, 0)
	if stringField(status, "phase") == "Pending" {
		result = append(result, Finding{Code: "kubernetes.pod_pending", Severity: "warning", Message: fmt.Sprintf("Pod %s 仍处于 Pending 状态", name)})
	}
	for _, condition := range arrayObjects(status, "conditions") {
		if stringField(condition, "type") == "PodScheduled" && stringField(condition, "status") == "False" && stringField(condition, "reason") == "Unschedulable" {
			result = append(result, Finding{Code: "kubernetes.pod_unschedulable", Severity: "warning", Message: fmt.Sprintf("Pod %s 无法调度", name)})
		}
		if stringField(condition, "type") == "Ready" && stringField(condition, "status") == "False" {
			result = append(result, Finding{Code: "kubernetes.pod_not_ready", Severity: "warning", Message: fmt.Sprintf("Pod %s 尚未就绪", name)})
		}
	}
	for _, container := range arrayObjects(status, "containerStatuses") {
		containerName := stringField(container, "name")
		if numberField(container, "restartCount") > 0 {
			result = append(result, Finding{Code: "kubernetes.container_restarts", Severity: "warning", Message: fmt.Sprintf("Pod %s 的容器 %s 已重启", name, containerName)})
		}
		if ready, ok := container["ready"].(bool); ok && !ready {
			result = append(result, Finding{Code: "kubernetes.container_not_ready", Severity: "warning", Message: fmt.Sprintf("Pod %s 的容器 %s 未就绪", name, containerName)})
		}
		if waiting := objectField(objectField(container, "state"), "waiting"); waiting != nil && stringField(waiting, "reason") != "" {
			result = append(result, Finding{Code: "kubernetes.container_waiting", Severity: "warning", Message: fmt.Sprintf("Pod %s 的容器 %s 处于等待状态", name, containerName)})
		}
	}
	for _, container := range arrayObjects(spec, "containers") {
		resources := objectField(container, "resources")
		limits := objectField(resources, "limits")
		if limits == nil || (stringField(limits, "cpu") == "" && stringField(limits, "memory") == "") {
			result = append(result, Finding{Code: "kubernetes.missing_resource_limits", Severity: "info", Message: fmt.Sprintf("Pod %s 的容器 %s 未设置 CPU 或内存限制", name, stringField(container, "name"))})
		}
		if objectField(container, "readinessProbe") == nil && objectField(container, "livenessProbe") == nil {
			result = append(result, Finding{Code: "kubernetes.missing_probes", Severity: "info", Message: fmt.Sprintf("Pod %s 的容器 %s 未设置就绪或存活探针", name, stringField(container, "name"))})
		}
	}
	return result
}

func eventFindings(event map[string]any) []Finding {
	if stringField(event, "type") != "Warning" {
		return nil
	}
	return []Finding{{Code: "kubernetes.warning_event", Severity: "warning", Message: fmt.Sprintf("Kubernetes Warning 事件：%s", kubernetesName(event))}}
}

func deploymentFindings(item map[string]any) []Finding {
	desired := numberField(objectField(item, "spec"), "replicas")
	status := objectField(item, "status")
	available, updated := numberField(status, "availableReplicas"), numberField(status, "updatedReplicas")
	result := make([]Finding, 0, 2)
	if available < desired {
		result = append(result, Finding{Code: "kubernetes.deployment_unavailable", Severity: "warning", Message: fmt.Sprintf("Deployment %s 可用副本少于期望值", kubernetesName(item))})
	}
	if updated < desired {
		result = append(result, Finding{Code: "kubernetes.deployment_rollout_incomplete", Severity: "warning", Message: fmt.Sprintf("Deployment %s 发布尚未完成", kubernetesName(item))})
	}
	for _, condition := range arrayObjects(status, "conditions") {
		if stringField(condition, "type") == "Progressing" && stringField(condition, "status") == "False" {
			result = append(result, Finding{Code: "kubernetes.deployment_progressing_false", Severity: "warning", Message: fmt.Sprintf("Deployment %s 发布进度异常", kubernetesName(item))})
		}
	}
	return result
}

func daemonSetFindings(item map[string]any) []Finding {
	spec, status := objectField(item, "spec"), objectField(item, "status")
	desired := numberField(status, "desiredNumberScheduled")
	ready := numberField(status, "numberReady")
	updated := numberField(status, "updatedNumberScheduled")
	result := make([]Finding, 0, 2)
	if ready < desired {
		result = append(result, Finding{Code: "kubernetes.daemonset_not_ready", Severity: "warning", Message: fmt.Sprintf("DaemonSet %s 就绪节点少于期望值", kubernetesName(item))})
	}
	if updated < desired || (spec != nil && numberField(status, "numberAvailable") < desired) {
		result = append(result, Finding{Code: "kubernetes.daemonset_rollout_incomplete", Severity: "warning", Message: fmt.Sprintf("DaemonSet %s 发布尚未完成", kubernetesName(item))})
	}
	return result
}

func statefulSetFindings(item map[string]any) []Finding {
	desired := numberField(objectField(item, "spec"), "replicas")
	ready := numberField(objectField(item, "status"), "readyReplicas")
	if ready >= desired {
		return nil
	}
	return []Finding{{Code: "kubernetes.statefulset_not_ready", Severity: "warning", Message: fmt.Sprintf("StatefulSet %s 就绪副本少于期望值", kubernetesName(item))}}
}

func jobFindings(item map[string]any) []Finding {
	if numberField(objectField(item, "status"), "failed") <= 0 {
		return nil
	}
	return []Finding{{Code: "kubernetes.job_failed", Severity: "warning", Message: fmt.Sprintf("Job %s 存在失败执行", kubernetesName(item))}}
}

func uniqueFindings(items []Finding) []Finding {
	seen := make(map[string]bool, len(items))
	result := make([]Finding, 0, len(items))
	for _, item := range items {
		key := item.Code + "\x00" + item.Message
		if !seen[key] {
			seen[key] = true
			result = append(result, item)
		}
	}
	return result
}

func kubernetesName(item map[string]any) string {
	if name := stringField(objectField(item, "metadata"), "name"); name != "" {
		return name
	}
	return "<unknown>"
}
func objectField(item map[string]any, key string) map[string]any {
	value, _ := item[key].(map[string]any)
	return value
}
func arrayObjects(item map[string]any, key string) []map[string]any {
	values, _ := item[key].([]any)
	result := make([]map[string]any, 0, len(values))
	for _, value := range values {
		if object, ok := value.(map[string]any); ok {
			result = append(result, object)
		}
	}
	return result
}
func stringField(item map[string]any, key string) string {
	value, _ := item[key].(string)
	return strings.TrimSpace(value)
}
func numberField(item map[string]any, key string) int64 {
	switch value := item[key].(type) {
	case int64:
		return value
	case int:
		return int64(value)
	case float64:
		return int64(value)
	default:
		return 0
	}
}
