package organization

import (
	"context"
	"errors"
	"testing"
)

const testUUID = "11111111-1111-4111-8111-111111111111"

type stubStore struct {
	createTeamInput    CreateTeamInput
	listTeamsInput     Pagination
	createProjectInput CreateProjectInput
	updateTeamInput    UpdateTeamInput
	called             bool
}

func (r *stubStore) GetPlatform(context.Context) (Platform, error) {
	return Platform{}, nil
}

func (r *stubStore) CreateTeam(_ context.Context, input CreateTeamInput) (Team, error) {
	r.called = true
	r.createTeamInput = input
	return Team{ID: testUUID, Name: input.Name, Code: input.Code, Labels: input.Labels}, nil
}

func (r *stubStore) ListTeams(_ context.Context, pagination Pagination) (Page[Team], error) {
	r.listTeamsInput = pagination
	return Page[Team]{Page: pagination.Page, PageSize: pagination.PageSize}, nil
}

func (r *stubStore) GetTeam(context.Context, string) (Team, error) {
	return Team{ID: testUUID}, nil
}

func (r *stubStore) UpdateTeam(_ context.Context, _ string, input UpdateTeamInput) (Team, error) {
	r.updateTeamInput = input
	return Team{ID: testUUID}, nil
}

func (r *stubStore) CreateProject(_ context.Context, input CreateProjectInput) (Project, error) {
	r.createProjectInput = input
	return Project{ID: testUUID, TeamID: input.TeamID, Source: input.Source}, nil
}

func (r *stubStore) ListProjects(_ context.Context, _ string, pagination Pagination) (Page[Project], error) {
	return Page[Project]{Page: pagination.Page, PageSize: pagination.PageSize}, nil
}

func (r *stubStore) GetProject(context.Context, string) (Project, error) {
	return Project{ID: testUUID}, nil
}

func (r *stubStore) UpdateProject(context.Context, string, UpdateProjectInput) (Project, error) {
	return Project{ID: testUUID}, nil
}

func TestCreateTeamNormalizesInput(t *testing.T) {
	store := &stubStore{}
	service := NewService(store)

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
	if team.Name != "Payments" || store.createTeamInput.Name != "Payments" {
		t.Fatalf("CreateTeam() did not trim name: %#v", store.createTeamInput)
	}
	if store.createTeamInput.Labels == nil {
		t.Fatal("CreateTeam() passed nil labels")
	}
	if store.createTeamInput.Icon != "team" {
		t.Fatalf("CreateTeam() icon = %q, want team", store.createTeamInput.Icon)
	}
}

func TestCreateProjectPreservesCustomIcon(t *testing.T) {
	store := &stubStore{}
	service := NewService(store)

	if _, err := service.CreateProject(context.Background(), CreateProjectInput{
		TeamID: testUUID,
		Name:   "Checkout",
		Code:   "checkout",
		Icon:   "rocket",
	}); err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	if store.createProjectInput.Icon != "rocket" {
		t.Fatalf("CreateProject() icon = %q, want rocket", store.createProjectInput.Icon)
	}
}

func TestCreateTeamRejectsInvalidCode(t *testing.T) {
	store := &stubStore{}
	service := NewService(store)

	_, err := service.CreateTeam(context.Background(), CreateTeamInput{Name: "Payments", Code: "Payments API"})
	var validationError *ValidationError
	if !errors.As(err, &validationError) {
		t.Fatalf("CreateTeam() error = %v, want ValidationError", err)
	}
	if store.called {
		t.Fatal("CreateTeam() called store for invalid input")
	}
}

func TestListTeamsAppliesPaginationDefaults(t *testing.T) {
	store := &stubStore{}
	service := NewService(store)

	if _, err := service.ListTeams(context.Background(), Pagination{}); err != nil {
		t.Fatalf("ListTeams() error = %v", err)
	}
	if store.listTeamsInput.Page != 1 || store.listTeamsInput.PageSize != 20 {
		t.Fatalf("ListTeams() pagination = %#v", store.listTeamsInput)
	}
}

func TestCreateProjectDefaultsSource(t *testing.T) {
	store := &stubStore{}
	service := NewService(store)

	_, err := service.CreateProject(context.Background(), CreateProjectInput{
		TeamID: testUUID,
		Name:   "Checkout",
		Code:   "checkout",
	})
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	if store.createProjectInput.Source != "manual" {
		t.Fatalf("CreateProject() source = %q, want manual", store.createProjectInput.Source)
	}
}

func TestUpdateTeamRequiresAtLeastOneField(t *testing.T) {
	store := &stubStore{}
	service := NewService(store)

	_, err := service.UpdateTeam(context.Background(), testUUID, UpdateTeamInput{})
	var validationError *ValidationError
	if !errors.As(err, &validationError) {
		t.Fatalf("UpdateTeam() error = %v, want ValidationError", err)
	}
}
