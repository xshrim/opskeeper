package resource

import "time"

const (
	StatusActive   = "active"
	StatusDisabled = "disabled"
	StatusUnknown  = "unknown"
)

type Resource struct {
	ID               string            `json:"id"`
	ScopeID          string            `json:"scope_id"`
	Kind             string            `json:"kind"`
	SchemaVersion    int               `json:"schema_version"`
	Name             string            `json:"name"`
	ExternalUID      string            `json:"external_uid,omitempty"`
	SourceResourceID string            `json:"source_resource_id,omitempty"`
	Labels           map[string]string `json:"labels"`
	Config           map[string]any    `json:"config"`
	Status           string            `json:"status"`
	CredentialID     *string           `json:"credential_id,omitempty"`
	CreatedAt        time.Time         `json:"created_at"`
	UpdatedAt        time.Time         `json:"updated_at"`
}

type Schema struct {
	ID        string         `json:"id"`
	Kind      string         `json:"kind"`
	Version   int            `json:"version"`
	Schema    map[string]any `json:"schema"`
	Status    string         `json:"status"`
	CreatedAt time.Time      `json:"created_at"`
}

type Relation struct {
	ID               string         `json:"id"`
	SourceResourceID string         `json:"source_resource_id"`
	TargetResourceID string         `json:"target_resource_id"`
	RelationType     string         `json:"relation_type"`
	Attributes       map[string]any `json:"attributes"`
	DiscoverySource  string         `json:"discovery_source"`
	Confidence       float64        `json:"confidence"`
	Confirmed        bool           `json:"confirmed"`
	CreatedBy        *string        `json:"created_by,omitempty"`
	CreatedAt        time.Time      `json:"created_at"`
}

type TopologyNode struct {
	Resource Resource `json:"resource"`
	Depth    int      `json:"depth"`
}

type Page[T any] struct {
	Items    []T   `json:"items"`
	Page     int   `json:"page"`
	PageSize int   `json:"page_size"`
	Total    int64 `json:"total"`
}

type Pagination struct {
	Page     int
	PageSize int
}

func (p Pagination) Offset() int { return (p.Page - 1) * p.PageSize }

type CreateInput struct {
	ScopeID          string
	Kind             string
	SchemaVersion    int
	Name             string
	ExternalUID      string
	SourceResourceID string
	Labels           map[string]string
	Config           map[string]any
	Status           string
	CredentialID     *string
}

type UpdateInput struct {
	ScopeID          *string
	Name             *string
	ExternalUID      *string
	SourceResourceID *string
	Labels           *map[string]string
	Config           *map[string]any
	Status           *string
	CredentialID     **string
}

type CreateRelationInput struct {
	SourceResourceID string
	TargetResourceID string
	RelationType     string
	Attributes       map[string]any
	DiscoverySource  string
	Confidence       float64
	Confirmed        bool
}

type Default struct {
	ScopeID    string    `json:"scope_id"`
	DefaultKey string    `json:"default_key"`
	ResourceID string    `json:"resource_id"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}
