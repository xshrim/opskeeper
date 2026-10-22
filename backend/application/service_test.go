package application

import (
	"context"
	"testing"
)

type testStore struct{ input CreateInput }

func (s *testStore) Create(_ context.Context, input CreateInput) (Application, error) {
	s.input = input
	return Application{}, nil
}
func (*testStore) Get(context.Context, string, string) (Application, error) {
	return Application{}, nil
}
func (*testStore) List(context.Context, string) ([]Application, error) { return nil, nil }
func (*testStore) Update(context.Context, string, string, UpdateInput) (Application, error) {
	return Application{}, nil
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
	if _, err := s.Create(context.Background(), CreateInput{ProjectID: "p", Name: "Orders", Code: "Bad Code"}); err == nil {
		t.Fatal("invalid code accepted")
	}
	if _, err := s.Create(context.Background(), CreateInput{ProjectID: "p", Name: "Orders", Code: "orders-api", RuntimeKind: RuntimeHost, Instances: []CreateInstanceInput{{Name: "primary", RuntimeKind: RuntimeHost, TargetResourceID: "host-1", Selector: map[string]any{"process_keyword": "orders"}}}}); err != nil {
		t.Fatalf("valid application rejected: %v", err)
	}
}

func TestCreatePreservesResponsibleUserOrder(t *testing.T) {
	store := &testStore{}
	s := NewService(store)
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
