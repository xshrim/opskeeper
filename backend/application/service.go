package application

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"opskeeper/backend/authorization"
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
	CreateMany(context.Context, []CreateInput) ([]Application, error)
	Get(context.Context, string, string) (Application, error)
	List(context.Context, string) ([]Application, error)
	Update(context.Context, string, string, UpdateInput) (Application, error)
	Delete(context.Context, string, string) error
	CreateInstance(context.Context, string, string, CreateInstanceInput) (Instance, error)
	DeleteInstance(context.Context, string, string, string) error
	ContextResourceIDs(context.Context, string, string) ([]string, error)
	DiagnosticContext(context.Context, string, string) (DiagnosticContext, error)
	Workspace(context.Context, string) (Workspace, error)
	Topology(context.Context, string) (Topology, error)
	SetTopologyPosition(context.Context, string, string, TopologyPositionInput) error
	CreateTopologyAnnotation(context.Context, string, TopologyAnnotationInput) (TopologyNode, error)
	AddTopologyResource(context.Context, string, TopologyResourceInput) error
	RemoveTopologyResource(context.Context, string, string) error
	DeleteTopologyAnnotation(context.Context, string, string) error
	CreateTopologyEdge(context.Context, string, TopologyEdgeInput) (TopologyEdge, error)
	DeleteTopologyEdge(context.Context, string, string) error
}

type BindingValidator interface {
	ValidateApplicationBinding(context.Context, string, string, map[string]any) error
}

type Service struct {
	store            Store
	bindingValidator BindingValidator
}

type diagnosticCandidateStore interface {
	DiagnosticCandidates(context.Context, string) ([]Application, error)
}

type diagnosticScopeQueryStore interface {
	DiagnosticScopeTeams(context.Context, string) ([]DiagnosticScopeTeam, error)
	DiagnosticScopeProjects(context.Context, string, string) ([]DiagnosticScopeProject, error)
	DiagnosticScopeApplications(context.Context, string, string, string, int, int) (DiagnosticScopeApplicationPage, error)
	DiagnosticApplicationContextByLocator(context.Context, string, string, string) (DiagnosticContext, error)
}

func NewService(store Store) *Service { return &Service{store: store} }
func (s *Service) SetBindingValidator(validator BindingValidator) {
	s.bindingValidator = validator
}
func validateCreate(in CreateInput) error {
	if strings.TrimSpace(in.Name) == "" || !codePattern.MatchString(strings.TrimSpace(in.Code)) {
		return ErrInvalid
	}
	if !validRuntimeKind(in.RuntimeKind) {
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
	required := map[string]string{RuntimeHost: "process_command_line", RuntimeDocker: "container_name", RuntimeKubernetes: "workload_name"}
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

func validInstanceStatus(status string) bool {
	return status == "" || status == "active" || status == "disabled" || status == "unknown"
}

func validDependencyKind(kind string) bool {
	switch kind {
	case "messaging", "cache", "database", "registry", "configuration", "repository", "storage", "observability", "other":
		return true
	default:
		return false
	}
}

func validRelationKind(kind string) bool {
	return kind == "depends_on"
}

func (s *Service) Create(ctx context.Context, in CreateInput) (Application, error) {
	applications, err := s.CreateMany(ctx, []CreateInput{in})
	if err != nil {
		return Application{}, err
	}
	return applications[0], nil
}

func (s *Service) CreateMany(ctx context.Context, inputs []CreateInput) ([]Application, error) {
	if len(inputs) == 0 {
		return nil, ErrInvalid
	}
	projectID := strings.TrimSpace(inputs[0].ProjectID)
	if projectID == "" {
		return nil, ErrInvalid
	}
	prepared := make([]CreateInput, len(inputs))
	codes := make(map[string]struct{}, len(inputs))
	for index, input := range inputs {
		if projectID != strings.TrimSpace(input.ProjectID) {
			return nil, ErrInvalid
		}
		input.ProjectID = projectID
		var err error
		prepared[index], err = PrepareCreateInput(input)
		if err != nil || prepared[index].ProjectID != projectID {
			return nil, ErrInvalid
		}
		if _, exists := codes[prepared[index].Code]; exists {
			return nil, ErrInvalid
		}
		codes[prepared[index].Code] = struct{}{}
	}
	for _, input := range prepared {
		if err := s.validateBindings(ctx, input.Instances); err != nil {
			return nil, err
		}
	}
	return s.store.CreateMany(ctx, prepared)
}

func (s *Service) validateBinding(ctx context.Context, runtimeKind, resourceID string, selector map[string]any) error {
	if s.bindingValidator == nil {
		return fmt.Errorf("%w: application binding validation is unavailable", ErrInvalid)
	}
	if err := s.bindingValidator.ValidateApplicationBinding(ctx, runtimeKind, resourceID, selector); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalid, err)
	}
	return nil
}

func (s *Service) validateBindings(ctx context.Context, instances []CreateInstanceInput) error {
	seen := make(map[string]struct{}, len(instances))
	for _, instance := range instances {
		key := instance.RuntimeKind + "\x00" + instance.TargetResourceID
		if instance.RuntimeKind == RuntimeKubernetes {
			key += "\x00" + selectorString(instance.Selector, "namespace") + "/" + selectorString(instance.Selector, "workload_kind") + "/" + selectorString(instance.Selector, "workload_name")
		} else {
			selector, err := json.Marshal(instance.Selector)
			if err != nil {
				return fmt.Errorf("encode application binding selector: %w", err)
			}
			key += "\x00" + string(selector)
		}
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		if err := s.validateBinding(ctx, instance.RuntimeKind, instance.TargetResourceID, instance.Selector); err != nil {
			return err
		}
	}
	return nil
}

func selectorString(selector map[string]any, key string) string {
	value, _ := selector[key].(string)
	return strings.TrimSpace(value)
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
	if in.Dependencies, err = normalizeDependencies(in.Dependencies); err != nil {
		return CreateInput{}, err
	}
	if in.Relations, err = normalizeRelations(in.Relations); err != nil {
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
		if !validInstanceStatus(in.Instances[index].Status) {
			return CreateInput{}, ErrInvalid
		}
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

// DiagnosticCandidates lists applications under the currently selected
// diagnosis scope. Authorization comes from the diagnosis:start filter in the
// request context, rather than from organization:read.
func (s *Service) DiagnosticCandidates(ctx context.Context, scopeID string) ([]Application, error) {
	scopeID = strings.TrimSpace(scopeID)
	if scopeID == "" {
		return nil, ErrInvalid
	}
	if filter, restricted := authorization.ScopeFilterFromContext(ctx); restricted && !filter.Allows(scopeID) {
		return nil, ErrNotFound
	}
	store, ok := s.store.(diagnosticCandidateStore)
	if !ok {
		return nil, ErrNotFound
	}
	return store.DiagnosticCandidates(ctx, scopeID)
}
func (s *Service) UpdateInProject(ctx context.Context, projectID, id string, in UpdateInput) (Application, error) {
	if strings.TrimSpace(projectID) == "" || strings.TrimSpace(id) == "" {
		return Application{}, ErrInvalid
	}
	if in.Name == nil && in.Code == nil && in.Description == nil && in.Icon == nil && in.Status == nil && in.RuntimeKind == nil && in.ExternalUID == nil && in.Labels == nil && in.Critical == nil && in.TechnicalOwnerIDs == nil && in.OperationsOwnerIDs == nil && in.Instances == nil && in.Dependencies == nil && in.Relations == nil {
		return Application{}, ErrInvalid
	}
	if in.Name != nil {
		value := strings.TrimSpace(*in.Name)
		if value == "" {
			return Application{}, ErrInvalid
		}
		in.Name = &value
	}
	if in.Code != nil {
		value := strings.TrimSpace(*in.Code)
		if !codePattern.MatchString(value) {
			return Application{}, ErrInvalid
		}
		in.Code = &value
	}
	if in.Icon != nil {
		value := strings.TrimSpace(*in.Icon)
		if value == "" || len([]rune(value)) > maxIconLength {
			value = "lucide:AppWindow"
		}
		in.Icon = &value
	}
	if in.ExternalUID != nil {
		value := strings.TrimSpace(*in.ExternalUID)
		if len(value) > 512 {
			return Application{}, ErrInvalid
		}
		in.ExternalUID = &value
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
	if in.Dependencies != nil {
		dependencies, err := normalizeDependencies(*in.Dependencies)
		if err != nil {
			return Application{}, err
		}
		in.Dependencies = &dependencies
	}
	if in.Relations != nil {
		relations, err := normalizeRelations(*in.Relations)
		if err != nil {
			return Application{}, err
		}
		in.Relations = &relations
	}
	if in.RuntimeKind != nil || in.Instances != nil {
		current, err := s.store.Get(ctx, strings.TrimSpace(projectID), strings.TrimSpace(id))
		if err != nil {
			return Application{}, err
		}
		runtimeKind := current.RuntimeKind
		if in.RuntimeKind != nil {
			runtimeKind = strings.TrimSpace(*in.RuntimeKind)
			if !validRuntimeKind(runtimeKind) || (runtimeKind != current.RuntimeKind && in.Instances == nil) {
				return Application{}, ErrInvalid
			}
			in.RuntimeKind = &runtimeKind
		}
		if in.Instances == nil {
			return s.store.Update(ctx, strings.TrimSpace(projectID), strings.TrimSpace(id), in)
		}
		if runtimeKind == "" {
			return Application{}, ErrInvalid
		}
		seen := make(map[string]struct{}, len(*in.Instances))
		for index := range *in.Instances {
			instance := &(*in.Instances)[index]
			instance.Name = strings.TrimSpace(instance.Name)
			instance.TargetResourceID = strings.TrimSpace(instance.TargetResourceID)
			instance.RuntimeKind = runtimeKind
			if instance.Name == "" || instance.TargetResourceID == "" || !validSelector(runtimeKind, instance.Selector) || !validInstanceStatus(instance.Status) {
				return Application{}, ErrInvalid
			}
			if _, exists := seen[instance.Name]; exists {
				return Application{}, ErrInvalid
			}
			seen[instance.Name] = struct{}{}
		}
		if err := s.validateBindings(ctx, *in.Instances); err != nil {
			return Application{}, err
		}
	}
	return s.store.Update(ctx, strings.TrimSpace(projectID), strings.TrimSpace(id), in)
}

func normalizeDependencies(items []CreateDependencyInput) ([]CreateDependencyInput, error) {
	result := make([]CreateDependencyInput, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		item.TargetResourceID = strings.TrimSpace(item.TargetResourceID)
		item.DependencyKind = strings.TrimSpace(item.DependencyKind)
		if item.TargetResourceID == "" || !validDependencyKind(item.DependencyKind) {
			return nil, ErrInvalid
		}
		binding, err := normalizeDependencyBinding(item.DependencyKind, item.Binding)
		if err != nil {
			return nil, ErrInvalid
		}
		item.Binding = binding
		key := item.DependencyKind + "\x00" + item.TargetResourceID
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		if item.Status == "" {
			item.Status = "unknown"
		}
		if !validInstanceStatus(item.Status) {
			return nil, ErrInvalid
		}
		result = append(result, item)
	}
	return result, nil
}

func validDependencyBinding(kind string, binding map[string]any) bool {
	_, err := normalizeDependencyBinding(kind, binding)
	return err == nil
}

func normalizeDependencyBinding(kind string, binding map[string]any) (map[string]any, error) {
	if binding == nil {
		return nil, nil
	}
	result := make(map[string]any, len(binding))
	for key, value := range binding {
		key = strings.TrimSpace(key)
		if !allowedDependencyBindingField(kind, key) {
			return nil, ErrInvalid
		}
		if kind == "repository" && (key == "directory" || key == "path") {
			text, ok := value.(string)
			if !ok {
				return nil, ErrInvalid
			}
			text, err := normalizeRepositoryDirectory(text)
			if err != nil {
				return nil, err
			}
			if key == "path" {
				if _, exists := result["directory"]; exists {
					continue
				}
				key = "directory"
			}
			result[key] = text
			continue
		}
		switch value.(type) {
		case string, bool, float64, int, int64:
		default:
			return nil, ErrInvalid
		}
		result[key] = value
	}
	return result, nil
}

func normalizeRepositoryDirectory(value string) (string, error) {
	value = strings.TrimSpace(strings.ReplaceAll(value, "\\", "/"))
	if value == "" {
		return "", nil
	}
	if strings.ContainsAny(value, "\x00\r\n") || filepath.IsAbs(value) || strings.HasPrefix(value, "/") {
		return "", ErrInvalid
	}
	clean := filepath.ToSlash(filepath.Clean(value))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") {
		return "", ErrInvalid
	}
	return strings.Trim(clean, "/"), nil
}

func allowedDependencyBindingField(kind, key string) bool {
	switch kind {
	case "database":
		return oneOf(key, "database", "schema", "instance", "service", "sid")
	case "messaging":
		return oneOf(key, "topic", "queue", "exchange", "group", "vhost")
	case "configuration":
		return oneOf(key, "namespace", "group", "data_id", "file")
	case "registry":
		return oneOf(key, "namespace", "group")
	case "cache":
		return oneOf(key, "database", "key_prefix")
	case "storage":
		return oneOf(key, "bucket", "prefix")
	case "repository":
		return oneOf(key, "directory", "path", "revision", "branch")
	case "observability":
		return oneOf(key, "target", "metric", "namespace")
	case "other":
		return oneOf(key, "object", "identifier")
	default:
		return false
	}
}

func oneOf(value string, options ...string) bool {
	for _, option := range options {
		if value == option {
			return true
		}
	}
	return false
}

func normalizeRelations(items []CreateRelationInput) ([]CreateRelationInput, error) {
	result := make([]CreateRelationInput, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		item.TargetApplicationID = strings.TrimSpace(item.TargetApplicationID)
		item.RelationKind = strings.TrimSpace(item.RelationKind)
		if item.TargetApplicationID == "" || !uuidPattern.MatchString(item.TargetApplicationID) || !validRelationKind(item.RelationKind) {
			return nil, ErrInvalid
		}
		key := item.RelationKind + "\x00" + item.TargetApplicationID
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		if item.Status == "" {
			item.Status = "active"
		}
		if !validInstanceStatus(item.Status) {
			return nil, ErrInvalid
		}
		result = append(result, item)
	}
	return result, nil
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
func (s *Service) DiagnosticContext(ctx context.Context, applicationID, scopeID string) (DiagnosticContext, error) {
	if strings.TrimSpace(applicationID) == "" || strings.TrimSpace(scopeID) == "" {
		return DiagnosticContext{}, ErrInvalid
	}
	return s.store.DiagnosticContext(ctx, strings.TrimSpace(scopeID), strings.TrimSpace(applicationID))
}

// DiagnosticScopeTeams returns the visible teams for a diagnosis scope.
func (s *Service) DiagnosticScopeTeams(ctx context.Context, scopeID string) ([]DiagnosticScopeTeam, error) {
	reader, ok := s.store.(diagnosticScopeQueryStore)
	if !ok {
		return nil, ErrInvalid
	}
	return reader.DiagnosticScopeTeams(ctx, strings.TrimSpace(scopeID))
}

// DiagnosticScopeProjects returns visible projects, optionally narrowed by
// the human-facing team name supplied by the model or UI.
func (s *Service) DiagnosticScopeProjects(ctx context.Context, scopeID, teamName string) ([]DiagnosticScopeProject, error) {
	reader, ok := s.store.(diagnosticScopeQueryStore)
	if !ok {
		return nil, ErrInvalid
	}
	return reader.DiagnosticScopeProjects(ctx, strings.TrimSpace(scopeID), strings.TrimSpace(teamName))
}

func (s *Service) DiagnosticScopeApplications(ctx context.Context, scopeID, teamName, projectCode string, offset, limit int) (DiagnosticScopeApplicationPage, error) {
	reader, ok := s.store.(diagnosticScopeQueryStore)
	if !ok {
		return DiagnosticScopeApplicationPage{}, ErrInvalid
	}
	return reader.DiagnosticScopeApplications(ctx, strings.TrimSpace(scopeID), strings.TrimSpace(teamName), strings.TrimSpace(projectCode), offset, limit)
}

// DiagnosticApplicationContextByLocator resolves an application by its
// human-facing name or code inside the authorized scope and returns its full
// server-side diagnostic snapshot.
func (s *Service) DiagnosticApplicationContextByLocator(ctx context.Context, scopeID, name, code string) (DiagnosticContext, error) {
	reader, ok := s.store.(diagnosticScopeQueryStore)
	if !ok {
		return DiagnosticContext{}, ErrInvalid
	}
	return reader.DiagnosticApplicationContextByLocator(ctx, strings.TrimSpace(scopeID), strings.TrimSpace(name), strings.TrimSpace(code))
}
func (s *Service) Workspace(ctx context.Context, pid string) (Workspace, error) {
	if pid == "" {
		return Workspace{}, ErrInvalid
	}
	return s.store.Workspace(ctx, pid)
}
func (s *Service) Topology(ctx context.Context, pid string) (Topology, error) {
	if strings.TrimSpace(pid) == "" {
		return Topology{}, ErrInvalid
	}
	return s.store.Topology(ctx, strings.TrimSpace(pid))
}
func (s *Service) SetTopologyPosition(ctx context.Context, pid, nodeID string, input TopologyPositionInput) error {
	if strings.TrimSpace(pid) == "" || !uuidPattern.MatchString(strings.TrimSpace(nodeID)) || input.X < 0 || input.Y < 0 || input.X > 10000 || input.Y > 10000 {
		return ErrInvalid
	}
	return s.store.SetTopologyPosition(ctx, strings.TrimSpace(pid), strings.TrimSpace(nodeID), input)
}
func (s *Service) CreateTopologyAnnotation(ctx context.Context, pid string, input TopologyAnnotationInput) (TopologyNode, error) {
	if strings.TrimSpace(pid) == "" || strings.TrimSpace(input.Name) == "" {
		return TopologyNode{}, ErrInvalid
	}
	return s.store.CreateTopologyAnnotation(ctx, strings.TrimSpace(pid), input)
}
func (s *Service) AddTopologyResource(ctx context.Context, pid string, input TopologyResourceInput) error {
	if strings.TrimSpace(pid) == "" || strings.TrimSpace(input.ResourceID) == "" {
		return ErrInvalid
	}
	return s.store.AddTopologyResource(ctx, strings.TrimSpace(pid), TopologyResourceInput{ResourceID: strings.TrimSpace(input.ResourceID)})
}
func (s *Service) RemoveTopologyResource(ctx context.Context, pid, resourceID string) error {
	if strings.TrimSpace(pid) == "" || strings.TrimSpace(resourceID) == "" {
		return ErrInvalid
	}
	return s.store.RemoveTopologyResource(ctx, strings.TrimSpace(pid), strings.TrimSpace(resourceID))
}
func (s *Service) DeleteTopologyAnnotation(ctx context.Context, pid, annotationID string) error {
	if strings.TrimSpace(pid) == "" || strings.TrimSpace(annotationID) == "" {
		return ErrInvalid
	}
	return s.store.DeleteTopologyAnnotation(ctx, strings.TrimSpace(pid), strings.TrimSpace(annotationID))
}
func (s *Service) CreateTopologyEdge(ctx context.Context, pid string, input TopologyEdgeInput) (TopologyEdge, error) {
	if strings.TrimSpace(pid) == "" || !uuidPattern.MatchString(strings.TrimSpace(input.SourceID)) || !uuidPattern.MatchString(strings.TrimSpace(input.TargetID)) || strings.TrimSpace(input.SourceID) == strings.TrimSpace(input.TargetID) || strings.TrimSpace(input.Relation) == "" {
		return TopologyEdge{}, ErrInvalid
	}
	return s.store.CreateTopologyEdge(ctx, strings.TrimSpace(pid), input)
}
func (s *Service) DeleteTopologyEdge(ctx context.Context, pid, edgeID string) error {
	if strings.TrimSpace(pid) == "" || !uuidPattern.MatchString(strings.TrimSpace(edgeID)) {
		return ErrInvalid
	}
	return s.store.DeleteTopologyEdge(ctx, strings.TrimSpace(pid), strings.TrimSpace(edgeID))
}
