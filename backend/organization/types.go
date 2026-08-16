package organization

import "time"

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
	ID         string            `json:"id"`
	PlatformID string            `json:"platform_id"`
	Scope      Scope             `json:"scope"`
	Name       string            `json:"name"`
	Code       string            `json:"code"`
	Icon       string            `json:"icon"`
	Labels     map[string]string `json:"labels"`
	Status     string            `json:"status"`
	CreatedAt  time.Time         `json:"created_at"`
	UpdatedAt  time.Time         `json:"updated_at"`
}

type Project struct {
	ID         string            `json:"id"`
	PlatformID string            `json:"platform_id"`
	TeamID     string            `json:"team_id"`
	Scope      Scope             `json:"scope"`
	Name       string            `json:"name"`
	Code       string            `json:"code"`
	Icon       string            `json:"icon"`
	Labels     map[string]string `json:"labels"`
	Source     string            `json:"source"`
	Status     string            `json:"status"`
	CreatedAt  time.Time         `json:"created_at"`
	UpdatedAt  time.Time         `json:"updated_at"`
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
	Name   string
	Code   string
	Icon   string
	Labels map[string]string
}

type UpdateTeamInput struct {
	Name   *string
	Icon   *string
	Labels *map[string]string
	Status *string
}

type CreateProjectInput struct {
	TeamID string
	Name   string
	Code   string
	Icon   string
	Labels map[string]string
	Source string
}

type UpdateProjectInput struct {
	Name   *string
	Icon   *string
	Labels *map[string]string
	Status *string
}
