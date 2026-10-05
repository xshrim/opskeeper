package organization

import (
	"context"
	"opskeeper/backend/application"
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
	input.Description = strings.TrimSpace(input.Description)
	if err := validateName(input.Name); err != nil {
		return Team{}, err
	}
	if len([]rune(input.Description)) > 1000 {
		return Team{}, invalid("description must be at most 1000 characters")
	}
	input.Icon = normalizeIcon(input.Icon, "lucide:UsersRound")
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
	if input.Name == nil && input.Description == nil && input.Icon == nil && input.Labels == nil && input.Status == nil {
		return Team{}, invalid("at least one field must be provided")
	}
	if input.Name != nil {
		trimmed := strings.TrimSpace(*input.Name)
		if err := validateName(trimmed); err != nil {
			return Team{}, err
		}
		input.Name = &trimmed
	}
	if input.Description != nil {
		description := strings.TrimSpace(*input.Description)
		if len([]rune(description)) > 1000 {
			return Team{}, invalid("description must be at most 1000 characters")
		}
		input.Description = &description
	}
	if input.Icon != nil {
		icon := normalizeIcon(*input.Icon, "lucide:UsersRound")
		input.Icon = &icon
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
	input.Description = strings.TrimSpace(input.Description)
	if err := validateName(input.Name); err != nil {
		return Project{}, err
	}
	if err := validateCode(input.Code); err != nil {
		return Project{}, err
	}
	if len([]rune(input.Description)) > 1000 {
		return Project{}, invalid("description must be at most 1000 characters")
	}
	input.Icon = normalizeIcon(input.Icon, "lucide:FolderKanban")
	if err := validateLabels(input.Labels); err != nil {
		return Project{}, err
	}
	input.Labels = cloneLabels(input.Labels)
	for index, app := range input.Applications {
		prepared, err := application.PrepareCreateInput(app)
		if err != nil {
			return Project{}, invalid("application configuration is invalid")
		}
		if prepared.Labels == nil {
			prepared.Labels = map[string]string{}
		}
		input.Applications[index] = prepared
	}
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
	if input.Name == nil && input.Description == nil && input.Icon == nil && input.Labels == nil && input.Status == nil {
		return Project{}, invalid("at least one field must be provided")
	}
	if input.Name != nil {
		trimmed := strings.TrimSpace(*input.Name)
		if err := validateName(trimmed); err != nil {
			return Project{}, err
		}
		input.Name = &trimmed
	}
	if input.Description != nil {
		description := strings.TrimSpace(*input.Description)
		if len([]rune(description)) > 1000 {
			return Project{}, invalid("description must be at most 1000 characters")
		}
		input.Description = &description
	}
	if input.Icon != nil {
		icon := normalizeIcon(*input.Icon, "lucide:FolderKanban")
		input.Icon = &icon
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
