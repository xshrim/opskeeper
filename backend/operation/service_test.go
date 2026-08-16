package operation

import (
	"context"
	"testing"
	"time"

	"opskeeper/backend/authorization"
	"opskeeper/backend/resource"
)

type memoryStore struct {
	item      Request
	execution string
}

func (s *memoryStore) Create(_ context.Context, item Request) (Request, error) {
	item.ID = "request"
	s.item = item
	return item, nil
}
func (s *memoryStore) Get(_ context.Context, _ string) (Request, error) { return s.item, nil }
func (s *memoryStore) List(context.Context, string, int) ([]Request, error) {
	return []Request{s.item}, nil
}
func (s *memoryStore) Approve(_ context.Context, a Approval) (Request, error) {
	s.item.Status = Approved
	return s.item, nil
}
func (s *memoryStore) StartExecution(context.Context, string, string, time.Time) (string, error) {
	s.execution = "execution"
	return s.execution, nil
}
func (s *memoryStore) GetExecution(context.Context, string) (Execution, error) {
	return Execution{ID: "execution", OperationRequestID: s.item.ID}, nil
}

type memoryResources struct{ item resource.Resource }

func (s memoryResources) Get(context.Context, string) (resource.Resource, error) { return s.item, nil }

func TestRequestRejectsResourceOutsideScopeFilter(t *testing.T) {
	store := &memoryStore{}
	service := NewService(store, memoryResources{item: resource.Resource{ID: "resource", ScopeID: "scope", Status: resource.StatusActive}})
	ctx := authorization.WithResourceFilter(context.Background(), authorization.ResourceFilter{ScopeIDs: []string{"other"}})
	_, err := service.Request(ctx, Request{ScopeID: "scope", TargetResourceID: "resource", RequestedBy: "user", OperationName: RestartWorkload, RiskLevel: RiskMedium, Parameters: map[string]any{}, IdempotencyKey: "key"})
	if err != authorization.ErrForbidden {
		t.Fatalf("error=%v, want forbidden", err)
	}
}

func TestRequestAndStartLowRiskWithinScope(t *testing.T) {
	store := &memoryStore{}
	service := NewService(store, memoryResources{item: resource.Resource{ID: "resource", ScopeID: "scope", Status: resource.StatusActive}})
	ctx := authorization.WithResourceFilter(context.Background(), authorization.ResourceFilter{ScopeIDs: []string{"scope"}})
	item, err := service.Request(ctx, Request{ScopeID: "scope", TargetResourceID: "resource", RequestedBy: "user", OperationName: RestartWorkload, RiskLevel: RiskLow, Parameters: map[string]any{}, IdempotencyKey: "key"})
	if err != nil || item.Status != Approved {
		t.Fatalf("request=%#v error=%v", item, err)
	}
	if item.DryRun["parameters_hash"] != item.ParametersHash {
		t.Fatalf("dry-run plan was not bound to exact parameters: %#v", item.DryRun)
	}
	if _, err = service.Start(ctx, item.ID, "execution-key"); err != nil {
		t.Fatalf("start low risk: %v", err)
	}
}

func TestRequestRejectsOperationOutsideCatalog(t *testing.T) {
	store := &memoryStore{}
	service := NewService(store, memoryResources{item: resource.Resource{ID: "resource", ScopeID: "scope", Status: resource.StatusActive}})
	_, err := service.Request(context.Background(), Request{ScopeID: "scope", TargetResourceID: "resource", RequestedBy: "user", OperationName: "shell.exec", RiskLevel: RiskHigh, Parameters: map[string]any{}, IdempotencyKey: "key"})
	if err == nil {
		t.Fatal("arbitrary operation was accepted")
	}
}
