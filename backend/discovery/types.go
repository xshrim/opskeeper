package discovery

import (
	"context"
	"time"

	"opskeeper/backend/organization"
	"opskeeper/backend/resource"
)

const (
	RunQueued    = "queued"
	RunRunning   = "running"
	RunSucceeded = "succeeded"
	RunFailed    = "failed"
	RunCancelled = "cancelled"

	ItemPending  = "pending"
	ItemImported = "imported"
	ItemIgnored  = "ignored"
	ItemMissing  = "missing"
)

type Run struct {
	ID                string     `json:"id"`
	ClusterResourceID string     `json:"cluster_resource_id"`
	Status            string     `json:"status"`
	ErrorMessage      string     `json:"error_message,omitempty"`
	StartedAt         *time.Time `json:"started_at,omitempty"`
	CompletedAt       *time.Time `json:"completed_at,omitempty"`
	ItemCount         int        `json:"item_count"`
	ImportedCount     int        `json:"imported_count"`
	CreatedBy         *string    `json:"created_by,omitempty"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
}

type Item struct {
	ID                 string            `json:"id"`
	RunID              string            `json:"run_id"`
	Kind               string            `json:"kind"`
	Namespace          string            `json:"namespace,omitempty"`
	Name               string            `json:"name"`
	ExternalUID        string            `json:"external_uid"`
	ResourceVersion    string            `json:"resource_version,omitempty"`
	Labels             map[string]string `json:"labels"`
	Payload            map[string]any    `json:"payload"`
	Status             string            `json:"status"`
	ImportedResourceID *string           `json:"imported_resource_id,omitempty"`
	ImportedProjectID  *string           `json:"imported_project_id,omitempty"`
	CreatedAt          time.Time         `json:"created_at"`
	UpdatedAt          time.Time         `json:"updated_at"`
}

type ScannedItem struct {
	Kind            string
	Namespace       string
	Name            string
	ExternalUID     string
	ResourceVersion string
	Labels          map[string]string
	Payload         map[string]any
}

type ImportInput struct {
	ItemIDs         []string                  `json:"item_ids"`
	ProjectMappings map[string]ProjectMapping `json:"project_mappings"`
}

type ProjectMapping struct {
	ProjectID string `json:"project_id,omitempty"`
	TeamID    string `json:"team_id,omitempty"`
	Name      string `json:"name,omitempty"`
	Code      string `json:"code,omitempty"`
	Ignore    bool   `json:"ignore,omitempty"`
}

type ImportResult struct {
	Run      Run    `json:"run"`
	Imported []Item `json:"imported"`
	Ignored  []Item `json:"ignored"`
}

type Store interface {
	CreateRun(context.Context, string, string) (Run, error)
	GetRun(context.Context, string) (Run, error)
	ListRuns(context.Context, string) ([]Run, error)
	SetRunning(context.Context, string) error
	ReplaceItems(context.Context, string, []ScannedItem) error
	CompleteRun(context.Context, string) error
	FailRun(context.Context, string, error) error
	ListItems(context.Context, string) ([]Item, error)
	MarkImported(context.Context, string, string, string) error
	MarkProjectMapped(context.Context, string, string, string) error
	MarkIgnored(context.Context, string) error
	MarkMissing(context.Context, string, []string) error
	ValidateImportTarget(context.Context, string, string) error
	ValidateProjectParent(context.Context, string, string) error
}

type ResourceReader interface {
	Get(context.Context, string) (resource.Resource, error)
}

type ResourceImporter interface {
	Import(context.Context, resource.ImportedInput) (resource.Resource, error)
}

type ProjectManager interface {
	GetTeam(context.Context, string) (organization.Team, error)
	GetProject(context.Context, string) (organization.Project, error)
	CreateProject(context.Context, organization.CreateProjectInput) (organization.Project, error)
	BindProjectSource(context.Context, string, organization.ProjectSourceInput) (organization.Project, error)
}

type CredentialReader interface {
	RevealLinked(context.Context, string) ([]byte, error)
}

type Scanner interface {
	Scan(context.Context, resource.Resource, string) ([]ScannedItem, error)
}
