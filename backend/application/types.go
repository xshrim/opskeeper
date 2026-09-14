package application

import "time"

type Application struct {
	ID        string `json:"id"`
	ProjectID string `json:"project_id"`
	// ProjectScopeID is used internally by authorization and diagnosis. It is
	// deliberately omitted from the public representation because applications
	// are always addressed through their project.
	ProjectScopeID string            `json:"-"`
	Name           string            `json:"name"`
	Code           string            `json:"code"`
	Description    string            `json:"description"`
	Icon           string            `json:"icon"`
	Status         string            `json:"status"`
	Source         string            `json:"source"`
	ExternalUID    string            `json:"external_uid,omitempty"`
	Labels         map[string]string `json:"labels"`
	Instances      []Instance        `json:"instances"`
	Dependencies   []Dependency      `json:"dependencies"`
	CreatedAt      time.Time         `json:"created_at"`
	UpdatedAt      time.Time         `json:"updated_at"`
}

type Instance struct {
	ID                 string         `json:"id"`
	ApplicationID      string         `json:"application_id"`
	Name               string         `json:"name"`
	RuntimeKind        string         `json:"runtime_kind"`
	TargetResourceID   string         `json:"target_resource_id"`
	TargetResourceName string         `json:"target_resource_name,omitempty"`
	TargetResourceKind string         `json:"target_resource_kind,omitempty"`
	Selector           map[string]any `json:"selector"`
	LogBinding         map[string]any `json:"log_binding"`
	Status             string         `json:"status"`
}

type Dependency struct {
	ID                 string         `json:"id"`
	ApplicationID      string         `json:"application_id"`
	TargetResourceID   string         `json:"target_resource_id"`
	TargetResourceName string         `json:"target_resource_name,omitempty"`
	TargetResourceKind string         `json:"target_resource_kind,omitempty"`
	DependencyKind     string         `json:"dependency_kind"`
	Binding            map[string]any `json:"binding"`
	Required           bool           `json:"required"`
	Status             string         `json:"status"`
}

type ProjectSummary struct {
	ProjectID    string `json:"project_id"`
	Applications int    `json:"applications"`
	Instances    int    `json:"instances"`
	Resources    int    `json:"resources"`
	Dependencies int    `json:"dependencies"`
	Alerts       int    `json:"alerts"`
}

type Workspace struct {
	Summary      ProjectSummary    `json:"summary"`
	Resources    []RelatedResource `json:"resources"`
	Alerts       []Alert           `json:"alerts"`
	Applications []Application     `json:"applications"`
}

type RelatedResource struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Kind   string `json:"kind"`
	Status string `json:"status"`
	Role   string `json:"role"`
}
type Alert struct {
	ID       string `json:"id"`
	Severity string `json:"severity"`
	Title    string `json:"title"`
	Status   string `json:"status"`
}

type CreateInput struct {
	ProjectID, Name, Code, Description, Icon, Source, ExternalUID string
	Labels                                                        map[string]string
	Instances                                                     []CreateInstanceInput
	Dependencies                                                  []CreateDependencyInput
}
type UpdateInput struct {
	Name, Description, Icon, Status *string
	Labels                          *map[string]string
}
type CreateInstanceInput struct {
	ApplicationID    string         `json:"application_id,omitempty"`
	Name             string         `json:"name"`
	RuntimeKind      string         `json:"runtime_kind"`
	TargetResourceID string         `json:"target_resource_id"`
	Selector         map[string]any `json:"selector"`
	LogBinding       map[string]any `json:"log_binding"`
	Status           string         `json:"status"`
}
type CreateDependencyInput struct {
	ApplicationID    string         `json:"application_id,omitempty"`
	TargetResourceID string         `json:"target_resource_id"`
	DependencyKind   string         `json:"dependency_kind"`
	Binding          map[string]any `json:"binding"`
	Required         bool           `json:"required"`
	Status           string         `json:"status"`
}
type ImportInput struct {
	ProjectID    string                  `json:"project_id"`
	Name         string                  `json:"name"`
	Code         string                  `json:"code"`
	Description  string                  `json:"description"`
	Icon         string                  `json:"icon"`
	Source       string                  `json:"source"`
	ExternalUID  string                  `json:"external_uid"`
	Labels       map[string]string       `json:"labels"`
	Instances    []CreateInstanceInput   `json:"instances"`
	Dependencies []CreateDependencyInput `json:"dependencies"`
}
