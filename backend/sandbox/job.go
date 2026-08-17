// Package sandbox builds the only permitted execution envelope for custom
// code. It does not execute processes: callers submit the returned Job to a
// Kubernetes control plane after approval.
package sandbox

import "fmt"

type JobSpec struct {
	Name, Namespace, Image, ServiceAccount string
	Command                                []string
	CPU, Memory                            string
	AutomountServiceAccountToken           bool
}

func BuildJob(input JobSpec) (map[string]any, error) {
	if input.Name == "" || input.Namespace == "" || input.Image == "" || len(input.Command) == 0 {
		return nil, fmt.Errorf("name, namespace, image and command are required")
	}
	if input.ServiceAccount == "" {
		input.ServiceAccount = "opskeeper-sandbox"
	}
	if input.CPU == "" {
		input.CPU = "500m"
	}
	if input.Memory == "" {
		input.Memory = "512Mi"
	}
	container := map[string]any{
		"name": "runner", "image": input.Image, "command": input.Command,
		"resources":       map[string]any{"limits": map[string]string{"cpu": input.CPU, "memory": input.Memory}, "requests": map[string]string{"cpu": input.CPU, "memory": input.Memory}},
		"securityContext": map[string]any{"allowPrivilegeEscalation": false, "readOnlyRootFilesystem": true, "privileged": false, "capabilities": map[string]any{"drop": []string{"ALL"}}},
	}
	pod := map[string]any{"serviceAccountName": input.ServiceAccount, "automountServiceAccountToken": input.AutomountServiceAccountToken, "restartPolicy": "Never", "securityContext": map[string]any{"runAsNonRoot": true, "seccompProfile": map[string]string{"type": "RuntimeDefault"}}, "containers": []any{container}}
	return map[string]any{"apiVersion": "batch/v1", "kind": "Job", "metadata": map[string]any{"name": input.Name, "namespace": input.Namespace, "labels": map[string]string{"app.kubernetes.io/managed-by": "opskeeper", "opskeeper.io/sandbox": "true"}}, "spec": map[string]any{"backoffLimit": 0, "ttlSecondsAfterFinished": 300, "template": map[string]any{"spec": pod}}}, nil
}
