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
	GetInProject(context.Context, string, string) (application.Application, error)
	List(context.Context, string) ([]application.Application, error)
	UpdateInProject(context.Context, string, string, application.UpdateInput) (application.Application, error)
	DeleteInProject(context.Context, string, string) error
	CreateInstanceInProject(context.Context, string, string, application.CreateInstanceInput) (application.Instance, error)
	DeleteInstanceInProject(context.Context, string, string, string) error
	Workspace(context.Context, string) (application.Workspace, error)
	Topology(context.Context, string) (application.Topology, error)
}
type applicationBatchService interface {
	CreateMany(context.Context, []application.CreateInput) ([]application.Application, error)
}
type diagnosticApplicationCandidateService interface {
	DiagnosticCandidates(context.Context, string) ([]application.Application, error)
}
type applicationTopologyAnnotationService interface {
	CreateTopologyAnnotation(context.Context, string, application.TopologyAnnotationInput) (application.TopologyNode, error)
}
type applicationTopologyResourceService interface {
	SetTopologyPosition(context.Context, string, string, application.TopologyPositionInput) error
	AddTopologyResource(context.Context, string, application.TopologyResourceInput) error
	RemoveTopologyResource(context.Context, string, string) error
	DeleteTopologyAnnotation(context.Context, string, string) error
	CreateTopologyEdge(context.Context, string, application.TopologyEdgeInput) (application.TopologyEdge, error)
	DeleteTopologyEdge(context.Context, string, string) error
}
type applicationHandler struct {
	service     applicationService
	apiBasePath string
}
type createApplicationRequest struct {
	Name               string                              `json:"name"`
	Code               string                              `json:"code"`
	Description        string                              `json:"description"`
	Icon               string                              `json:"icon"`
	ExternalUID        string                              `json:"external_uid"`
	RuntimeKind        string                              `json:"runtime_kind"`
	Labels             map[string]string                   `json:"labels"`
	Critical           bool                                `json:"critical"`
	TechnicalOwnerIDs  []string                            `json:"technical_owner_ids"`
	OperationsOwnerIDs []string                            `json:"operations_owner_ids"`
	Instances          []application.CreateInstanceInput   `json:"instances"`
	Dependencies       []application.CreateDependencyInput `json:"dependencies"`
	Relations          []application.CreateRelationInput   `json:"relations"`
}
type createApplicationBatchRequest struct {
	Items []createApplicationRequest `json:"items"`
}
type createTopologyAnnotationRequest struct {
	Name     string         `json:"name"`
	Metadata map[string]any `json:"metadata"`
}
type topologyPositionRequest struct {
	X int `json:"x"`
	Y int `json:"y"`
}
type topologyEdgeRequest struct {
	SourceID string         `json:"source_id"`
	TargetID string         `json:"target_id"`
	Relation string         `json:"relation"`
	Metadata map[string]any `json:"metadata"`
}
type updateApplicationRequest struct {
	Name               *string                              `json:"name"`
	Code               *string                              `json:"code"`
	Description        *string                              `json:"description"`
	Icon               *string                              `json:"icon"`
	Status             *string                              `json:"status"`
	RuntimeKind        *string                              `json:"runtime_kind"`
	ExternalUID        *string                              `json:"external_uid"`
	Labels             *map[string]string                   `json:"labels"`
	Critical           *bool                                `json:"critical"`
	TechnicalOwnerIDs  *[]string                            `json:"technical_owner_ids"`
	OperationsOwnerIDs *[]string                            `json:"operations_owner_ids"`
	Instances          *[]application.CreateInstanceInput   `json:"instances"`
	Dependencies       *[]application.CreateDependencyInput `json:"dependencies"`
	Relations          *[]application.CreateRelationInput   `json:"relations"`
}
type createInstanceRequest struct {
	Name             string         `json:"name"`
	RuntimeKind      string         `json:"runtime_kind"`
	TargetResourceID string         `json:"target_resource_id"`
	Selector         map[string]any `json:"selector"`
	LogBinding       map[string]any `json:"log_binding"`
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
	router.With(guard(authorization.OrganizationRead)).Get("/projects/{projectID}/topology", h.topology)
	if _, ok := service.(applicationTopologyAnnotationService); ok {
		router.With(guard(authorization.ProjectManage)).Post("/projects/{projectID}/topology/annotations", h.createTopologyAnnotation)
	}
	if _, ok := service.(applicationTopologyResourceService); ok {
		router.With(guard(authorization.ProjectManage)).Put("/projects/{projectID}/topology/nodes/{nodeID}/position", h.setTopologyPosition)
		router.With(guard(authorization.ProjectManage)).Post("/projects/{projectID}/topology/resources", h.addTopologyResource)
		router.With(guard(authorization.ProjectManage)).Delete("/projects/{projectID}/topology/resources/{resourceID}", h.removeTopologyResource)
		router.With(guard(authorization.ProjectManage)).Delete("/projects/{projectID}/topology/annotations/{annotationID}", h.deleteTopologyAnnotation)
		router.With(guard(authorization.ProjectManage)).Post("/projects/{projectID}/topology/edges", h.createTopologyEdge)
		router.With(guard(authorization.ProjectManage)).Delete("/projects/{projectID}/topology/edges/{edgeID}", h.deleteTopologyEdge)
	}
	router.With(guard(authorization.OrganizationRead)).Get("/projects/{projectID}/applications", h.list)
	if _, ok := service.(diagnosticApplicationCandidateService); ok {
		router.With(guard(authorization.DiagnosisStart)).Get("/diagnosis/applications", h.diagnosticCandidates)
	}
	router.With(guard(authorization.ProjectManage)).Post("/projects/{projectID}/applications", h.create)
	if _, ok := service.(applicationBatchService); ok {
		router.With(guard(authorization.ProjectManage)).Post("/projects/{projectID}/applications/batch", h.createBatch)
	}
	router.With(guard(authorization.OrganizationRead)).Get("/projects/{projectID}/applications/{applicationID}/", h.get)
	router.With(guard(authorization.ProjectManage)).Patch("/projects/{projectID}/applications/{applicationID}/", h.update)
	router.With(guard(authorization.ProjectManage)).Delete("/projects/{projectID}/applications/{applicationID}/", h.delete)
	router.With(guard(authorization.ProjectManage)).Post("/projects/{projectID}/applications/{applicationID}/instances", h.createInstance)
	router.With(guard(authorization.ProjectManage)).Delete("/projects/{projectID}/applications/{applicationID}/instances/{instanceID}/", h.deleteInstance)
}
func (h applicationHandler) diagnosticCandidates(w http.ResponseWriter, r *http.Request) {
	service, ok := h.service.(diagnosticApplicationCandidateService)
	if !ok {
		writeApplicationError(w, r, application.ErrNotFound)
		return
	}
	items, err := service.DiagnosticCandidates(r.Context(), r.URL.Query().Get("scope_id"))
	if err != nil {
		writeApplicationError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}
func (h applicationHandler) setTopologyPosition(w http.ResponseWriter, r *http.Request) {
	service, ok := h.service.(applicationTopologyResourceService)
	if !ok {
		writeApplicationError(w, r, application.ErrInvalid)
		return
	}
	var body topologyPositionRequest
	if !decodeRequest(w, r, &body) {
		return
	}
	if err := service.SetTopologyPosition(r.Context(), chi.URLParam(r, "projectID"), chi.URLParam(r, "nodeID"), application.TopologyPositionInput{X: body.X, Y: body.Y}); err != nil {
		writeApplicationError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func (h applicationHandler) workspace(w http.ResponseWriter, r *http.Request) {
	v, e := h.service.Workspace(r.Context(), chi.URLParam(r, "projectID"))
	if e != nil {
		writeApplicationError(w, r, e)
		return
	}
	writeJSON(w, http.StatusOK, v)
}
func (h applicationHandler) topology(w http.ResponseWriter, r *http.Request) {
	v, e := h.service.Topology(r.Context(), chi.URLParam(r, "projectID"))
	if e != nil {
		writeApplicationError(w, r, e)
		return
	}
	writeJSON(w, http.StatusOK, v)
}
func (h applicationHandler) createTopologyAnnotation(w http.ResponseWriter, r *http.Request) {
	service, ok := h.service.(applicationTopologyAnnotationService)
	if !ok {
		writeApplicationError(w, r, application.ErrInvalid)
		return
	}
	var body createTopologyAnnotationRequest
	if !decodeRequest(w, r, &body) {
		return
	}
	item, err := service.CreateTopologyAnnotation(r.Context(), chi.URLParam(r, "projectID"), application.TopologyAnnotationInput{Name: body.Name, Metadata: body.Metadata})
	if err != nil {
		writeApplicationError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, item)
}
func (h applicationHandler) addTopologyResource(w http.ResponseWriter, r *http.Request) {
	service, ok := h.service.(applicationTopologyResourceService)
	if !ok {
		writeApplicationError(w, r, application.ErrInvalid)
		return
	}
	var body application.TopologyResourceInput
	if !decodeRequest(w, r, &body) {
		return
	}
	if err := service.AddTopologyResource(r.Context(), chi.URLParam(r, "projectID"), body); err != nil {
		writeApplicationError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func (h applicationHandler) removeTopologyResource(w http.ResponseWriter, r *http.Request) {
	service, ok := h.service.(applicationTopologyResourceService)
	if !ok {
		writeApplicationError(w, r, application.ErrInvalid)
		return
	}
	if err := service.RemoveTopologyResource(r.Context(), chi.URLParam(r, "projectID"), chi.URLParam(r, "resourceID")); err != nil {
		writeApplicationError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func (h applicationHandler) deleteTopologyAnnotation(w http.ResponseWriter, r *http.Request) {
	service, ok := h.service.(applicationTopologyResourceService)
	if !ok {
		writeApplicationError(w, r, application.ErrInvalid)
		return
	}
	if err := service.DeleteTopologyAnnotation(r.Context(), chi.URLParam(r, "projectID"), chi.URLParam(r, "annotationID")); err != nil {
		writeApplicationError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func (h applicationHandler) createTopologyEdge(w http.ResponseWriter, r *http.Request) {
	service, ok := h.service.(applicationTopologyResourceService)
	if !ok {
		writeApplicationError(w, r, application.ErrInvalid)
		return
	}
	var body topologyEdgeRequest
	if !decodeRequest(w, r, &body) {
		return
	}
	edge, err := service.CreateTopologyEdge(r.Context(), chi.URLParam(r, "projectID"), application.TopologyEdgeInput{SourceID: body.SourceID, TargetID: body.TargetID, Relation: body.Relation, Metadata: body.Metadata})
	if err != nil {
		writeApplicationError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, edge)
}
func (h applicationHandler) deleteTopologyEdge(w http.ResponseWriter, r *http.Request) {
	service, ok := h.service.(applicationTopologyResourceService)
	if !ok {
		writeApplicationError(w, r, application.ErrInvalid)
		return
	}
	if err := service.DeleteTopologyEdge(r.Context(), chi.URLParam(r, "projectID"), chi.URLParam(r, "edgeID")); err != nil {
		writeApplicationError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
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
	v, e := h.service.Create(r.Context(), application.CreateInput{ProjectID: chi.URLParam(r, "projectID"), Name: b.Name, Code: b.Code, Description: b.Description, Icon: b.Icon, RuntimeKind: b.RuntimeKind, ExternalUID: b.ExternalUID, Labels: b.Labels, Critical: b.Critical, TechnicalOwnerIDs: b.TechnicalOwnerIDs, OperationsOwnerIDs: b.OperationsOwnerIDs, Instances: b.Instances, Dependencies: b.Dependencies, Relations: b.Relations})
	if e != nil {
		writeApplicationError(w, r, e)
		return
	}
	w.Header().Set("Location", h.apiBasePath+"/projects/"+chi.URLParam(r, "projectID")+"/applications/"+v.ID)
	writeJSON(w, http.StatusCreated, v)
}
func (h applicationHandler) createBatch(w http.ResponseWriter, r *http.Request) {
	var body createApplicationBatchRequest
	if !decodeRequest(w, r, &body) {
		return
	}
	service, ok := h.service.(applicationBatchService)
	if !ok {
		writeApplicationError(w, r, application.ErrInvalid)
		return
	}
	inputs := make([]application.CreateInput, 0, len(body.Items))
	for _, item := range body.Items {
		inputs = append(inputs, application.CreateInput{
			ProjectID: chi.URLParam(r, "projectID"), Name: item.Name, Code: item.Code,
			Description: item.Description, Icon: item.Icon, RuntimeKind: item.RuntimeKind,
			ExternalUID: item.ExternalUID, Labels: item.Labels, Critical: item.Critical,
			TechnicalOwnerIDs: item.TechnicalOwnerIDs, OperationsOwnerIDs: item.OperationsOwnerIDs,
			Instances: item.Instances, Dependencies: item.Dependencies,
			Relations: item.Relations,
		})
	}
	created, err := service.CreateMany(r.Context(), inputs)
	if err != nil {
		writeApplicationError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"items": created})
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
	v, e := h.service.UpdateInProject(r.Context(), chi.URLParam(r, "projectID"), chi.URLParam(r, "applicationID"), application.UpdateInput{Name: b.Name, Code: b.Code, Description: b.Description, Icon: b.Icon, Status: b.Status, RuntimeKind: b.RuntimeKind, ExternalUID: b.ExternalUID, Labels: b.Labels, Critical: b.Critical, TechnicalOwnerIDs: b.TechnicalOwnerIDs, OperationsOwnerIDs: b.OperationsOwnerIDs, Instances: b.Instances, Dependencies: b.Dependencies, Relations: b.Relations})
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
func writeApplicationError(w http.ResponseWriter, r *http.Request, e error) {
	if errors.Is(e, application.ErrInvalid) {
		writeError(w, r, http.StatusBadRequest, "invalid_request", e.Error())
		return
	}
	if errors.Is(e, application.ErrNotFound) {
		writeError(w, r, http.StatusNotFound, "not_found", e.Error())
		return
	}
	writeInternalError(w, r, "application operation", e)
}
