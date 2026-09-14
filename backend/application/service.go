package application

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

var ErrNotFound = errors.New("application not found")
var ErrInvalid = errors.New("invalid application")
var codePattern = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,62}[a-z0-9])?$`)

type Store interface {
	Create(context.Context, CreateInput) (Application, error)
	Get(context.Context, string) (Application, error)
	List(context.Context, string) ([]Application, error)
	Update(context.Context, string, UpdateInput) (Application, error)
	Delete(context.Context, string) error
	CreateInstance(context.Context, CreateInstanceInput) (Instance, error)
	DeleteInstance(context.Context, string) error
	CreateDependency(context.Context, CreateDependencyInput) (Dependency, error)
	DeleteDependency(context.Context, string) error
	Workspace(context.Context, string) (Workspace, error)
}

type Service struct{ store Store }

func NewService(store Store) *Service { return &Service{store: store} }
func validateCreate(in CreateInput) error {
	if strings.TrimSpace(in.ProjectID) == "" || strings.TrimSpace(in.Name) == "" || !codePattern.MatchString(strings.TrimSpace(in.Code)) {
		return ErrInvalid
	}
	return nil
}
func (s *Service) Create(ctx context.Context, in CreateInput) (Application, error) {
	in.Name = strings.TrimSpace(in.Name)
	in.Code = strings.TrimSpace(in.Code)
	if in.Icon == "" {
		in.Icon = "AppWindow"
	}
	if in.Source == "" {
		in.Source = "manual"
	}
	if err := validateCreate(in); err != nil {
		return Application{}, fmt.Errorf("%w: project, name and valid code are required", err)
	}
	return s.store.Create(ctx, in)
}
func (s *Service) Get(ctx context.Context, id string) (Application, error) {
	if strings.TrimSpace(id) == "" {
		return Application{}, ErrInvalid
	}
	return s.store.Get(ctx, id)
}
func (s *Service) List(ctx context.Context, pid string) ([]Application, error) {
	if strings.TrimSpace(pid) == "" {
		return nil, ErrInvalid
	}
	return s.store.List(ctx, pid)
}
func (s *Service) Update(ctx context.Context, id string, in UpdateInput) (Application, error) {
	if id == "" {
		return Application{}, ErrInvalid
	}
	if in.Name != nil {
		v := strings.TrimSpace(*in.Name)
		if v == "" {
			return Application{}, ErrInvalid
		}
		in.Name = &v
	}
	return s.store.Update(ctx, id, in)
}
func (s *Service) Delete(ctx context.Context, id string) error {
	if id == "" {
		return ErrInvalid
	}
	return s.store.Delete(ctx, id)
}
func (s *Service) CreateInstance(ctx context.Context, in CreateInstanceInput) (Instance, error) {
	if in.ApplicationID == "" || in.Name == "" || in.RuntimeKind == "" || in.TargetResourceID == "" {
		return Instance{}, ErrInvalid
	}
	return s.store.CreateInstance(ctx, in)
}
func (s *Service) DeleteInstance(ctx context.Context, id string) error {
	if id == "" {
		return ErrInvalid
	}
	return s.store.DeleteInstance(ctx, id)
}
func (s *Service) CreateDependency(ctx context.Context, in CreateDependencyInput) (Dependency, error) {
	if in.ApplicationID == "" || in.TargetResourceID == "" || in.DependencyKind == "" {
		return Dependency{}, ErrInvalid
	}
	return s.store.CreateDependency(ctx, in)
}
func (s *Service) DeleteDependency(ctx context.Context, id string) error {
	if id == "" {
		return ErrInvalid
	}
	return s.store.DeleteDependency(ctx, id)
}
func (s *Service) Workspace(ctx context.Context, pid string) (Workspace, error) {
	if pid == "" {
		return Workspace{}, ErrInvalid
	}
	return s.store.Workspace(ctx, pid)
}
