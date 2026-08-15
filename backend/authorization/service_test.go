package authorization

import (
	"context"
	"errors"
	"testing"
)

type stubStore struct {
	filter ScopeFilter
	err    error
}

func (s *stubStore) ResolveScopes(context.Context, Subject, Permission) (ScopeFilter, error) {
	return s.filter, s.err
}

func (s *stubStore) EnsureBootstrapAdmin(context.Context, string) error { return nil }

func TestAuthorizeUsesServerScopeFilter(t *testing.T) {
	service := NewService(&stubStore{filter: ScopeFilter{SubjectID: "user-1", Permission: OrganizationRead, ScopeIDs: []string{"scope-1", "scope-2"}}})
	if err := service.Authorize(context.Background(), Subject{UserID: "user-1"}, OrganizationRead, "scope-2"); err != nil {
		t.Fatalf("Authorize(allowed) error = %v", err)
	}
	if err := service.Authorize(context.Background(), Subject{UserID: "user-1"}, OrganizationRead, "scope-3"); !errors.Is(err, ErrForbidden) {
		t.Fatalf("Authorize(denied) error = %v, want ErrForbidden", err)
	}
}

func TestScopeFilterRejectsMissingSubjectAndPermission(t *testing.T) {
	service := NewService(&stubStore{})
	if _, err := service.ScopeFilter(context.Background(), Subject{}, OrganizationRead); !errors.Is(err, ErrInvalidSubject) {
		t.Fatalf("ScopeFilter(missing subject) error = %v", err)
	}
	if _, err := service.ScopeFilter(context.Background(), Subject{UserID: "user-1"}, ""); !errors.Is(err, ErrInvalidSubject) {
		t.Fatalf("ScopeFilter(missing permission) error = %v", err)
	}
}

func TestScopeFilterPropagatesStoreError(t *testing.T) {
	want := errors.New("database unavailable")
	service := NewService(&stubStore{err: want})
	_, err := service.ScopeFilter(context.Background(), Subject{UserID: "user-1"}, OrganizationRead)
	if !errors.Is(err, want) {
		t.Fatalf("ScopeFilter() error = %v, want %v", err, want)
	}
}
