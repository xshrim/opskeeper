package connector

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	k8stesting "k8s.io/client-go/testing"
	"opskeeper/backend/resource"
)

type stubResourceReader struct {
	item resource.Resource
	err  error
}

func (r stubResourceReader) Get(context.Context, string) (resource.Resource, error) {
	return r.item, r.err
}

type stubCredentialReader struct {
	secret []byte
	err    error
}

func (r stubCredentialReader) RevealLinked(context.Context, string) ([]byte, error) {
	return append([]byte(nil), r.secret...), r.err
}

type memoryCheckStore struct {
	mu     sync.Mutex
	checks []Check
	err    error
}

func (s *memoryCheckStore) Save(_ context.Context, check Check) (Check, error) {
	if s.err != nil {
		return Check{}, s.err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	check.ID = "check-1"
	s.checks = append(s.checks, check)
	return check, nil
}

func (s *memoryCheckStore) Latest(context.Context, string) (Check, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.checks) == 0 {
		return Check{}, ErrNotFound
	}
	return s.checks[len(s.checks)-1], nil
}

func TestServiceConnectionTestPersistsSuccess(t *testing.T) {
	checks := &memoryCheckStore{}
	registry := NewRegistry()
	if err := registry.Register("Prometheus", 1, 1, func(target Target) (Adapter, error) {
		if string(target.Secret) != `{"token":"secret"}` {
			t.Fatalf("factory secret = %q", target.Secret)
		}
		return &stubAdapter{kind: "Prometheus", capabilities: []Capability{CapabilityQueryMetrics}}, nil
	}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	credentialID := "credential-1"
	service := NewService(registry, stubResourceReader{item: resource.Resource{
		ID: "resource-1", Kind: "Prometheus", SchemaVersion: 1, Status: resource.StatusActive, CredentialID: &credentialID,
	}}, stubCredentialReader{secret: []byte(`{"token":"secret"}`)}, checks, DefaultLimits())
	service.now = func() time.Time { return time.Date(2026, 8, 16, 12, 0, 0, 0, time.UTC) }

	check, err := service.Test(context.Background(), "actor-1", "resource-1")
	if err != nil {
		t.Fatalf("Test() error = %v", err)
	}
	if check.Status != "succeeded" || check.Message != "连接测试通过" || check.CheckedBy == nil || *check.CheckedBy != "actor-1" {
		t.Fatalf("Test() check = %#v", check)
	}
	if len(check.Capabilities) != 1 || check.Capabilities[0] != CapabilityQueryMetrics {
		t.Fatalf("Test() capabilities = %#v", check.Capabilities)
	}
	latest, err := service.Latest(context.Background(), "resource-1")
	if err != nil || latest.ID != check.ID {
		t.Fatalf("Latest() = %#v, %v", latest, err)
	}
}

func TestServiceConnectionTestPersistsSanitizedFailureAndRetriesTemporaryErrors(t *testing.T) {
	checks := &memoryCheckStore{}
	registry := NewRegistry()
	calls := 0
	if err := registry.Register("Prometheus", 1, 1, func(Target) (Adapter, error) {
		return &stubAdapter{kind: "Prometheus", test: func(context.Context) error {
			calls++
			return connectorError(CategoryUpstream, "query private.example", true, errors.New("secret response body"))
		}}, nil
	}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	limits := DefaultLimits()
	limits.Retries = 1
	service := NewService(registry, stubResourceReader{item: resource.Resource{
		ID: "resource-1", Kind: "Prometheus", SchemaVersion: 1, Status: resource.StatusActive,
	}}, nil, checks, limits)

	check, err := service.Test(context.Background(), "", "resource-1")
	if err != nil {
		t.Fatalf("Test() error = %v", err)
	}
	if calls != 2 {
		t.Fatalf("adapter Test() calls = %d, want 2", calls)
	}
	if check.Status != "failed" || check.ErrorCategory != CategoryUpstream || check.Message != "上游服务不可用或返回错误" {
		t.Fatalf("Test() check = %#v", check)
	}
	if check.CheckedBy != nil {
		t.Fatalf("Test() CheckedBy = %#v, want nil", check.CheckedBy)
	}
}

func TestServiceDoesNotRetryPermanentFailure(t *testing.T) {
	registry := NewRegistry()
	calls := 0
	if err := registry.Register("Prometheus", 1, 1, func(Target) (Adapter, error) {
		return &stubAdapter{test: func(context.Context) error {
			calls++
			return connectorError(CategoryAuthentication, "authenticate", false, errors.New("denied"))
		}}, nil
	}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	service := NewService(registry, stubResourceReader{item: resource.Resource{
		ID: "resource-1", Kind: "Prometheus", SchemaVersion: 1, Status: resource.StatusActive,
	}}, nil, &memoryCheckStore{}, DefaultLimits())
	if _, err := service.Test(context.Background(), "", "resource-1"); err != nil {
		t.Fatalf("Test() error = %v", err)
	}
	if calls != 1 {
		t.Fatalf("adapter Test() calls = %d, want 1", calls)
	}
}

func TestServiceRejectsConcurrentExecutionBeyondLimit(t *testing.T) {
	limits := DefaultLimits()
	limits.MaxConcurrent = 1
	service := NewService(NewRegistry(), nil, nil, nil, limits)
	started := make(chan struct{})
	release := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- service.execute(context.Background(), func(context.Context) error {
			close(started)
			<-release
			return nil
		})
	}()
	<-started
	err := service.execute(context.Background(), func(context.Context) error { return nil })
	if category, temporary := classify(err); category != CategoryRateLimited || !temporary {
		t.Fatalf("second execute() error = %v, category = %q, temporary = %v", err, category, temporary)
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatalf("first execute() error = %v", err)
	}
}

func TestServiceValidatesQueryLimitsBeforeResolvingResource(t *testing.T) {
	service := NewService(NewRegistry(), stubResourceReader{err: errors.New("resource should not be read")}, nil, nil, DefaultLimits())
	now := time.Now()
	tests := []struct {
		name string
		run  func() error
	}{
		{name: "metrics window", run: func() error {
			_, err := service.QueryMetrics(context.Background(), "resource-1", MetricsQuery{Query: "up", Start: now.Add(-25 * time.Hour), End: now, Step: time.Minute})
			return err
		}},
		{name: "metrics step", run: func() error {
			_, err := service.QueryMetrics(context.Background(), "resource-1", MetricsQuery{Query: "up", Start: now.Add(-time.Hour), End: now, Step: time.Second})
			return err
		}},
		{name: "log limit", run: func() error {
			_, err := service.QueryLogs(context.Background(), "resource-1", LogsQuery{Query: "{}", Start: now.Add(-time.Hour), End: now, Limit: 1001})
			return err
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.run(); !errors.Is(err, ErrInvalid) {
				t.Fatalf("query error = %v, want ErrInvalid", err)
			}
		})
	}
}

func TestKubernetesReaderRejectsSecrets(t *testing.T) {
	adapter := &kubernetesAdapter{}
	_, err := adapter.ReadKubernetes(context.Background(), KubernetesQuery{Resource: "secrets", Namespace: "default", Limit: 10})
	if category, _ := classify(err); category != CategoryUnsupported {
		t.Fatalf("ReadKubernetes(secrets) error = %v, category = %q", err, category)
	}
}

func TestKubernetesReaderMarksLimitedPageAsPartial(t *testing.T) {
	pods := allowedKubernetesResources["pods"].gvr
	client := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), map[schema.GroupVersionResource]string{pods: "PodList"})
	client.PrependReactor("list", "pods", func(k8stesting.Action) (bool, runtime.Object, error) {
		list := &unstructured.UnstructuredList{Items: []unstructured.Unstructured{{Object: map[string]any{
			"apiVersion": "v1", "kind": "Pod", "metadata": map[string]any{"name": "pod-a"},
		}}}}
		list.SetContinue("next-page")
		return true, list, nil
	})
	adapter := &kubernetesAdapter{dynamicClient: client}
	evidence, err := adapter.ReadKubernetes(context.Background(), KubernetesQuery{Resource: "pods", Namespace: "default", Limit: 1})
	if err != nil {
		t.Fatalf("ReadKubernetes() error = %v", err)
	}
	if !evidence.Partial || evidence.Summary["item_count"] != 1 {
		t.Fatalf("ReadKubernetes() evidence = %#v", evidence)
	}
}

func TestServiceReportsMissingDependenciesInsteadOfPanicking(t *testing.T) {
	service := NewService(nil, nil, nil, nil, DefaultLimits())
	if _, err := service.Test(context.Background(), "", "resource-1"); err == nil {
		t.Fatal("Test() error = nil")
	}
	if _, err := service.Latest(context.Background(), "resource-1"); err == nil {
		t.Fatal("Latest() error = nil")
	}
}

func TestServiceRejectsOversizedEvidenceFromAnyAdapter(t *testing.T) {
	limits := DefaultLimits()
	limits.MaxResponseBytes = 4
	service := NewService(NewRegistry(), nil, nil, nil, limits)
	_, err := service.collect(context.Background(), "resource-1", CapabilityKubernetesRead, func(context.Context) (Evidence, error) {
		return Evidence{Data: []byte("12345")}, nil
	})
	if category, _ := classify(err); category != CategoryResponseTooLarge {
		t.Fatalf("collect() error = %v, category = %q", err, category)
	}
}

func TestKubernetesErrorsUseConnectorCategories(t *testing.T) {
	tests := []struct {
		name      string
		err       error
		category  Category
		temporary bool
	}{
		{name: "forbidden", err: apierrors.NewForbidden(schema.GroupResource{Resource: "pods"}, "pod-a", errors.New("denied")), category: CategoryAuthentication},
		{name: "rate limited", err: apierrors.NewTooManyRequests("busy", 1), category: CategoryRateLimited, temporary: true},
		{name: "timeout", err: apierrors.NewTimeoutError("slow", 1), category: CategoryTimeout, temporary: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			category, temporary := classify(kubernetesError("read Kubernetes", test.err))
			if category != test.category || temporary != test.temporary {
				t.Fatalf("classify() = %q, %v; want %q, %v", category, temporary, test.category, test.temporary)
			}
		})
	}
}
