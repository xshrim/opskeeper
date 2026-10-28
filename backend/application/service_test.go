package application

import (
	"context"
	"errors"
	"fmt"
	"testing"
)

type testStore struct {
	input  CreateInput
	app    Application
	update UpdateInput
}
type testBindingValidator struct{ err error }

func (v testBindingValidator) ValidateApplicationBinding(context.Context, string, string, map[string]any) error {
	return v.err
}

func (s *testStore) Create(_ context.Context, input CreateInput) (Application, error) {
	s.input = input
	return Application{}, nil
}
func (s *testStore) Get(context.Context, string, string) (Application, error) {
	return s.app, nil
}
func (*testStore) List(context.Context, string) ([]Application, error) { return nil, nil }
func (s *testStore) Update(_ context.Context, _ string, _ string, input UpdateInput) (Application, error) {
	s.update = input
	return s.app, nil
}
func (*testStore) Delete(context.Context, string, string) error { return nil }
func (*testStore) CreateInstance(context.Context, string, string, CreateInstanceInput) (Instance, error) {
	return Instance{}, nil
}
func (*testStore) DeleteInstance(context.Context, string, string, string) error { return nil }
func (*testStore) ContextResourceIDs(context.Context, string, string) ([]string, error) {
	return nil, nil
}
func (*testStore) Workspace(context.Context, string) (Workspace, error) { return Workspace{}, nil }

func TestCreateValidatesProjectAndCode(t *testing.T) {
	s := NewService(&testStore{})
	s.SetBindingValidator(testBindingValidator{})
	if _, err := s.Create(context.Background(), CreateInput{ProjectID: "p", Name: "Orders", Code: "Bad Code"}); err == nil {
		t.Fatal("invalid code accepted")
	}
	if _, err := s.Create(context.Background(), CreateInput{ProjectID: "p", Name: "Orders", Code: "orders-api", RuntimeKind: RuntimeHost, Instances: []CreateInstanceInput{{Name: "primary", RuntimeKind: RuntimeHost, TargetResourceID: "host-1", Selector: map[string]any{"process_keyword": "orders"}}}}); err != nil {
		t.Fatalf("valid application rejected: %v", err)
	}
}

func TestCreateAllowsApplicationWithoutInstances(t *testing.T) {
	for _, runtimeKind := range []string{RuntimeHost, RuntimeDocker, RuntimeKubernetes} {
		store := &testStore{}
		s := NewService(store)
		if _, err := s.Create(context.Background(), CreateInput{ProjectID: "p", Name: "Orders", Code: "orders-api", RuntimeKind: runtimeKind}); err != nil {
			t.Fatalf("Create(%s) without instances error = %v", runtimeKind, err)
		}
		if len(store.input.Instances) != 0 {
			t.Fatalf("Create(%s) instances = %#v, want none", runtimeKind, store.input.Instances)
		}
	}
}

func TestCreatePreservesResponsibleUserOrder(t *testing.T) {
	store := &testStore{}
	s := NewService(store)
	s.SetBindingValidator(testBindingValidator{})
	first := "11111111-1111-4111-8111-111111111111"
	second := "22222222-2222-4222-8222-222222222222"
	_, err := s.Create(context.Background(), CreateInput{
		ProjectID:          "p",
		Name:               "Orders",
		Code:               "orders-api",
		RuntimeKind:        RuntimeHost,
		TechnicalOwnerIDs:  []string{first, second},
		OperationsOwnerIDs: []string{second, first},
		Instances:          []CreateInstanceInput{{Name: "primary", RuntimeKind: RuntimeHost, TargetResourceID: "host-1", Selector: map[string]any{"process_keyword": "orders"}}},
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if got := store.input.TechnicalOwnerIDs; len(got) != 2 || got[0] != first || got[1] != second {
		t.Fatalf("technical owners = %#v, want ordered primary then backup", got)
	}
}

func TestCreateRejectsInvalidRuntimeBinding(t *testing.T) {
	s := NewService(&testStore{})
	s.SetBindingValidator(testBindingValidator{err: fmt.Errorf("found 2 matches")})
	_, err := s.Create(context.Background(), CreateInput{
		ProjectID: "p", Name: "Orders", Code: "orders-api", RuntimeKind: RuntimeHost,
		Instances: []CreateInstanceInput{{Name: "primary", RuntimeKind: RuntimeHost, TargetResourceID: "host-1", Selector: map[string]any{"process_keyword": "orders"}}},
	})
	if err == nil || !errors.Is(err, ErrInvalid) {
		t.Fatalf("Create() error = %v, want invalid binding", err)
	}
}

func TestUpdateAllowsReplacingInstanceBindings(t *testing.T) {
	store := &testStore{app: Application{ID: "application-1", RuntimeKind: RuntimeHost}}
	s := NewService(store)
	s.SetBindingValidator(testBindingValidator{})
	instances := []CreateInstanceInput{{Name: "primary", TargetResourceID: "host-2", Selector: map[string]any{"process_keyword": "orders-api"}}}
	if _, err := s.UpdateInProject(context.Background(), "project-1", "application-1", UpdateInput{Instances: &instances}); err != nil {
		t.Fatalf("UpdateInProject() error = %v", err)
	}
	if store.update.Instances == nil || len(*store.update.Instances) != 1 || (*store.update.Instances)[0].RuntimeKind != RuntimeHost {
		t.Fatalf("updated instances = %#v, want normalized Host binding", store.update.Instances)
	}
}

func TestUpdateAllowsRemovingAllInstancesAndChangingRuntime(t *testing.T) {
	store := &testStore{app: Application{ID: "application-1", RuntimeKind: RuntimeHost, Instances: []Instance{{ID: "instance-1"}}}}
	s := NewService(store)
	runtimeKind := RuntimeDocker
	instances := []CreateInstanceInput{}
	if _, err := s.UpdateInProject(context.Background(), "project-1", "application-1", UpdateInput{RuntimeKind: &runtimeKind, Instances: &instances}); err != nil {
		t.Fatalf("UpdateInProject() error = %v", err)
	}
	if store.update.RuntimeKind == nil || *store.update.RuntimeKind != RuntimeDocker || store.update.Instances == nil || len(*store.update.Instances) != 0 {
		t.Fatalf("UpdateInProject() update = %#v, want Docker runtime and no instances", store.update)
	}
}

func TestUpdateRejectsChangingRuntimeWhileInstancesRemain(t *testing.T) {
	store := &testStore{app: Application{ID: "application-1", RuntimeKind: RuntimeHost, Instances: []Instance{{ID: "instance-1"}}}}
	s := NewService(store)
	runtimeKind := RuntimeDocker
	instances := []CreateInstanceInput{{Name: "primary", TargetResourceID: "docker-1", Selector: map[string]any{"container_name": "orders"}}}
	if _, err := s.UpdateInProject(context.Background(), "project-1", "application-1", UpdateInput{RuntimeKind: &runtimeKind, Instances: &instances}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("UpdateInProject() error = %v, want invalid runtime change", err)
	}
}
