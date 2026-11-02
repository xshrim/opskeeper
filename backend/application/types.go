package application

import "time"

type Application struct {
	ID        string `json:"id"`
	ProjectID string `json:"project_id"`
	// ProjectScopeID is used internally by authorization and diagnosis. It is
	// deliberately omitted from the public representation because applications
	// are always addressed through their project.
	ProjectScopeID     string            `json:"-"`
	Name               string            `json:"name"`
	Code               string            `json:"code"`
	Description        string            `json:"description"`
	Icon               string            `json:"icon"`
	RuntimeKind        string            `json:"runtime_kind"`
	Status             string            `json:"status"`
	ExternalUID        string            `json:"external_uid,omitempty"`
	Labels             map[string]string `json:"labels"`
	Critical           bool              `json:"critical"`
	TechnicalOwnerIDs  []string          `json:"technical_owner_ids"`
	OperationsOwnerIDs []string          `json:"operations_owner_ids"`
	Instances          []Instance        `json:"instances"`
	Dependencies       []Dependency      `json:"dependencies"`
	Relations          []Relation        `json:"relations"`
	CreatedAt          time.Time         `json:"created_at"`
	UpdatedAt          time.Time         `json:"updated_at"`
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

type Topology struct {
	Nodes []TopologyNode `json:"nodes"`
	Edges []TopologyEdge `json:"edges"`
}

type TopologyNode struct {
	ID           string         `json:"id"`
	Kind         string         `json:"kind"`
	Name         string         `json:"name"`
	ScopeID      string         `json:"scope_id,omitempty"`
	ProjectID    string         `json:"project_id,omitempty"`
	ProjectName  string         `json:"project_name,omitempty"`
	ResourceKind string         `json:"resource_kind,omitempty"`
	Metadata     map[string]any `json:"metadata,omitempty"`
}

type TopologyEdge struct {
	ID       string         `json:"id"`
	SourceID string         `json:"source_id"`
	TargetID string         `json:"target_id"`
	Relation string         `json:"relation"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

// DiagnosticContext is the immutable application metadata snapshot supplied
// to a diagnosis run. It contains configuration facts only; live state still
// comes from resource tools and observations.
type DiagnosticContext struct {
	Application  DiagnosticApplication            `json:"application"`
	Project      DiagnosticProject                `json:"project"`
	Resources    []DiagnosticResource             `json:"resources"`
	Dependencies []DiagnosticApplicationReference `json:"dependencies"`
	Dependents   []DiagnosticApplicationReference `json:"dependents"`
}

type DiagnosticApplication struct {
	ID          string            `json:"id"`
	ProjectID   string            `json:"project_id"`
	Name        string            `json:"name"`
	Code        string            `json:"code"`
	Description string            `json:"description,omitempty"`
	RuntimeKind string            `json:"runtime_kind"`
	Status      string            `json:"status"`
	Critical    bool              `json:"critical"`
	Labels      map[string]string `json:"labels,omitempty"`
}

type DiagnosticProject struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Code        string            `json:"code"`
	Description string            `json:"description,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
}

type DiagnosticResource struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Kind        string         `json:"kind"`
	Relation    string         `json:"relation"`
	RuntimeKind string         `json:"runtime_kind,omitempty"`
	Status      string         `json:"status"`
	Binding     map[string]any `json:"binding,omitempty"`
}

type DiagnosticApplicationReference struct {
	ID           string `json:"id"`
	ProjectID    string `json:"project_id"`
	ProjectName  string `json:"project_name"`
	ProjectCode  string `json:"project_code"`
	Name         string `json:"name"`
	Code         string `json:"code"`
	Description  string `json:"description,omitempty"`
	RuntimeKind  string `json:"runtime_kind"`
	Status       string `json:"status"`
	LinkStatus   string `json:"link_status"`
	RelationKind string `json:"relation_kind"`
}

type RelatedResource struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Kind string `json:"kind"`
	Role string `json:"role"`
}
type Alert struct {
	ID       string `json:"id"`
	Severity string `json:"severity"`
	Title    string `json:"title"`
	Status   string `json:"status"`
}

type CreateInput struct {
	ProjectID          string                  `json:"project_id,omitempty"`
	Name               string                  `json:"name"`
	Code               string                  `json:"code"`
	Description        string                  `json:"description"`
	Icon               string                  `json:"icon"`
	RuntimeKind        string                  `json:"runtime_kind"`
	ExternalUID        string                  `json:"external_uid,omitempty"`
	Labels             map[string]string       `json:"labels"`
	Critical           bool                    `json:"critical"`
	TechnicalOwnerIDs  []string                `json:"technical_owner_ids"`
	OperationsOwnerIDs []string                `json:"operations_owner_ids"`
	Instances          []CreateInstanceInput   `json:"instances"`
	Dependencies       []CreateDependencyInput `json:"dependencies,omitempty"`
	Relations          []CreateRelationInput   `json:"relations,omitempty"`
}
type UpdateInput struct {
	Name, Code, Description, Icon, Status *string
	RuntimeKind                           *string
	ExternalUID                           *string
	Labels                                *map[string]string
	Critical                              *bool
	TechnicalOwnerIDs                     *[]string
	OperationsOwnerIDs                    *[]string
	Instances                             *[]CreateInstanceInput
	Dependencies                          *[]CreateDependencyInput
	Relations                             *[]CreateRelationInput
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
	TargetResourceID string         `json:"target_resource_id"`
	DependencyKind   string         `json:"dependency_kind"`
	Binding          map[string]any `json:"binding"`
	Required         *bool          `json:"required"`
	Status           string         `json:"status"`
}

type Relation struct {
	ID                    string `json:"id"`
	ApplicationID         string `json:"application_id"`
	TargetApplicationID   string `json:"target_application_id"`
	TargetApplicationName string `json:"target_application_name,omitempty"`
	TargetApplicationCode string `json:"target_application_code,omitempty"`
	RelationKind          string `json:"relation_kind"`
	Status                string `json:"status"`
}

type CreateRelationInput struct {
	TargetApplicationID string `json:"target_application_id"`
	RelationKind        string `json:"relation_kind"`
	Status              string `json:"status"`
}
