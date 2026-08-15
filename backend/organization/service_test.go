package organization

import (
	"context"
	"errors"
	"testing"
)

const testUUID = "11111111-1111-4111-8111-111111111111"

type stubRepository struct {
	createTeamInput    CreateTeamInput
	listTeamsInput     Pagination
	createProjectInput CreateProjectInput
	updateTeamInput    UpdateTeamInput
	called             bool
}

func (r *stubRepository) GetPlatform(context.Context) (Platform, error) {
	return Platform{}, nil
}

func (r *stubRepository) CreateTeam(_ context.Context, input CreateTeamInput) (Team, error) {
	r.called = true
	r.createTeamInput = input
	return Team{ID: testUUID, Name: input.Name, Code: input.Code, Labels: input.Labels}, nil
}

func (r *stubRepository) ListTeams(_ context.Context, pagination Pagination) (Page[Team], error) {
	r.listTeamsInput = pagination
	return Page[Team]{Page: pagination.Page, PageSize: pagination.PageSize}, nil
}

func (r *stubRepository) GetTeam(context.Context, string) (Team, error) {
	return Team{ID: testUUID}, nil
}

func (r *stubRepository) UpdateTeam(_ context.Context, _ string, input UpdateTeamInput) (Team, error) {
	r.updateTeamInput = input
	return Team{ID: testUUID}, nil
}

func (r *stubRepository) CreateProject(_ context.Context, input CreateProjectInput) (Project, error) {
	r.createProjectInput = input
	return Project{ID: testUUID, TeamID: input.TeamID, Source: input.Source}, nil
}

func (r *stubRepository) ListProjects(_ context.Context, _ string, pagination Pagination) (Page[Project], error) {
	return Page[Project]{Page: pagination.Page, PageSize: pagination.PageSize}, nil
}

func (r *stubRepository) GetProject(context.Context, string) (Project, error) {
	return Project{ID: testUUID}, nil
}

func (r *stubRepository) UpdateProject(context.Context, string, UpdateProjectInput) (Project, error) {
	return Project{ID: testUUID}, nil
}

func TestCreateTeamNormalizesInput(t *testing.T) {
	repository := &stubRepository{}
	service := NewService(repository)

	team, err := service.CreateTeam(context.Background(), CreateTeamInput{
		Name: "  Payments  ",
		Code: "payments",
		Labels: map[string]string{
			"environment": "production",
		},
	})
	if err != nil {
		t.Fatalf("CreateTeam() error = %v", err)
	}
	if team.Name != "Payments" || repository.createTeamInput.Name != "Payments" {
		t.Fatalf("CreateTeam() did not trim name: %#v", repository.createTeamInput)
	}
	if repository.createTeamInput.Labels == nil {
		t.Fatal("CreateTeam() passed nil labels")
	}
}

func TestCreateTeamRejectsInvalidCode(t *testing.T) {
	repository := &stubRepository{}
	service := NewService(repository)

	_, err := service.CreateTeam(context.Background(), CreateTeamInput{Name: "Payments", Code: "Payments API"})
	var validationError *ValidationError
	if !errors.As(err, &validationError) {
		t.Fatalf("CreateTeam() error = %v, want ValidationError", err)
	}
	if repository.called {
		t.Fatal("CreateTeam() called repository for invalid input")
	}
}

func TestListTeamsAppliesPaginationDefaults(t *testing.T) {
	repository := &stubRepository{}
	service := NewService(repository)

	if _, err := service.ListTeams(context.Background(), Pagination{}); err != nil {
		t.Fatalf("ListTeams() error = %v", err)
	}
	if repository.listTeamsInput.Page != 1 || repository.listTeamsInput.PageSize != 20 {
		t.Fatalf("ListTeams() pagination = %#v", repository.listTeamsInput)
	}
}

func TestCreateProjectDefaultsSource(t *testing.T) {
	repository := &stubRepository{}
	service := NewService(repository)

	_, err := service.CreateProject(context.Background(), CreateProjectInput{
		TeamID: testUUID,
		Name:   "Checkout",
		Code:   "checkout",
	})
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	if repository.createProjectInput.Source != "manual" {
		t.Fatalf("CreateProject() source = %q, want manual", repository.createProjectInput.Source)
	}
}

func TestUpdateTeamRequiresAtLeastOneField(t *testing.T) {
	repository := &stubRepository{}
	service := NewService(repository)

	_, err := service.UpdateTeam(context.Background(), testUUID, UpdateTeamInput{})
	var validationError *ValidationError
	if !errors.As(err, &validationError) {
		t.Fatalf("UpdateTeam() error = %v, want ValidationError", err)
	}
}
