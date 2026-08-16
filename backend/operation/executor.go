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

// BuildSandboxJob creates a declarative envelope only. It never executes a
// process and deliberately has no os/exec dependency. A future Kubernetes
// submission component may submit this object only after Start has atomically
// created an approved execution record.
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
		// This is a fixed runner entrypoint, not user-supplied code. The runner
		// fetches the immutable request by ID and revalidates its hash.
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
