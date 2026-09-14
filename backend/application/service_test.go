package application

import (
	"context"
	"testing"
)

type testStore struct{}

func (testStore) Create(context.Context, CreateInput) (Application, error) { return Application{}, nil }
func (testStore) Import(context.Context, ImportInput) (Application, error) { return Application{}, nil }
func (testStore) Get(context.Context, string, string) (Application, error) { return Application{}, nil }
func (testStore) List(context.Context, string) ([]Application, error)      { return nil, nil }
func (testStore) Update(context.Context, string, string, UpdateInput) (Application, error) {
	return Application{}, nil
}
func (testStore) Delete(context.Context, string, string) error { return nil }
func (testStore) CreateInstance(context.Context, string, string, CreateInstanceInput) (Instance, error) {
	return Instance{}, nil
}
func (testStore) DeleteInstance(context.Context, string, string, string) error { return nil }
func (testStore) CreateDependency(context.Context, string, string, CreateDependencyInput) (Dependency, error) {
	return Dependency{}, nil
}
func (testStore) DeleteDependency(context.Context, string, string, string) error { return nil }
func (testStore) ContextResourceIDs(context.Context, string, string) ([]string, error) {
	return nil, nil
}
func (testStore) Workspace(context.Context, string) (Workspace, error) { return Workspace{}, nil }

func TestCreateValidatesProjectAndCode(t *testing.T) {
	s := NewService(testStore{})
	if _, err := s.Create(context.Background(), CreateInput{ProjectID: "p", Name: "Orders", Code: "Bad Code"}); err == nil {
		t.Fatal("invalid code accepted")
	}
	if _, err := s.Create(context.Background(), CreateInput{ProjectID: "p", Name: "Orders", Code: "orders-api"}); err != nil {
		t.Fatalf("valid application rejected: %v", err)
	}
}
