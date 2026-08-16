package authorization

import (
	"context"
	"strings"
)

type Store interface {
	ResolveScopes(context.Context, Subject, Permission) (ScopeFilter, error)
	ResolveResourceAccess(context.Context, Subject, Permission) (ResourceFilter, error)
	EnsureBootstrapAdmin(context.Context, string) error
}

type Service struct {
	store Store
}

func NewService(store Store) *Service {
	return &Service{store: store}
}

func (s *Service) ScopeFilter(ctx context.Context, subject Subject, permission Permission) (ScopeFilter, error) {
	if strings.TrimSpace(subject.UserID) == "" || strings.TrimSpace(string(permission)) == "" {
		return ScopeFilter{}, ErrInvalidSubject
	}
	return s.store.ResolveScopes(ctx, subject, permission)
}

func (s *Service) ResourceFilter(ctx context.Context, subject Subject, permission Permission) (ResourceFilter, error) {
	if strings.TrimSpace(subject.UserID) == "" || strings.TrimSpace(string(permission)) == "" {
		return ResourceFilter{}, ErrInvalidSubject
	}
	return s.store.ResolveResourceAccess(ctx, subject, permission)
}

func (s *Service) Authorize(ctx context.Context, subject Subject, permission Permission, targetScopeID string) error {
	filter, err := s.ScopeFilter(ctx, subject, permission)
	if err != nil {
		return err
	}
	if !filter.Allows(targetScopeID) {
		return ErrForbidden
	}
	return nil
}

func (s *Service) EnsureBootstrapAdmin(ctx context.Context, userID string) error {
	if strings.TrimSpace(userID) == "" {
		return ErrInvalidSubject
	}
	return s.store.EnsureBootstrapAdmin(ctx, userID)
}
