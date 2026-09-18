package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"opskeeper/backend/audit"
	"opskeeper/backend/authorization"
	"opskeeper/backend/resource"
)

type resourceService interface {
	Create(context.Context, resource.CreateInput) (resource.Resource, error)
	List(context.Context, resource.Pagination, string, map[string]string) (resource.Page[resource.Resource], error)
	Get(context.Context, string) (resource.Resource, error)
	Update(context.Context, string, resource.UpdateInput) (resource.Resource, error)
	Delete(context.Context, string) error
	ListSchemas(context.Context) ([]resource.Schema, error)
	CreateRelation(context.Context, string, resource.CreateRelationInput) (resource.Relation, error)
	ListRelations(context.Context, string) ([]resource.Relation, error)
	DeleteRelation(context.Context, string, string) error
	Topology(context.Context, string, int, int) ([]resource.TopologyNode, error)
	SetDefault(context.Context, string, string, string) (resource.Default, error)
	ResolveDefault(context.Context, string, string) (resource.Resource, error)
}

type resourceHandler struct {
	resources resourceService
	auditor   audit.Logger
}

type createResourceRequest struct {
	ScopeID          string                     `json:"scope_id"`
	Kind             string                     `json:"kind"`
	Subtype          string                     `json:"subtype,omitempty"`
	AgentRef         *string                    `json:"agent_ref,omitempty"`
	SchemaVersion    int                        `json:"schema_version,omitempty"`
	Name             string                     `json:"name"`
	ExternalUID      string                     `json:"external_uid,omitempty"`
	SourceResourceID string                     `json:"source_resource_id,omitempty"`
	Labels           map[string]string          `json:"labels"`
	Config           map[string]any             `json:"config"`
	Status           string                     `json:"status,omitempty"`
	Credential       *resourceCredentialRequest `json:"credential,omitempty"`
}

type updateResourceRequest struct {
	ScopeID          *string                 `json:"scope_id"`
	Subtype          *string                 `json:"subtype"`
	AgentRef         **string                `json:"agent_ref"`
	Name             *string                 `json:"name"`
	ExternalUID      *string                 `json:"external_uid"`
	SourceResourceID *string                 `json:"source_resource_id"`
	Labels           *map[string]string      `json:"labels"`
	Config           *map[string]any         `json:"config"`
	Status           *string                 `json:"status"`
	Credential       nullableCredentialPatch `json:"credential"`
}

type resourceCredentialRequest struct {
	Purpose *string `json:"purpose,omitempty"`
	Secret  string  `json:"secret"`
}

type nullableCredentialPatch struct {
	Set   bool
	Value *resourceCredentialRequest
}

func (patch *nullableCredentialPatch) UnmarshalJSON(data []byte) error {
	patch.Set = true
	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		return nil
	}
	var value resourceCredentialRequest
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	patch.Value = &value
	return nil
}

type createRelationRequest struct {
	TargetResourceID string         `json:"target_resource_id"`
	RelationType     string         `json:"relation_type"`
	Attributes       map[string]any `json:"attributes"`
	DiscoverySource  string         `json:"discovery_source,omitempty"`
	Confidence       float64        `json:"confidence,omitempty"`
	Confirmed        *bool          `json:"confirmed"`
}

type setDefaultRequest struct {
	ScopeID    string `json:"scope_id"`
	DefaultKey string `json:"default_key"`
	ResourceID string `json:"resource_id"`
}

func registerResourceRoutes(router chi.Router, services resourceService, auditor audit.Logger, requirePermission func(authorization.Permission) func(http.Handler) http.Handler) {
	handler := resourceHandler{resources: services, auditor: auditor}
	guard := func(permission authorization.Permission) func(http.Handler) http.Handler {
		if requirePermission == nil {
			return func(next http.Handler) http.Handler { return next }
		}
		return requirePermission(permission)
	}
	if services != nil {
		// Context selection must only expose resources the caller can use.
		router.With(guard(authorization.ResourceUse)).Get("/resources/context", handler.listResources)
		router.With(guard(authorization.ResourceRead)).Get("/resources", handler.listResources)
		router.With(guard(authorization.ResourceCreate)).Post("/resources", handler.createResource)
		router.With(guard(authorization.ResourceRead)).Get("/resources/schemas", handler.listSchemas)
		router.With(guard(authorization.ResourceUpdate)).Put("/resource-defaults", handler.setDefault)
		router.With(guard(authorization.ResourceRead)).Get("/resource-defaults", handler.resolveDefault)
		router.Route("/resources/{resourceID}", func(router chi.Router) {
			router.With(guard(authorization.ResourceRead)).Get("/", handler.getResource)
			router.With(guard(authorization.ResourceUpdate)).Patch("/", handler.updateResource)
			router.With(guard(authorization.ResourceDelete)).Delete("/", handler.deleteResource)
			router.With(guard(authorization.RelationManage)).Get("/relations", handler.listRelations)
			router.With(guard(authorization.RelationManage)).Post("/relations", handler.createRelation)
			router.With(guard(authorization.RelationManage)).Delete("/relations/{relationID}", handler.deleteRelation)
			router.With(guard(authorization.ResourceRead)).Get("/topology", handler.topology)
		})
	}
}

func (h resourceHandler) listResources(writer http.ResponseWriter, request *http.Request) {
	pagination, ok := parseResourcePagination(writer, request)
	if !ok {
		return
	}
	labels := make(map[string]string)
	for key, values := range request.URL.Query() {
		if len(key) > len("label_") && len(values) > 0 && len(key) > 6 && key[:6] == "label_" {
			labels[key[6:]] = values[0]
		}
	}
	items, err := h.resources.List(request.Context(), pagination, request.URL.Query().Get("kind"), labels)
	if err != nil {
		writeResourceError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, items)
}

func (h resourceHandler) createResource(writer http.ResponseWriter, request *http.Request) {
	var body createResourceRequest
	if !decodeRequest(writer, request, &body) {
		return
	}
	input := resource.CreateInput{ScopeID: body.ScopeID, Kind: body.Kind, Subtype: body.Subtype, AgentRef: body.AgentRef, SchemaVersion: body.SchemaVersion, Name: body.Name, ExternalUID: body.ExternalUID, SourceResourceID: body.SourceResourceID, Labels: body.Labels, Config: body.Config, Status: body.Status}
	if body.Credential != nil {
		input.Credential = &resource.CredentialInput{Purpose: valueOrEmpty(body.Credential.Purpose), Secret: body.Credential.Secret}
	}
	item, err := h.resources.Create(request.Context(), input)
	if err != nil {
		writeResourceError(writer, request, err)
		return
	}
	h.record(request, "resource.create", "resource", item.ID, item.ScopeID)
	writer.Header().Set("Location", request.URL.Path+"/"+item.ID)
	writeJSON(writer, http.StatusCreated, item)
}

func (h resourceHandler) getResource(writer http.ResponseWriter, request *http.Request) {
	item, err := h.resources.Get(request.Context(), chi.URLParam(request, "resourceID"))
	if err != nil {
		writeResourceError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, item)
}

func (h resourceHandler) updateResource(writer http.ResponseWriter, request *http.Request) {
	var body updateResourceRequest
	if !decodeRequest(writer, request, &body) {
		return
	}
	input := resource.UpdateInput{ScopeID: body.ScopeID, Subtype: body.Subtype, AgentRef: body.AgentRef, Name: body.Name, ExternalUID: body.ExternalUID, SourceResourceID: body.SourceResourceID, Labels: body.Labels, Config: body.Config, Status: body.Status}
	if body.Credential.Set {
		if body.Credential.Value == nil {
			input.ClearCredential = true
		} else {
			input.Credential = &resource.CredentialPatch{Purpose: body.Credential.Value.Purpose}
			if body.Credential.Value.Secret != "" {
				secret := body.Credential.Value.Secret
				input.Credential.Secret = &secret
			}
		}
	}
	item, err := h.resources.Update(request.Context(), chi.URLParam(request, "resourceID"), input)
	if err != nil {
		writeResourceError(writer, request, err)
		return
	}
	h.record(request, "resource.update", "resource", item.ID, item.ScopeID)
	writeJSON(writer, http.StatusOK, item)
}

func valueOrEmpty(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func (h resourceHandler) deleteResource(writer http.ResponseWriter, request *http.Request) {
	id := chi.URLParam(request, "resourceID")
	if err := h.resources.Delete(request.Context(), id); err != nil {
		writeResourceError(writer, request, err)
		return
	}
	h.record(request, "resource.delete", "resource", id, "")
	writer.WriteHeader(http.StatusNoContent)
}

func (h resourceHandler) listSchemas(writer http.ResponseWriter, request *http.Request) {
	items, err := h.resources.ListSchemas(request.Context())
	if err != nil {
		writeResourceError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, items)
}

func (h resourceHandler) setDefault(writer http.ResponseWriter, request *http.Request) {
	var body setDefaultRequest
	if !decodeRequest(writer, request, &body) {
		return
	}
	item, err := h.resources.SetDefault(request.Context(), body.ScopeID, body.DefaultKey, body.ResourceID)
	if err != nil {
		writeResourceError(writer, request, err)
		return
	}
	h.record(request, "resource.default.set", "scope_default", body.DefaultKey, body.ScopeID)
	writeJSON(writer, http.StatusOK, item)
}

func (h resourceHandler) resolveDefault(writer http.ResponseWriter, request *http.Request) {
	item, err := h.resources.ResolveDefault(request.Context(), request.URL.Query().Get("scope_id"), request.URL.Query().Get("default_key"))
	if err != nil {
		writeResourceError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, item)
}

func (h resourceHandler) listRelations(writer http.ResponseWriter, request *http.Request) {
	items, err := h.resources.ListRelations(request.Context(), chi.URLParam(request, "resourceID"))
	if err != nil {
		writeResourceError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, items)
}

func (h resourceHandler) createRelation(writer http.ResponseWriter, request *http.Request) {
	var body createRelationRequest
	if !decodeRequest(writer, request, &body) {
		return
	}
	confirmed := true
	if body.Confirmed != nil {
		confirmed = *body.Confirmed
	}
	item, err := h.resources.CreateRelation(request.Context(), currentUser(request).ID, resource.CreateRelationInput{SourceResourceID: chi.URLParam(request, "resourceID"), TargetResourceID: body.TargetResourceID, RelationType: body.RelationType, Attributes: body.Attributes, DiscoverySource: body.DiscoverySource, Confidence: body.Confidence, Confirmed: confirmed})
	if err != nil {
		writeResourceError(writer, request, err)
		return
	}
	h.record(request, "resource.relation.create", "resource_relation", item.ID, "")
	writeJSON(writer, http.StatusCreated, item)
}

func (h resourceHandler) deleteRelation(writer http.ResponseWriter, request *http.Request) {
	if err := h.resources.DeleteRelation(request.Context(), chi.URLParam(request, "resourceID"), chi.URLParam(request, "relationID")); err != nil {
		writeResourceError(writer, request, err)
		return
	}
	h.record(request, "resource.relation.delete", "resource_relation", chi.URLParam(request, "relationID"), "")
	writer.WriteHeader(http.StatusNoContent)
}

func (h resourceHandler) topology(writer http.ResponseWriter, request *http.Request) {
	depth, err := parseQueryInt(request, "depth")
	if err != nil {
		writeError(writer, request, http.StatusBadRequest, "invalid_request", "depth must be an integer")
		return
	}
	maxNodes, err := parseQueryInt(request, "max_nodes")
	if err != nil {
		writeError(writer, request, http.StatusBadRequest, "invalid_request", "max_nodes must be an integer")
		return
	}
	items, err := h.resources.Topology(request.Context(), chi.URLParam(request, "resourceID"), depth, maxNodes)
	if err != nil {
		writeResourceError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, map[string]any{"items": items, "depth": depth, "max_nodes": maxNodes})
}

func (h resourceHandler) record(request *http.Request, action, targetType, targetID, scopeID string) {
	if h.auditor == nil {
		return
	}
	user := currentUser(request)
	_ = h.auditor.Record(request.Context(), audit.Event{ActorUserID: user.ID, Action: action, TargetType: targetType, TargetID: targetID, ScopeID: scopeID, Result: "success", RequestID: middleware.GetReqID(request.Context()), ClientIP: requestClientIP(request)})
}

func parseResourcePagination(writer http.ResponseWriter, request *http.Request) (resource.Pagination, bool) {
	page, err := parseOptionalPositiveInt(request.URL.Query().Get("page"))
	if err != nil {
		writeError(writer, request, http.StatusBadRequest, "invalid_request", "page must be a positive integer")
		return resource.Pagination{}, false
	}
	pageSize, err := parseOptionalPositiveInt(request.URL.Query().Get("page_size"))
	if err != nil {
		writeError(writer, request, http.StatusBadRequest, "invalid_request", "page_size must be a positive integer")
		return resource.Pagination{}, false
	}
	return resource.Pagination{Page: page, PageSize: pageSize}, true
}

func parseQueryInt(request *http.Request, key string) (int, error) {
	value := request.URL.Query().Get(key)
	if value == "" {
		return 0, nil
	}
	return strconv.Atoi(value)
}

func writeResourceError(writer http.ResponseWriter, request *http.Request, err error) {
	var validationError *resource.ValidationError
	switch {
	case errors.As(err, &validationError):
		writeError(writer, request, http.StatusBadRequest, "invalid_request", validationError.Message)
	case errors.Is(err, authorization.ErrForbidden):
		writeError(writer, request, http.StatusForbidden, "forbidden", "You do not have permission for this operation")
	case errors.Is(err, resource.ErrNotFound):
		writeError(writer, request, http.StatusNotFound, "not_found", "Resource not found")
	case errors.Is(err, resource.ErrConflict):
		writeError(writer, request, http.StatusConflict, "conflict", "Resource conflicts with existing data")
	case errors.Is(err, resource.ErrSchemaNotFound):
		writeError(writer, request, http.StatusBadRequest, "invalid_request", "Resource kind or schema is not registered")
	case errors.Is(err, resource.ErrRelationCycle):
		writeError(writer, request, http.StatusConflict, "relation_cycle", "Resource relation would create a cycle")
	default:
		writeError(writer, request, http.StatusInternalServerError, "internal_error", "Internal server error")
	}
}
