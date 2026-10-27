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
var uuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[1-5][0-9a-fA-F]{3}-[89abAB][0-9a-fA-F]{3}-[0-9a-fA-F]{12}$`)

const (
	RuntimeHost       = "virtual_machine"
	RuntimeDocker     = "containerized"
	RuntimeKubernetes = "cloud_native"
)

const maxIconLength = 262144

type Store interface {
	Create(context.Context, CreateInput) (Application, error)
	Get(context.Context, string, string) (Application, error)
	List(context.Context, string) ([]Application, error)
	Update(context.Context, string, string, UpdateInput) (Application, error)
	Delete(context.Context, string, string) error
	CreateInstance(context.Context, string, string, CreateInstanceInput) (Instance, error)
	DeleteInstance(context.Context, string, string, string) error
	ContextResourceIDs(context.Context, string, string) ([]string, error)
	Workspace(context.Context, string) (Workspace, error)
}

type BindingValidator interface {
	ValidateApplicationBinding(context.Context, string, string, map[string]any) error
}

type Service struct {
	store            Store
	bindingValidator BindingValidator
}

func NewService(store Store) *Service { return &Service{store: store} }
func (s *Service) SetBindingValidator(validator BindingValidator) {
	s.bindingValidator = validator
}
func validateCreate(in CreateInput) error {
	if strings.TrimSpace(in.Name) == "" || !codePattern.MatchString(strings.TrimSpace(in.Code)) {
		return ErrInvalid
	}
	if !validRuntimeKind(in.RuntimeKind) || (in.RuntimeKind != RuntimeKubernetes && len(in.Instances) == 0) {
		return ErrInvalid
	}
	seen := make(map[string]struct{}, len(in.Instances))
	for _, instance := range in.Instances {
		if strings.TrimSpace(instance.Name) == "" || strings.TrimSpace(instance.TargetResourceID) == "" || instance.RuntimeKind != in.RuntimeKind {
			return ErrInvalid
		}
		if _, ok := seen[instance.Name]; ok {
			return ErrInvalid
		}
		seen[instance.Name] = struct{}{}
		if !validSelector(in.RuntimeKind, instance.Selector) {
			return ErrInvalid
		}
	}
	return nil
}

func validRuntimeKind(value string) bool {
	return value == RuntimeHost || value == RuntimeDocker || value == RuntimeKubernetes
}

func normalizeOwnerIDs(ids []string) ([]string, error) {
	if len(ids) > 20 {
		return nil, ErrInvalid
	}
	seen := make(map[string]struct{}, len(ids))
	result := make([]string, 0, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if !uuidPattern.MatchString(id) {
			return nil, ErrInvalid
		}
		if _, exists := seen[id]; exists {
			return nil, ErrInvalid
		}
		seen[id] = struct{}{}
		result = append(result, id)
	}
	return result, nil
}

func validSelector(kind string, selector map[string]any) bool {
	if selector == nil {
		return false
	}
	required := map[string]string{RuntimeHost: "process_keyword", RuntimeDocker: "container_name", RuntimeKubernetes: "workload_name"}
	value, ok := selector[required[kind]].(string)
	if !ok || strings.TrimSpace(value) == "" {
		return false
	}
	if kind == RuntimeKubernetes {
		for _, field := range []string{"namespace", "workload_kind"} {
			value, ok := selector[field].(string)
			if !ok || strings.TrimSpace(value) == "" {
				return false
			}
		}
	}
	return true
}
func (s *Service) Create(ctx context.Context, in CreateInput) (Application, error) {
	if strings.TrimSpace(in.ProjectID) == "" {
		return Application{}, ErrInvalid
	}
	var err error
	in, err = PrepareCreateInput(in)
	if err != nil {
		return Application{}, err
	}
	for _, instance := range in.Instances {
		if err := s.validateBinding(ctx, instance.RuntimeKind, instance.TargetResourceID, instance.Selector); err != nil {
			return Application{}, err
		}
	}
	return s.store.Create(ctx, in)
}

func (s *Service) validateBinding(ctx context.Context, runtimeKind, resourceID string, selector map[string]any) error {
	if runtimeKind == RuntimeKubernetes {
		return nil
	}
	if s.bindingValidator == nil {
		return fmt.Errorf("%w: application binding validation is unavailable", ErrInvalid)
	}
	if err := s.bindingValidator.ValidateApplicationBinding(ctx, runtimeKind, resourceID, selector); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalid, err)
	}
	return nil
}

func PrepareCreateInput(in CreateInput) (CreateInput, error) {
	in.Name = strings.TrimSpace(in.Name)
	in.Code = strings.TrimSpace(in.Code)
	in.ProjectID = strings.TrimSpace(in.ProjectID)
	in.Description = strings.TrimSpace(in.Description)
	in.ExternalUID = strings.TrimSpace(in.ExternalUID)
	in.RuntimeKind = strings.TrimSpace(in.RuntimeKind)
	in.Icon = strings.TrimSpace(in.Icon)
	var err error
	if in.TechnicalOwnerIDs, err = normalizeOwnerIDs(in.TechnicalOwnerIDs); err != nil {
		return CreateInput{}, err
	}
	if in.OperationsOwnerIDs, err = normalizeOwnerIDs(in.OperationsOwnerIDs); err != nil {
		return CreateInput{}, err
	}
	if in.Icon == "" || len([]rune(in.Icon)) > maxIconLength {
		in.Icon = "lucide:AppWindow"
	}
	if err := validateCreate(in); err != nil {
		return CreateInput{}, fmt.Errorf("%w: project, name, valid code, runtime kind and instance bindings are required", err)
	}
	for index := range in.Instances {
		in.Instances[index].ApplicationID = ""
	}
	return in, nil
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
	if in.Name == nil && in.Description == nil && in.Icon == nil && in.Status == nil && in.Labels == nil && in.Critical == nil && in.TechnicalOwnerIDs == nil && in.OperationsOwnerIDs == nil && in.Instances == nil {
		return Application{}, ErrInvalid
	}
	if in.Name != nil {
		value := strings.TrimSpace(*in.Name)
		if value == "" {
			return Application{}, ErrInvalid
		}
		in.Name = &value
	}
	if in.Icon != nil {
		value := strings.TrimSpace(*in.Icon)
		if value == "" || len([]rune(value)) > maxIconLength {
			value = "lucide:AppWindow"
		}
		in.Icon = &value
	}
	if in.TechnicalOwnerIDs != nil {
		owners, err := normalizeOwnerIDs(*in.TechnicalOwnerIDs)
		if err != nil {
			return Application{}, err
		}
		in.TechnicalOwnerIDs = &owners
	}
	if in.OperationsOwnerIDs != nil {
		owners, err := normalizeOwnerIDs(*in.OperationsOwnerIDs)
		if err != nil {
			return Application{}, err
		}
		in.OperationsOwnerIDs = &owners
	}
	if in.Instances != nil {
		current, err := s.store.Get(ctx, strings.TrimSpace(projectID), strings.TrimSpace(id))
		if err != nil {
			return Application{}, err
		}
		if current.RuntimeKind != RuntimeKubernetes && len(*in.Instances) == 0 {
			return Application{}, ErrInvalid
		}
		seen := make(map[string]struct{}, len(*in.Instances))
		for index := range *in.Instances {
			instance := &(*in.Instances)[index]
			instance.Name = strings.TrimSpace(instance.Name)
			instance.TargetResourceID = strings.TrimSpace(instance.TargetResourceID)
			instance.RuntimeKind = current.RuntimeKind
			if instance.Name == "" || instance.TargetResourceID == "" || !validSelector(current.RuntimeKind, instance.Selector) {
				return Application{}, ErrInvalid
			}
			if _, exists := seen[instance.Name]; exists {
				return Application{}, ErrInvalid
			}
			seen[instance.Name] = struct{}{}
			if err := s.validateBinding(ctx, current.RuntimeKind, instance.TargetResourceID, instance.Selector); err != nil {
				return Application{}, err
			}
		}
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
	app, err := s.store.Get(ctx, strings.TrimSpace(projectID), strings.TrimSpace(applicationID))
	if err != nil {
		return Instance{}, err
	}
	if app.RuntimeKind != in.RuntimeKind || !validSelector(in.RuntimeKind, in.Selector) {
		return Instance{}, ErrInvalid
	}
	if err := s.validateBinding(ctx, in.RuntimeKind, in.TargetResourceID, in.Selector); err != nil {
		return Instance{}, err
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
