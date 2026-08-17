package operation

import (
	"fmt"
	"strings"

	"opskeeper/backend/sandbox"
)

const (
	RestartWorkload = "kubernetes.restart_workload"
	ScaleWorkload   = "kubernetes.scale_workload"
)

// BuildSandboxJob preserves the minimal declarative envelope used by sandbox
// policy tests. Production operation submission uses BuildOperationJob below.
func BuildSandboxJob(request Request, executionID, namespace, image string) (map[string]any, error) {
	if request.OperationName != RestartWorkload && request.OperationName != ScaleWorkload {
		return nil, fmt.Errorf("operation is not supported by the sandbox executor")
	}
	if request.Status != Executing || strings.TrimSpace(executionID) == "" {
		return nil, ErrApprovalRequired
	}
	shortID := strings.ReplaceAll(executionID, "-", "")
	if len(shortID) < 12 {
		return nil, fmt.Errorf("execution ID is invalid")
	}
	name := "opskeeper-operation-" + shortID[:12]
	job, err := sandbox.BuildJob(sandbox.JobSpec{
		Name: name, Namespace: namespace, Image: image,
		// This is a fixed runner entrypoint, not user-supplied code.
		Command: []string{"/opskeeper-operation-runner", "--execution-id", executionID},
	})
	if err != nil {
		return nil, err
	}
	metadata := job["metadata"].(map[string]any)
	labels := metadata["labels"].(map[string]string)
	labels["opskeeper.io/operation"] = request.OperationName
	return job, nil
}

// BuildOperationJob is the only operation-specific Job envelope. Parameters
// are converted into a fixed runner argument list; user input can never become
// a shell command or an image/entrypoint.
func BuildOperationJob(request Request, executionID, namespace, image, workloadKind, workload string, replicas *int64) (map[string]any, error) {
	if request.OperationName != RestartWorkload && request.OperationName != ScaleWorkload {
		return nil, fmt.Errorf("operation is not supported by the operation runner")
	}
	if request.Status != Executing || strings.TrimSpace(executionID) == "" || strings.TrimSpace(workload) == "" {
		return nil, ErrApprovalRequired
	}
	if workloadKind == "" {
		workloadKind = "Deployment"
	}
	if workloadKind != "Deployment" && workloadKind != "StatefulSet" && workloadKind != "DaemonSet" {
		return nil, fmt.Errorf("workload kind is not supported")
	}
	shortID := strings.ReplaceAll(executionID, "-", "")
	if len(shortID) < 12 {
		return nil, fmt.Errorf("execution ID is invalid")
	}
	args := []string{"/opskeeper-operation-runner", "--execution-id", executionID, "--operation", request.OperationName, "--kind", workloadKind, "--namespace", namespace, "--workload", workload}
	if request.OperationName == ScaleWorkload {
		if replicas == nil || *replicas < 0 || *replicas > 100 {
			return nil, fmt.Errorf("replicas must be between 0 and 100")
		}
		args = append(args, "--replicas", fmt.Sprintf("%d", *replicas))
	}
	job, err := sandbox.BuildJob(sandbox.JobSpec{
		Name: "opskeeper-operation-" + shortID[:12], Namespace: namespace, Image: image,
		ServiceAccount: "opskeeper-operation-runner", Command: args,
		AutomountServiceAccountToken: true,
	})
	if err != nil {
		return nil, err
	}
	labels := job["metadata"].(map[string]any)["labels"].(map[string]string)
	// The fixed operation runner needs Kubernetes API egress and therefore is
	// deliberately not selected by the custom-code sandbox deny-all policy.
	labels["opskeeper.io/sandbox"] = "false"
	labels["opskeeper.io/operation-runner"] = "true"
	return job, nil
}
