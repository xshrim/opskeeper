package operation

import (
	"context"
	"testing"

	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"

	"opskeeper/backend/resource"
)

type resourceMap map[string]resource.Resource

func (items resourceMap) Get(_ context.Context, id string) (resource.Resource, error) {
	item, ok := items[id]
	if !ok {
		return resource.Resource{}, resource.ErrNotFound
	}
	return item, nil
}

func TestKubernetesSubmitterCreatesFixedOperationJob(t *testing.T) {
	client := fake.NewSimpleClientset()
	resources := resourceMap{
		"application": {ID: "application", Kind: "Application", Name: "payments", SourceResourceID: "cluster", Config: map[string]any{"namespace": "payments", "kubernetes": map[string]any{"workload_kind": "Deployment", "workload_name": "payments-api"}}},
		"cluster":     {ID: "cluster", Kind: "Kubernetes"},
	}
	submitter := &KubernetesSubmitter{Resources: resources, Image: "opskeeper:test", Factory: func(context.Context, resource.Resource) (kubernetes.Interface, error) { return client, nil }}
	request := Request{TargetResourceID: "application", OperationName: ScaleWorkload, Status: Executing, Parameters: map[string]any{"replicas": float64(3)}}
	ref, err := submitter.Submit(context.Background(), request, "01234567-89ab-cdef")
	if err != nil {
		t.Fatal(err)
	}
	if ref != "payments/opskeeper-operation-0123456789ab" {
		t.Fatalf("job reference=%q", ref)
	}
	job, err := client.BatchV1().Jobs("payments").Get(context.Background(), "opskeeper-operation-0123456789ab", metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	container := job.Spec.Template.Spec.Containers[0]
	if container.Command[0] != "/opskeeper-operation-runner" || container.Command[len(container.Command)-1] != "3" {
		t.Fatalf("runner command=%#v", container.Command)
	}
	if !*job.Spec.Template.Spec.AutomountServiceAccountToken || job.Spec.Template.Spec.ServiceAccountName != "opskeeper-operation-runner" {
		t.Fatalf("operation service account is not configured: %#v", job.Spec.Template.Spec)
	}
}

type reconcileStore struct {
	items     []Execution
	completed bool
	succeeded bool
}

func (s *reconcileStore) ListActiveExecutions(context.Context, int) ([]Execution, error) {
	return s.items, nil
}
func (s *reconcileStore) CompleteExecution(_ context.Context, _ string, succeeded bool, _ map[string]any, _ string) error {
	s.completed, s.succeeded = true, succeeded
	return nil
}

func TestReconcilerCompletesSuccessfulJob(t *testing.T) {
	client := fake.NewSimpleClientset(&batchv1.Job{ObjectMeta: metav1.ObjectMeta{Name: "job", Namespace: "payments"}, Status: batchv1.JobStatus{Conditions: []batchv1.JobCondition{{Type: batchv1.JobComplete, Status: "True"}}}})
	store := &reconcileStore{items: []Execution{{ID: "execution", Result: map[string]any{"job_name": "payments/job"}}}}
	count, err := (&Reconciler{Store: store, Client: client}).RunOnce(context.Background())
	if err != nil || count != 1 || !store.completed || !store.succeeded {
		t.Fatalf("reconcile count=%d completed=%v succeeded=%v err=%v", count, store.completed, store.succeeded, err)
	}
}
