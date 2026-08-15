package httpapi

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"opskeeper/backend/health"
	"opskeeper/backend/organization"
)

const handlerTestUUID = "11111111-1111-4111-8111-111111111111"
const handlerTestBasePath = "/opskeeper"

type stubOrganizationService struct {
	createTeamInput organization.CreateTeamInput
	createTeamErr   error
}

func (s *stubOrganizationService) GetPlatform(context.Context) (organization.Platform, error) {
	return organization.Platform{ID: handlerTestUUID, Name: "OpsKeeper"}, nil
}

func (s *stubOrganizationService) CreateTeam(_ context.Context, input organization.CreateTeamInput) (organization.Team, error) {
	s.createTeamInput = input
	if s.createTeamErr != nil {
		return organization.Team{}, s.createTeamErr
	}
	return organization.Team{ID: handlerTestUUID, Name: input.Name, Code: input.Code}, nil
}

func (s *stubOrganizationService) ListTeams(_ context.Context, pagination organization.Pagination) (organization.Page[organization.Team], error) {
	return organization.Page[organization.Team]{Items: []organization.Team{}, Page: pagination.Page, PageSize: pagination.PageSize}, nil
}

func (s *stubOrganizationService) GetTeam(context.Context, string) (organization.Team, error) {
	return organization.Team{ID: handlerTestUUID}, nil
}

func (s *stubOrganizationService) UpdateTeam(context.Context, string, organization.UpdateTeamInput) (organization.Team, error) {
	return organization.Team{ID: handlerTestUUID}, nil
}

func (s *stubOrganizationService) CreateProject(context.Context, organization.CreateProjectInput) (organization.Project, error) {
	return organization.Project{ID: handlerTestUUID}, nil
}

func (s *stubOrganizationService) ListProjects(_ context.Context, _ string, pagination organization.Pagination) (organization.Page[organization.Project], error) {
	return organization.Page[organization.Project]{Items: []organization.Project{}, Page: pagination.Page, PageSize: pagination.PageSize}, nil
}

func (s *stubOrganizationService) GetProject(context.Context, string) (organization.Project, error) {
	return organization.Project{ID: handlerTestUUID}, nil
}

func (s *stubOrganizationService) UpdateProject(context.Context, string, organization.UpdateProjectInput) (organization.Project, error) {
	return organization.Project{ID: handlerTestUUID}, nil
}

func newOrganizationTestRouter(service organizationService) http.Handler {
	return newOrganizationTestRouterAt(handlerTestBasePath, service)
}

func newOrganizationTestRouterAt(basePath string, service organizationService) http.Handler {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return NewRouter(logger, health.NewService("opskeeper-api", time.Second, nil), testBuild, Options{BasePath: basePath}, service, nil)
}

func TestCreateTeam(t *testing.T) {
	service := &stubOrganizationService{}
	router := newOrganizationTestRouter(service)
	request := httptest.NewRequest(http.MethodPost, handlerTestBasePath+"/api/v1/teams", strings.NewReader(`{"name":"Payments","code":"payments","labels":{"tier":"critical"}}`))
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusCreated {
		t.Fatalf("POST /api/v1/teams status = %d, body = %s", response.Code, response.Body.String())
	}
	if service.createTeamInput.Code != "payments" {
		t.Fatalf("CreateTeam() input = %#v", service.createTeamInput)
	}
	if response.Header().Get("Location") != handlerTestBasePath+"/api/v1/teams/"+handlerTestUUID {
		t.Fatalf("Location = %q", response.Header().Get("Location"))
	}
}

func TestCreateTeamAtRootBasePath(t *testing.T) {
	service := &stubOrganizationService{}
	router := newOrganizationTestRouterAt("/", service)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/teams", strings.NewReader(`{"name":"Payments","code":"payments"}`))
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusCreated {
		t.Fatalf("POST /api/v1/teams status = %d, body = %s", response.Code, response.Body.String())
	}
	if response.Header().Get("Location") != "/api/v1/teams/"+handlerTestUUID {
		t.Fatalf("Location = %q", response.Header().Get("Location"))
	}
}

func TestCreateTeamRejectsUnknownField(t *testing.T) {
	router := newOrganizationTestRouter(&stubOrganizationService{})
	request := httptest.NewRequest(http.MethodPost, handlerTestBasePath+"/api/v1/teams", strings.NewReader(`{"name":"Payments","code":"payments","owner":"ignored"}`))
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("POST /api/v1/teams status = %d, body = %s", response.Code, response.Body.String())
	}
}

func TestListTeamsRejectsInvalidPagination(t *testing.T) {
	router := newOrganizationTestRouter(&stubOrganizationService{})
	request := httptest.NewRequest(http.MethodGet, handlerTestBasePath+"/api/v1/teams?page=zero", nil)
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("GET /api/v1/teams status = %d, body = %s", response.Code, response.Body.String())
	}
}

func TestCreateTeamMapsConflict(t *testing.T) {
	service := &stubOrganizationService{createTeamErr: organization.ErrConflict}
	router := newOrganizationTestRouter(service)
	request := httptest.NewRequest(http.MethodPost, handlerTestBasePath+"/api/v1/teams", strings.NewReader(`{"name":"Payments","code":"payments"}`))
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusConflict || !strings.Contains(response.Body.String(), `"code":"conflict"`) {
		t.Fatalf("POST /api/v1/teams response = %d %s", response.Code, response.Body.String())
	}
}
