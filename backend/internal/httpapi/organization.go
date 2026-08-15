package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/opskeeper/opskeeper/backend/internal/organization"
)

const maxRequestBodyBytes = 1 << 20

type organizationService interface {
	GetPlatform(context.Context) (organization.Platform, error)
	CreateTeam(context.Context, organization.CreateTeamInput) (organization.Team, error)
	ListTeams(context.Context, organization.Pagination) (organization.Page[organization.Team], error)
	GetTeam(context.Context, string) (organization.Team, error)
	UpdateTeam(context.Context, string, organization.UpdateTeamInput) (organization.Team, error)
	CreateProject(context.Context, organization.CreateProjectInput) (organization.Project, error)
	ListProjects(context.Context, string, organization.Pagination) (organization.Page[organization.Project], error)
	GetProject(context.Context, string) (organization.Project, error)
	UpdateProject(context.Context, string, organization.UpdateProjectInput) (organization.Project, error)
}

type organizationHandler struct {
	service organizationService
}

type createTeamRequest struct {
	Name   string            `json:"name"`
	Code   string            `json:"code"`
	Labels map[string]string `json:"labels"`
}

type createProjectRequest struct {
	Name   string            `json:"name"`
	Code   string            `json:"code"`
	Labels map[string]string `json:"labels"`
	Source string            `json:"source,omitempty"`
}

type updateOrganizationRequest struct {
	Name   *string            `json:"name"`
	Labels *map[string]string `json:"labels"`
	Status *string            `json:"status"`
}

func registerOrganizationRoutes(router chi.Router, service organizationService) {
	handler := organizationHandler{service: service}
	router.Get("/platform", handler.getPlatform)
	router.Route("/teams", func(router chi.Router) {
		router.Get("/", handler.listTeams)
		router.Post("/", handler.createTeam)
		router.Route("/{teamID}", func(router chi.Router) {
			router.Get("/", handler.getTeam)
			router.Patch("/", handler.updateTeam)
			router.Get("/projects", handler.listProjects)
			router.Post("/projects", handler.createProject)
		})
	})
	router.Route("/projects/{projectID}", func(router chi.Router) {
		router.Get("/", handler.getProject)
		router.Patch("/", handler.updateProject)
	})
}

func (h organizationHandler) getPlatform(writer http.ResponseWriter, request *http.Request) {
	platform, err := h.service.GetPlatform(request.Context())
	if err != nil {
		writeOrganizationError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, platform)
}

func (h organizationHandler) createTeam(writer http.ResponseWriter, request *http.Request) {
	var body createTeamRequest
	if !decodeRequest(writer, request, &body) {
		return
	}
	team, err := h.service.CreateTeam(request.Context(), organization.CreateTeamInput{
		Name:   body.Name,
		Code:   body.Code,
		Labels: body.Labels,
	})
	if err != nil {
		writeOrganizationError(writer, request, err)
		return
	}
	writer.Header().Set("Location", "/api/v1/teams/"+team.ID)
	writeJSON(writer, http.StatusCreated, team)
}

func (h organizationHandler) listTeams(writer http.ResponseWriter, request *http.Request) {
	pagination, ok := parsePagination(writer, request)
	if !ok {
		return
	}
	page, err := h.service.ListTeams(request.Context(), pagination)
	if err != nil {
		writeOrganizationError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, page)
}

func (h organizationHandler) getTeam(writer http.ResponseWriter, request *http.Request) {
	team, err := h.service.GetTeam(request.Context(), chi.URLParam(request, "teamID"))
	if err != nil {
		writeOrganizationError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, team)
}

func (h organizationHandler) updateTeam(writer http.ResponseWriter, request *http.Request) {
	var body updateOrganizationRequest
	if !decodeRequest(writer, request, &body) {
		return
	}
	team, err := h.service.UpdateTeam(request.Context(), chi.URLParam(request, "teamID"), organization.UpdateTeamInput{
		Name:   body.Name,
		Labels: body.Labels,
		Status: body.Status,
	})
	if err != nil {
		writeOrganizationError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, team)
}

func (h organizationHandler) createProject(writer http.ResponseWriter, request *http.Request) {
	var body createProjectRequest
	if !decodeRequest(writer, request, &body) {
		return
	}
	project, err := h.service.CreateProject(request.Context(), organization.CreateProjectInput{
		TeamID: chi.URLParam(request, "teamID"),
		Name:   body.Name,
		Code:   body.Code,
		Labels: body.Labels,
		Source: body.Source,
	})
	if err != nil {
		writeOrganizationError(writer, request, err)
		return
	}
	writer.Header().Set("Location", "/api/v1/projects/"+project.ID)
	writeJSON(writer, http.StatusCreated, project)
}

func (h organizationHandler) listProjects(writer http.ResponseWriter, request *http.Request) {
	pagination, ok := parsePagination(writer, request)
	if !ok {
		return
	}
	page, err := h.service.ListProjects(request.Context(), chi.URLParam(request, "teamID"), pagination)
	if err != nil {
		writeOrganizationError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, page)
}

func (h organizationHandler) getProject(writer http.ResponseWriter, request *http.Request) {
	project, err := h.service.GetProject(request.Context(), chi.URLParam(request, "projectID"))
	if err != nil {
		writeOrganizationError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, project)
}

func (h organizationHandler) updateProject(writer http.ResponseWriter, request *http.Request) {
	var body updateOrganizationRequest
	if !decodeRequest(writer, request, &body) {
		return
	}
	project, err := h.service.UpdateProject(request.Context(), chi.URLParam(request, "projectID"), organization.UpdateProjectInput{
		Name:   body.Name,
		Labels: body.Labels,
		Status: body.Status,
	})
	if err != nil {
		writeOrganizationError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, project)
}

func decodeRequest(writer http.ResponseWriter, request *http.Request, destination any) bool {
	request.Body = http.MaxBytesReader(writer, request.Body, maxRequestBodyBytes)
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		writeError(writer, request, http.StatusBadRequest, "invalid_json", "Request body must contain a valid JSON object")
		return false
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		writeError(writer, request, http.StatusBadRequest, "invalid_json", "Request body must contain a single JSON object")
		return false
	}
	return true
}

func parsePagination(writer http.ResponseWriter, request *http.Request) (organization.Pagination, bool) {
	page, err := parseOptionalPositiveInt(request.URL.Query().Get("page"))
	if err != nil {
		writeError(writer, request, http.StatusBadRequest, "invalid_request", "page must be a positive integer")
		return organization.Pagination{}, false
	}
	pageSize, err := parseOptionalPositiveInt(request.URL.Query().Get("page_size"))
	if err != nil {
		writeError(writer, request, http.StatusBadRequest, "invalid_request", "page_size must be a positive integer")
		return organization.Pagination{}, false
	}
	return organization.Pagination{Page: page, PageSize: pageSize}, true
}

func parseOptionalPositiveInt(value string) (int, error) {
	if value == "" {
		return 0, nil
	}
	number, err := strconv.Atoi(value)
	if err != nil || number < 1 {
		return 0, errors.New("not a positive integer")
	}
	return number, nil
}

func writeOrganizationError(writer http.ResponseWriter, request *http.Request, err error) {
	var validationError *organization.ValidationError
	switch {
	case errors.As(err, &validationError):
		writeError(writer, request, http.StatusBadRequest, "invalid_request", validationError.Message)
	case errors.Is(err, organization.ErrNotFound):
		writeError(writer, request, http.StatusNotFound, "not_found", "Organization not found")
	case errors.Is(err, organization.ErrConflict):
		writeError(writer, request, http.StatusConflict, "conflict", "Organization conflicts with existing data")
	case errors.Is(err, organization.ErrParentInactive):
		writeError(writer, request, http.StatusConflict, "parent_inactive", "Parent organization is inactive")
	default:
		writeError(writer, request, http.StatusInternalServerError, "internal_error", "Internal server error")
	}
}
