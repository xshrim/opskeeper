package organization

import (
	"context"
	"strings"
)

type Service struct {
	store Store
}

func NewService(store Store) *Service {
	return &Service{store: store}
}

func (s *Service) GetPlatform(ctx context.Context) (Platform, error) {
	return s.store.GetPlatform(ctx)
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
	return s.store.CreateTeam(ctx, input)
}

func (s *Service) ListTeams(ctx context.Context, pagination Pagination) (Page[Team], error) {
	pagination, err := normalizePagination(pagination)
	if err != nil {
		return Page[Team]{}, err
	}
	return s.store.ListTeams(ctx, pagination)
}

func (s *Service) GetTeam(ctx context.Context, teamID string) (Team, error) {
	if err := validateID(teamID, "team_id"); err != nil {
		return Team{}, err
	}
	return s.store.GetTeam(ctx, teamID)
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
	return s.store.UpdateTeam(ctx, teamID, input)
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
	return s.store.CreateProject(ctx, input)
}

func (s *Service) ListProjects(ctx context.Context, teamID string, pagination Pagination) (Page[Project], error) {
	if err := validateID(teamID, "team_id"); err != nil {
		return Page[Project]{}, err
	}
	pagination, err := normalizePagination(pagination)
	if err != nil {
		return Page[Project]{}, err
	}
	return s.store.ListProjects(ctx, teamID, pagination)
}

func (s *Service) GetProject(ctx context.Context, projectID string) (Project, error) {
	if err := validateID(projectID, "project_id"); err != nil {
		return Project{}, err
	}
	return s.store.GetProject(ctx, projectID)
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
	return s.store.UpdateProject(ctx, projectID, input)
}
