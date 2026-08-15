package organization

import (
	"context"
	"regexp"
	"strings"
)

const (
	defaultPageSize = 20
	maxPageSize     = 100
	maxLabels       = 50
)

var (
	codePattern = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,62}[a-z0-9])?$`)
	uuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[1-5][0-9a-fA-F]{3}-[89abAB][0-9a-fA-F]{3}-[0-9a-fA-F]{12}$`)
)

type Repository interface {
	GetPlatform(context.Context) (Platform, error)
	CreateTeam(context.Context, CreateTeamInput) (Team, error)
	ListTeams(context.Context, Pagination) (Page[Team], error)
	GetTeam(context.Context, string) (Team, error)
	UpdateTeam(context.Context, string, UpdateTeamInput) (Team, error)
	CreateProject(context.Context, CreateProjectInput) (Project, error)
	ListProjects(context.Context, string, Pagination) (Page[Project], error)
	GetProject(context.Context, string) (Project, error)
	UpdateProject(context.Context, string, UpdateProjectInput) (Project, error)
}

type Service struct {
	repository Repository
}

func NewService(repository Repository) *Service {
	return &Service{repository: repository}
}

func (s *Service) GetPlatform(ctx context.Context) (Platform, error) {
	return s.repository.GetPlatform(ctx)
}

func (s *Service) CreateTeam(ctx context.Context, input CreateTeamInput) (Team, error) {
	input.Name = strings.TrimSpace(input.Name)
	input.Code = strings.TrimSpace(input.Code)
	if err := validateName(input.Name); err != nil {
		return Team{}, err
	}
	if err := validateCode(input.Code); err != nil {
		return Team{}, err
	}
	if err := validateLabels(input.Labels); err != nil {
		return Team{}, err
	}
	input.Labels = cloneLabels(input.Labels)
	return s.repository.CreateTeam(ctx, input)
}

func (s *Service) ListTeams(ctx context.Context, pagination Pagination) (Page[Team], error) {
	pagination, err := normalizePagination(pagination)
	if err != nil {
		return Page[Team]{}, err
	}
	return s.repository.ListTeams(ctx, pagination)
}

func (s *Service) GetTeam(ctx context.Context, teamID string) (Team, error) {
	if err := validateID(teamID, "team_id"); err != nil {
		return Team{}, err
	}
	return s.repository.GetTeam(ctx, teamID)
}

func (s *Service) UpdateTeam(ctx context.Context, teamID string, input UpdateTeamInput) (Team, error) {
	if err := validateID(teamID, "team_id"); err != nil {
		return Team{}, err
	}
	if input.Name == nil && input.Labels == nil && input.Status == nil {
		return Team{}, invalid("at least one field must be provided")
	}
	if input.Name != nil {
		trimmed := strings.TrimSpace(*input.Name)
		if err := validateName(trimmed); err != nil {
			return Team{}, err
		}
		input.Name = &trimmed
	}
	if input.Labels != nil {
		if err := validateLabels(*input.Labels); err != nil {
			return Team{}, err
		}
		labels := cloneLabels(*input.Labels)
		input.Labels = &labels
	}
	if input.Status != nil {
		status := strings.TrimSpace(*input.Status)
		if err := validateStatus(status); err != nil {
			return Team{}, err
		}
		input.Status = &status
	}
	return s.repository.UpdateTeam(ctx, teamID, input)
}

func (s *Service) CreateProject(ctx context.Context, input CreateProjectInput) (Project, error) {
	if err := validateID(input.TeamID, "team_id"); err != nil {
		return Project{}, err
	}
	input.Name = strings.TrimSpace(input.Name)
	input.Code = strings.TrimSpace(input.Code)
	if err := validateName(input.Name); err != nil {
		return Project{}, err
	}
	if err := validateCode(input.Code); err != nil {
		return Project{}, err
	}
	if err := validateLabels(input.Labels); err != nil {
		return Project{}, err
	}
	if input.Source == "" {
		input.Source = "manual"
	}
	if input.Source != "manual" && input.Source != "kubernetes" {
		return Project{}, invalid("source must be manual or kubernetes")
	}
	input.Labels = cloneLabels(input.Labels)
	return s.repository.CreateProject(ctx, input)
}

func (s *Service) ListProjects(ctx context.Context, teamID string, pagination Pagination) (Page[Project], error) {
	if err := validateID(teamID, "team_id"); err != nil {
		return Page[Project]{}, err
	}
	pagination, err := normalizePagination(pagination)
	if err != nil {
		return Page[Project]{}, err
	}
	return s.repository.ListProjects(ctx, teamID, pagination)
}

func (s *Service) GetProject(ctx context.Context, projectID string) (Project, error) {
	if err := validateID(projectID, "project_id"); err != nil {
		return Project{}, err
	}
	return s.repository.GetProject(ctx, projectID)
}

func (s *Service) UpdateProject(ctx context.Context, projectID string, input UpdateProjectInput) (Project, error) {
	if err := validateID(projectID, "project_id"); err != nil {
		return Project{}, err
	}
	if input.Name == nil && input.Labels == nil && input.Status == nil {
		return Project{}, invalid("at least one field must be provided")
	}
	if input.Name != nil {
		trimmed := strings.TrimSpace(*input.Name)
		if err := validateName(trimmed); err != nil {
			return Project{}, err
		}
		input.Name = &trimmed
	}
	if input.Labels != nil {
		if err := validateLabels(*input.Labels); err != nil {
			return Project{}, err
		}
		labels := cloneLabels(*input.Labels)
		input.Labels = &labels
	}
	if input.Status != nil {
		status := strings.TrimSpace(*input.Status)
		if err := validateStatus(status); err != nil {
			return Project{}, err
		}
		input.Status = &status
	}
	return s.repository.UpdateProject(ctx, projectID, input)
}

func normalizePagination(pagination Pagination) (Pagination, error) {
	if pagination.Page == 0 {
		pagination.Page = 1
	}
	if pagination.PageSize == 0 {
		pagination.PageSize = defaultPageSize
	}
	if pagination.Page < 1 {
		return Pagination{}, invalid("page must be at least 1")
	}
	if pagination.PageSize < 1 || pagination.PageSize > maxPageSize {
		return Pagination{}, invalid("page_size must be between 1 and 100")
	}
	return pagination, nil
}

func validateName(name string) error {
	if name == "" {
		return invalid("name is required")
	}
	if len([]rune(name)) > 120 {
		return invalid("name must not exceed 120 characters")
	}
	return nil
}

func validateCode(code string) error {
	if !codePattern.MatchString(code) {
		return invalid("code must contain 1-64 lowercase letters, numbers, or internal hyphens")
	}
	return nil
}

func validateStatus(status string) error {
	if status != StatusActive && status != StatusDisabled {
		return invalid("status must be active or disabled")
	}
	return nil
}

func validateID(id, field string) error {
	if !uuidPattern.MatchString(id) {
		return invalid(field + " must be a valid UUID")
	}
	return nil
}

func validateLabels(labels map[string]string) error {
	if len(labels) > maxLabels {
		return invalid("labels must not contain more than 50 entries")
	}
	for key, value := range labels {
		if strings.TrimSpace(key) == "" || len([]rune(key)) > 63 {
			return invalid("label keys must contain 1-63 characters")
		}
		if len([]rune(value)) > 256 {
			return invalid("label values must not exceed 256 characters")
		}
	}
	return nil
}

func cloneLabels(labels map[string]string) map[string]string {
	copy := make(map[string]string, len(labels))
	for key, value := range labels {
		copy[key] = value
	}
	return copy
}
