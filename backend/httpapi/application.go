package httpapi

import (
	"context"
	"errors"
	"github.com/go-chi/chi/v5"
	"net/http"
	"opskeeper/backend/application"
	"opskeeper/backend/authorization"
)

type applicationService interface {
	Create(context.Context, application.CreateInput) (application.Application, error)
	Import(context.Context, application.ImportInput) (application.Application, error)
	GetInProject(context.Context, string, string) (application.Application, error)
	List(context.Context, string) ([]application.Application, error)
	UpdateInProject(context.Context, string, string, application.UpdateInput) (application.Application, error)
	DeleteInProject(context.Context, string, string) error
	CreateInstanceInProject(context.Context, string, string, application.CreateInstanceInput) (application.Instance, error)
	DeleteInstanceInProject(context.Context, string, string, string) error
	CreateDependencyInProject(context.Context, string, string, application.CreateDependencyInput) (application.Dependency, error)
	DeleteDependencyInProject(context.Context, string, string, string) error
	Workspace(context.Context, string) (application.Workspace, error)
}
type applicationHandler struct {
	service     applicationService
	apiBasePath string
}
type createApplicationRequest struct {
	Name         string                              `json:"name"`
	Code         string                              `json:"code"`
	Description  string                              `json:"description"`
	Icon         string                              `json:"icon"`
	Source       string                              `json:"source"`
	ExternalUID  string                              `json:"external_uid"`
	Labels       map[string]string                   `json:"labels"`
	Instances    []application.CreateInstanceInput   `json:"instances"`
	Dependencies []application.CreateDependencyInput `json:"dependencies"`
}
type updateApplicationRequest struct {
	Name        *string            `json:"name"`
	Description *string            `json:"description"`
	Icon        *string            `json:"icon"`
	Status      *string            `json:"status"`
	Labels      *map[string]string `json:"labels"`
}
type createInstanceRequest struct {
	Name             string         `json:"name"`
	RuntimeKind      string         `json:"runtime_kind"`
	TargetResourceID string         `json:"target_resource_id"`
	Selector         map[string]any `json:"selector"`
	LogBinding       map[string]any `json:"log_binding"`
	Status           string         `json:"status"`
}
type createDependencyRequest struct {
	TargetResourceID string         `json:"target_resource_id"`
	DependencyKind   string         `json:"dependency_kind"`
	Binding          map[string]any `json:"binding"`
	Required         *bool          `json:"required"`
	Status           string         `json:"status"`
}

func registerApplicationRoutes(router chi.Router, service applicationService, apiBasePath string, requirePermission func(authorization.Permission) func(http.Handler) http.Handler) {
	h := applicationHandler{service: service, apiBasePath: apiBasePath}
	guard := func(p authorization.Permission) func(http.Handler) http.Handler {
		if requirePermission == nil {
			return func(n http.Handler) http.Handler { return n }
		}
		return requirePermission(p)
	}
	router.With(guard(authorization.OrganizationRead)).Get("/projects/{projectID}/workspace", h.workspace)
	router.With(guard(authorization.OrganizationRead)).Get("/projects/{projectID}/applications", h.list)
	router.With(guard(authorization.ProjectManage)).Post("/projects/{projectID}/applications", h.create)
	router.With(guard(authorization.ProjectManage)).Post("/projects/{projectID}/applications/import", h.importApplication)
	router.With(guard(authorization.OrganizationRead)).Get("/projects/{projectID}/applications/{applicationID}/", h.get)
	router.With(guard(authorization.ProjectManage)).Patch("/projects/{projectID}/applications/{applicationID}/", h.update)
	router.With(guard(authorization.ProjectManage)).Delete("/projects/{projectID}/applications/{applicationID}/", h.delete)
	router.With(guard(authorization.ProjectManage)).Post("/projects/{projectID}/applications/{applicationID}/instances", h.createInstance)
	router.With(guard(authorization.ProjectManage)).Post("/projects/{projectID}/applications/{applicationID}/dependencies", h.createDependency)
	router.With(guard(authorization.ProjectManage)).Delete("/projects/{projectID}/applications/{applicationID}/instances/{instanceID}/", h.deleteInstance)
	router.With(guard(authorization.ProjectManage)).Delete("/projects/{projectID}/applications/{applicationID}/dependencies/{dependencyID}/", h.deleteDependency)
}
func (h applicationHandler) workspace(w http.ResponseWriter, r *http.Request) {
	v, e := h.service.Workspace(r.Context(), chi.URLParam(r, "projectID"))
	if e != nil {
		writeApplicationError(w, r, e)
		return
	}
	writeJSON(w, http.StatusOK, v)
}
func (h applicationHandler) list(w http.ResponseWriter, r *http.Request) {
	v, e := h.service.List(r.Context(), chi.URLParam(r, "projectID"))
	if e != nil {
		writeApplicationError(w, r, e)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": v})
}
func (h applicationHandler) create(w http.ResponseWriter, r *http.Request) {
	var b createApplicationRequest
	if !decodeRequest(w, r, &b) {
		return
	}
	v, e := h.service.Create(r.Context(), application.CreateInput{ProjectID: chi.URLParam(r, "projectID"), Name: b.Name, Code: b.Code, Description: b.Description, Icon: b.Icon, Source: b.Source, ExternalUID: b.ExternalUID, Labels: b.Labels, Instances: b.Instances, Dependencies: b.Dependencies})
	if e != nil {
		writeApplicationError(w, r, e)
		return
	}
	w.Header().Set("Location", h.apiBasePath+"/projects/"+chi.URLParam(r, "projectID")+"/applications/"+v.ID)
	writeJSON(w, http.StatusCreated, v)
}
func (h applicationHandler) importApplication(w http.ResponseWriter, r *http.Request) {
	var b application.ImportInput
	if !decodeRequest(w, r, &b) {
		return
	}
	b.ProjectID = chi.URLParam(r, "projectID")
	v, err := h.service.Import(r.Context(), b)
	if err != nil {
		writeApplicationError(w, r, err)
		return
	}
	w.Header().Set("Location", h.apiBasePath+"/projects/"+b.ProjectID+"/applications/"+v.ID)
	writeJSON(w, http.StatusCreated, v)
}
func (h applicationHandler) get(w http.ResponseWriter, r *http.Request) {
	v, e := h.service.GetInProject(r.Context(), chi.URLParam(r, "projectID"), chi.URLParam(r, "applicationID"))
	if e != nil {
		writeApplicationError(w, r, e)
		return
	}
	writeJSON(w, http.StatusOK, v)
}
func (h applicationHandler) update(w http.ResponseWriter, r *http.Request) {
	var b updateApplicationRequest
	if !decodeRequest(w, r, &b) {
		return
	}
	v, e := h.service.UpdateInProject(r.Context(), chi.URLParam(r, "projectID"), chi.URLParam(r, "applicationID"), application.UpdateInput{Name: b.Name, Description: b.Description, Icon: b.Icon, Status: b.Status, Labels: b.Labels})
	if e != nil {
		writeApplicationError(w, r, e)
		return
	}
	writeJSON(w, http.StatusOK, v)
}
func (h applicationHandler) delete(w http.ResponseWriter, r *http.Request) {
	if e := h.service.DeleteInProject(r.Context(), chi.URLParam(r, "projectID"), chi.URLParam(r, "applicationID")); e != nil {
		writeApplicationError(w, r, e)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func (h applicationHandler) createInstance(w http.ResponseWriter, r *http.Request) {
	var b createInstanceRequest
	if !decodeRequest(w, r, &b) {
		return
	}
	v, e := h.service.CreateInstanceInProject(r.Context(), chi.URLParam(r, "projectID"), chi.URLParam(r, "applicationID"), application.CreateInstanceInput{Name: b.Name, RuntimeKind: b.RuntimeKind, TargetResourceID: b.TargetResourceID, Selector: b.Selector, LogBinding: b.LogBinding, Status: b.Status})
	if e != nil {
		writeApplicationError(w, r, e)
		return
	}
	writeJSON(w, http.StatusCreated, v)
}
func (h applicationHandler) deleteInstance(w http.ResponseWriter, r *http.Request) {
	if e := h.service.DeleteInstanceInProject(r.Context(), chi.URLParam(r, "projectID"), chi.URLParam(r, "applicationID"), chi.URLParam(r, "instanceID")); e != nil {
		writeApplicationError(w, r, e)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func (h applicationHandler) createDependency(w http.ResponseWriter, r *http.Request) {
	var b createDependencyRequest
	if !decodeRequest(w, r, &b) {
		return
	}
	required := true
	if b.Required != nil {
		required = *b.Required
	}
	v, e := h.service.CreateDependencyInProject(r.Context(), chi.URLParam(r, "projectID"), chi.URLParam(r, "applicationID"), application.CreateDependencyInput{TargetResourceID: b.TargetResourceID, DependencyKind: b.DependencyKind, Binding: b.Binding, Required: required, Status: b.Status})
	if e != nil {
		writeApplicationError(w, r, e)
		return
	}
	writeJSON(w, http.StatusCreated, v)
}
func (h applicationHandler) deleteDependency(w http.ResponseWriter, r *http.Request) {
	if e := h.service.DeleteDependencyInProject(r.Context(), chi.URLParam(r, "projectID"), chi.URLParam(r, "applicationID"), chi.URLParam(r, "dependencyID")); e != nil {
		writeApplicationError(w, r, e)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func writeApplicationError(w http.ResponseWriter, r *http.Request, e error) {
	if errors.Is(e, application.ErrInvalid) {
		writeError(w, r, http.StatusBadRequest, "invalid_request", e.Error())
		return
	}
	if errors.Is(e, application.ErrNotFound) {
		writeError(w, r, http.StatusNotFound, "not_found", e.Error())
		return
	}
	writeError(w, r, http.StatusInternalServerError, "internal_error", "Internal server error")
}
