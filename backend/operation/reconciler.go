package operation

import (
	"context"
	"fmt"
	"strings"

	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type executionReconcileStore interface {
	ListActiveExecutions(context.Context, int) ([]Execution, error)
	CompleteExecution(context.Context, string, bool, map[string]any, string) error
}

type Reconciler struct {
	Store  executionReconcileStore
	Client kubernetes.Interface
}

func NewInClusterReconciler(store executionReconcileStore) (*Reconciler, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}
	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	return &Reconciler{Store: store, Client: client}, nil
}

func (r *Reconciler) RunOnce(ctx context.Context) (int, error) {
	if r == nil || r.Store == nil || r.Client == nil {
		return 0, fmt.Errorf("operation reconciler is not configured")
	}
	items, err := r.Store.ListActiveExecutions(ctx, 20)
	if err != nil {
		return 0, err
	}
	completed := 0
	for _, item := range items {
		jobRef, _ := item.Result["job_name"].(string)
		parts := strings.SplitN(jobRef, "/", 2)
		if len(parts) != 2 {
			if err := r.Store.CompleteExecution(ctx, item.ID, false, map[string]any{}, "invalid Kubernetes Job reference"); err != nil {
				return completed, err
			}
			completed++
			continue
		}
		job, getErr := r.Client.BatchV1().Jobs(parts[0]).Get(ctx, parts[1], metav1.GetOptions{})
		if getErr != nil {
			return completed, getErr
		}
		succeeded, failed, message := jobOutcome(job)
		if !succeeded && !failed {
			continue
		}
		result := map[string]any{"job_name": jobRef, "succeeded": job.Status.Succeeded, "failed": job.Status.Failed}
		if err := r.Store.CompleteExecution(ctx, item.ID, succeeded, result, message); err != nil {
			return completed, err
		}
		completed++
	}
	return completed, nil
}

func jobOutcome(job *batchv1.Job) (bool, bool, string) {
	for _, condition := range job.Status.Conditions {
		if condition.Status != "True" {
			continue
		}
		if condition.Type == batchv1.JobComplete {
			return true, false, condition.Message
		}
		if condition.Type == batchv1.JobFailed {
			return false, true, condition.Message
		}
	}
	return false, false, ""
}
