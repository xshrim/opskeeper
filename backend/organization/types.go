package organization

import (
	"opskeeper/backend/application"
	"time"
)

const (
	StatusActive   = "active"
	StatusDisabled = "disabled"
)

type Scope struct {
	ID       string  `json:"id"`
	Type     string  `json:"type"`
	ParentID *string `json:"parent_id,omitempty"`
	Status   string  `json:"status"`
}

type Platform struct {
	ID        string    `json:"id"`
	Scope     Scope     `json:"scope"`
	Name      string    `json:"name"`
	Code      string    `json:"code"`
	Icon      string    `json:"icon"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Team struct {
	ID          string            `json:"id"`
	PlatformID  string            `json:"platform_id"`
	Scope       Scope             `json:"scope"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Icon        string            `json:"icon"`
	Labels      map[string]string `json:"labels"`
	OwnerIDs    []string          `json:"owner_ids"`
	Status      string            `json:"status"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

type Project struct {
	ID                 string            `json:"id"`
	PlatformID         string            `json:"platform_id"`
	TeamID             string            `json:"team_id"`
	Scope              Scope             `json:"scope"`
	Name               string            `json:"name"`
	Code               string            `json:"code"`
	Description        string            `json:"description"`
	Icon               string            `json:"icon"`
	Labels             map[string]string `json:"labels"`
	ImportanceLevel    string            `json:"importance_level"`
	TechnicalOwnerIDs  []string          `json:"technical_owner_ids"`
	OperationsOwnerIDs []string          `json:"operations_owner_ids"`
	Summary            ProjectSummary    `json:"summary"`
	Status             string            `json:"status"`
	CreatedAt          time.Time         `json:"created_at"`
	UpdatedAt          time.Time         `json:"updated_at"`
}

type ProjectSummary struct {
	Applications        int `json:"applications"`
	ApplicationsHealthy int `json:"applications_healthy"`
	Resources           int `json:"resources"`
	Alerts              int `json:"alerts"`
	Dependencies        int `json:"dependencies"`
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

func (p Pagination) Offset() int {
	return (p.Page - 1) * p.PageSize
}

type CreateTeamInput struct {
	Name        string
	Description string
	Icon        string
	Labels      map[string]string
	OwnerIDs    []string
}

type UpdateTeamInput struct {
	Name        *string
	Description *string
	Icon        *string
	Labels      *map[string]string
	OwnerIDs    *[]string
	Status      *string
}

type CreateProjectInput struct {
	TeamID             string
	Name               string
	Code               string
	Description        string
	Icon               string
	Labels             map[string]string
	ImportanceLevel    string
	TechnicalOwnerIDs  []string
	OperationsOwnerIDs []string
	Applications       []application.CreateInput
}

type UpdateProjectInput struct {
	Name               *string
	Description        *string
	Icon               *string
	Labels             *map[string]string
	ImportanceLevel    *string
	TechnicalOwnerIDs  *[]string
	OperationsOwnerIDs *[]string
	Status             *string
}

// DeleteInput keeps destructive operations explicit. Resources are deliberately
// not part of this operation: applications only reference resources and a
// project or team removal must never remove infrastructure implicitly.
type DeleteInput struct {
	ConfirmationName string
	Cascade          bool
}

type DeleteSummary struct {
	Teams        int `json:"teams"`
	Projects     int `json:"projects"`
	Applications int `json:"applications"`
	Instances    int `json:"instances"`
	RoleBindings int `json:"role_bindings"`
	Resources    int `json:"resources"`
}
