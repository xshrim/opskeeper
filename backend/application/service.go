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
	Import(context.Context, ImportInput) (Application, error)
	Get(context.Context, string, string) (Application, error)
	List(context.Context, string) ([]Application, error)
	Update(context.Context, string, string, UpdateInput) (Application, error)
	Delete(context.Context, string, string) error
	CreateInstance(context.Context, string, string, CreateInstanceInput) (Instance, error)
	DeleteInstance(context.Context, string, string, string) error
	CreateDependency(context.Context, string, string, CreateDependencyInput) (Dependency, error)
	DeleteDependency(context.Context, string, string, string) error
	ContextResourceIDs(context.Context, string, string) ([]string, error)
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
	in.ProjectID = strings.TrimSpace(in.ProjectID)
	in.Description = strings.TrimSpace(in.Description)
	in.ExternalUID = strings.TrimSpace(in.ExternalUID)
	if in.Icon == "" {
		in.Icon = "AppWindow"
	}
	if in.Source == "" {
		in.Source = "manual"
	}
	if in.Source != "manual" && in.Source != "kubernetes" {
		return Application{}, fmt.Errorf("%w: source must be manual or kubernetes", ErrInvalid)
	}
	if err := validateCreate(in); err != nil {
		return Application{}, fmt.Errorf("%w: project, name and valid code are required", err)
	}
	for index := range in.Instances {
		in.Instances[index].ApplicationID = ""
	}
	for index := range in.Dependencies {
		in.Dependencies[index].ApplicationID = ""
	}
	return s.store.Create(ctx, in)
}

func (s *Service) Import(ctx context.Context, in ImportInput) (Application, error) {
	in.ProjectID = strings.TrimSpace(in.ProjectID)
	in.Name = strings.TrimSpace(in.Name)
	in.Code = strings.TrimSpace(in.Code)
	in.Description = strings.TrimSpace(in.Description)
	in.ExternalUID = strings.TrimSpace(in.ExternalUID)
	if in.Icon == "" {
		in.Icon = "AppWindow"
	}
	if in.Source == "" {
		in.Source = "kubernetes"
	}
	if in.Source != "kubernetes" {
		return Application{}, fmt.Errorf("%w: imported applications must use kubernetes source", ErrInvalid)
	}
	if in.ExternalUID == "" {
		return Application{}, fmt.Errorf("%w: external_uid is required for imported applications", ErrInvalid)
	}
	if err := validateCreate(CreateInput{ProjectID: in.ProjectID, Name: in.Name, Code: in.Code}); err != nil {
		return Application{}, fmt.Errorf("%w: project, name and valid code are required", err)
	}
	return s.store.Import(ctx, in)
}
func (s *Service) Get(ctx context.Context, id string) (Application, error) {
	if strings.TrimSpace(id) == "" {
		return Application{}, ErrInvalid
	}
	return s.store.Get(ctx, "", id)
}
func (s *Service) List(ctx context.Context, pid string) ([]Application, error) {
	if strings.TrimSpace(pid) == "" {
		return nil, ErrInvalid
	}
	return s.store.List(ctx, pid)
}
func (s *Service) UpdateInProject(ctx context.Context, projectID, id string, in UpdateInput) (Application, error) {
	if strings.TrimSpace(projectID) == "" || strings.TrimSpace(id) == "" {
		return Application{}, ErrInvalid
	}
	if in.Name == nil && in.Description == nil && in.Icon == nil && in.Status == nil && in.Labels == nil {
		return Application{}, ErrInvalid
	}
	if in.Name != nil {
		value := strings.TrimSpace(*in.Name)
		if value == "" {
			return Application{}, ErrInvalid
		}
		in.Name = &value
	}
	return s.store.Update(ctx, strings.TrimSpace(projectID), strings.TrimSpace(id), in)
}
func (s *Service) DeleteInProject(ctx context.Context, projectID, id string) error {
	if strings.TrimSpace(projectID) == "" || strings.TrimSpace(id) == "" {
		return ErrInvalid
	}
	return s.store.Delete(ctx, strings.TrimSpace(projectID), strings.TrimSpace(id))
}
func (s *Service) GetInProject(ctx context.Context, projectID, id string) (Application, error) {
	if strings.TrimSpace(projectID) == "" || strings.TrimSpace(id) == "" {
		return Application{}, ErrInvalid
	}
	return s.store.Get(ctx, strings.TrimSpace(projectID), strings.TrimSpace(id))
}
func (s *Service) CreateInstanceInProject(ctx context.Context, projectID, applicationID string, in CreateInstanceInput) (Instance, error) {
	if strings.TrimSpace(projectID) == "" || strings.TrimSpace(applicationID) == "" {
		return Instance{}, ErrInvalid
	}
	if strings.TrimSpace(in.Name) == "" || strings.TrimSpace(in.RuntimeKind) == "" || strings.TrimSpace(in.TargetResourceID) == "" {
		return Instance{}, ErrInvalid
	}
	in.ApplicationID = applicationID
	return s.store.CreateInstance(ctx, strings.TrimSpace(projectID), strings.TrimSpace(applicationID), in)
}
func (s *Service) DeleteInstanceInProject(ctx context.Context, projectID, applicationID, id string) error {
	if strings.TrimSpace(projectID) == "" || strings.TrimSpace(applicationID) == "" || strings.TrimSpace(id) == "" {
		return ErrInvalid
	}
	return s.store.DeleteInstance(ctx, strings.TrimSpace(projectID), strings.TrimSpace(applicationID), strings.TrimSpace(id))
}
func (s *Service) CreateDependencyInProject(ctx context.Context, projectID, applicationID string, in CreateDependencyInput) (Dependency, error) {
	if strings.TrimSpace(projectID) == "" || strings.TrimSpace(applicationID) == "" {
		return Dependency{}, ErrInvalid
	}
	if strings.TrimSpace(in.TargetResourceID) == "" || strings.TrimSpace(in.DependencyKind) == "" {
		return Dependency{}, ErrInvalid
	}
	in.ApplicationID = applicationID
	return s.store.CreateDependency(ctx, strings.TrimSpace(projectID), strings.TrimSpace(applicationID), in)
}
func (s *Service) DeleteDependencyInProject(ctx context.Context, projectID, applicationID, id string) error {
	if strings.TrimSpace(projectID) == "" || strings.TrimSpace(applicationID) == "" || strings.TrimSpace(id) == "" {
		return ErrInvalid
	}
	return s.store.DeleteDependency(ctx, strings.TrimSpace(projectID), strings.TrimSpace(applicationID), strings.TrimSpace(id))
}
func (s *Service) ContextResourceIDs(ctx context.Context, applicationID, scopeID string) ([]string, error) {
	if strings.TrimSpace(applicationID) == "" || strings.TrimSpace(scopeID) == "" {
		return nil, ErrInvalid
	}
	return s.store.ContextResourceIDs(ctx, strings.TrimSpace(scopeID), strings.TrimSpace(applicationID))
}
func (s *Service) Workspace(ctx context.Context, pid string) (Workspace, error) {
	if pid == "" {
		return Workspace{}, ErrInvalid
	}
	return s.store.Workspace(ctx, pid)
}
