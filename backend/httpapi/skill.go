package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"opskeeper/backend/audit"
	"opskeeper/backend/authorization"
	"opskeeper/backend/connector"
	"opskeeper/backend/llm"
	"opskeeper/backend/persona"
	"opskeeper/backend/resource"
	"opskeeper/backend/skill"
)

type llmService interface {
	TestConnection(context.Context, string, string, string, bool) (llm.ConnectionResult, error)
	TestDraftConnection(context.Context, llm.DraftConnection, bool) (llm.ConnectionResult, error)
}

type providerCatalogService interface {
	ListProviders(context.Context, string) ([]llm.Provider, error)
}

type providerCatalogWriterService interface {
	providerCatalogService
	CreateProvider(context.Context, llm.ProviderInput) (llm.Provider, error)
	UpdateProvider(context.Context, string, llm.ProviderPatch) (llm.Provider, error)
	DeleteProvider(context.Context, string) error
}

type connectionCheckRecorder interface {
	RecordCheck(context.Context, connector.Check) (connector.Check, error)
}

type providerConnectionTestRecorder interface {
	RecordConnectionTest(context.Context, string, string, string, int64, time.Time) error
}

type skillService interface {
	ListSkills(context.Context, string) ([]skill.Skill, error)
	CreateVersion(context.Context, string, skill.CreateVersionInput) (skill.Version, error)
	ListVersions(context.Context, string) ([]skill.Version, error)
	Publish(context.Context, string, string) (skill.Version, error)
	Disable(context.Context, string, string) (skill.Version, error)
	SetDefault(context.Context, string, string, string, string) (skill.Default, error)
	Resolve(context.Context, string, string, string) (skill.Version, error)
}

type personaService interface {
	ListPersonas(context.Context, string) ([]persona.Persona, error)
	CreateVersion(context.Context, string, string, map[string]any) (persona.PersonaVersion, error)
	ListVersions(context.Context, string) ([]persona.PersonaVersion, error)
	PublishVersion(context.Context, string, string) (persona.PersonaVersion, error)
	DisableVersion(context.Context, string, string) (persona.PersonaVersion, error)
}

type aiHandler struct {
	llms          llmService
	skills        skillService
	authorization authorizationService
	auditor       audit.Logger
	personas      personaService
	checks        connectionCheckRecorder
}

type testProviderRequest struct {
	ScopeID   string `json:"scope_id"`
	ModelName string `json:"model_name"`
	Stream    bool   `json:"stream"`
}
type testDraftProviderRequest struct {
	ScopeID        string   `json:"scope_id"`
	ProviderType   string   `json:"provider_type"`
	BaseURL        string   `json:"base_url"`
	ModelName      string   `json:"model_name"`
	APIKey         string   `json:"api_key"`
	TimeoutSeconds int      `json:"timeout_seconds"`
	ContextWindow  int      `json:"context_window"`
	Temperature    float64  `json:"temperature"`
	Capabilities   []string `json:"capabilities"`
	Stream         bool     `json:"stream"`
}
type createProviderRequest struct {
	ScopeID string             `json:"scope_id"`
	Name    string             `json:"name"`
	Status  string             `json:"status"`
	Config  llm.ProviderConfig `json:"config"`
	APIKey  *string            `json:"api_key"`
}
type updateProviderRequest struct {
	Name   *string             `json:"name"`
	Status *string             `json:"status"`
	Config *llm.ProviderConfig `json:"config"`
	APIKey *string             `json:"api_key"`
}
type createSkillVersionRequest struct {
	Manifest     skill.Manifest   `json:"manifest"`
	InputSchema  json.RawMessage  `json:"input_schema"`
	OutputSchema json.RawMessage  `json:"output_schema"`
	Tools        []skill.ToolSpec `json:"tools"`
	RiskLevel    string           `json:"risk_level"`
}
type setSkillDefaultRequest struct {
	ScopeID        string `json:"scope_id"`
	SkillID        string `json:"skill_id"`
	SkillVersionID string `json:"skill_version_id"`
}
type createPersonaVersionRequest struct {
	Config map[string]any `json:"config"`
}

func registerAIRoutes(router chi.Router, llms llmService, skills skillService, personas personaService, authorizer authorizationService, auditor audit.Logger, checks connectionCheckRecorder, requirePermission func(authorization.Permission) func(http.Handler) http.Handler) {
	h := aiHandler{llms: llms, skills: skills, personas: personas, authorization: authorizer, auditor: auditor, checks: checks}
	if bindings, ok := llms.(providerBindingService); ok {
		registerProviderBindingRoutes(router, bindings, requirePermission)
	}
	if availability, ok := llms.(providerAvailabilityService); ok {
		registerProviderAvailabilityRoute(router, availability, requirePermission)
	}
	guard := func(permission authorization.Permission) func(http.Handler) http.Handler {
		if requirePermission == nil {
			return func(next http.Handler) http.Handler { return next }
		}
		return requirePermission(permission)
	}
	if catalog, ok := llms.(providerCatalogService); ok {
		router.With(guard(authorization.ProviderRead)).Get("/providers", func(w http.ResponseWriter, r *http.Request) {
			items, err := catalog.ListProviders(r.Context(), r.URL.Query().Get("scope_id"))
			if err != nil {
				writeAIError(w, r, err)
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"items": items})
		})
		if writer, ok := llms.(providerCatalogWriterService); ok {
			router.With(guard(authorization.ProviderManage)).Post("/providers", func(w http.ResponseWriter, r *http.Request) {
				var body createProviderRequest
				if !decodeRequest(w, r, &body) {
					return
				}
				item, err := writer.CreateProvider(r.Context(), llm.ProviderInput{ScopeID: body.ScopeID, Name: body.Name, Status: body.Status, Config: body.Config, APIKey: body.APIKey})
				if err != nil {
					writeAIError(w, r, err)
					return
				}
				writeJSON(w, http.StatusCreated, item)
			})
			router.With(guard(authorization.ProviderManage)).Patch("/providers/{providerID}", func(w http.ResponseWriter, r *http.Request) {
				var body updateProviderRequest
				if !decodeRequest(w, r, &body) {
					return
				}
				item, err := writer.UpdateProvider(r.Context(), chi.URLParam(r, "providerID"), llm.ProviderPatch{Name: body.Name, Status: body.Status, Config: body.Config, APIKey: body.APIKey})
				if err != nil {
					writeAIError(w, r, err)
					return
				}
				writeJSON(w, http.StatusOK, item)
			})
			router.With(guard(authorization.ProviderManage)).Delete("/providers/{providerID}", func(w http.ResponseWriter, r *http.Request) {
				if err := writer.DeleteProvider(r.Context(), chi.URLParam(r, "providerID")); err != nil {
					writeAIError(w, r, err)
					return
				}
				w.WriteHeader(http.StatusNoContent)
			})
		}
	}
	if skills != nil {
		router.With(guard(authorization.SkillRead)).Get("/skills", func(w http.ResponseWriter, r *http.Request) {
			items, err := skills.ListSkills(r.Context(), r.URL.Query().Get("scope_id"))
			if err != nil {
				writeAIError(w, r, err)
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"items": items})
		})
	}
	if personas != nil {
		router.With(guard(authorization.PersonaRead)).Get("/personas", func(w http.ResponseWriter, r *http.Request) {
			items, err := personas.ListPersonas(r.Context(), r.URL.Query().Get("scope_id"))
			if err != nil {
				writeAIError(w, r, err)
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"items": items})
		})
	}
	if llms != nil {
		router.With(guard(authorization.ProviderUse)).Post("/providers/{providerID}/test", h.testProvider)
		router.With(guard(authorization.ProviderManage)).Post("/providers/test-draft", h.testDraftProvider)
	}
	if skills != nil {
		router.With(guard(authorization.SkillManage)).Post("/skills/{skillID}/versions", h.createVersion)
		router.With(guard(authorization.SkillRead)).Get("/skills/{skillID}/versions", h.listVersions)
		router.With(guard(authorization.SkillManage)).Post("/skills/{skillID}/versions/{versionID}/publish", h.publishVersion)
		router.With(guard(authorization.SkillManage)).Post("/skills/{skillID}/versions/{versionID}/disable", h.disableVersion)
		router.With(guard(authorization.SkillManage)).Put("/skill-defaults", h.setSkillDefault)
		router.With(guard(authorization.SkillRead)).Get("/skill-defaults", h.resolveSkillDefault)
	}
	if personas != nil {
		router.With(guard(authorization.PersonaManage)).Post("/personas/{personaID}/versions", h.createPersonaVersion)
		router.With(guard(authorization.PersonaRead)).Get("/personas/{personaID}/versions", h.listPersonaVersions)
		router.With(guard(authorization.PersonaManage)).Post("/personas/{personaID}/versions/{versionID}/publish", h.publishPersonaVersion)
		router.With(guard(authorization.PersonaManage)).Post("/personas/{personaID}/versions/{versionID}/disable", h.disablePersonaVersion)
	}
}

func (h aiHandler) createPersonaVersion(w http.ResponseWriter, r *http.Request) {
	var body createPersonaVersionRequest
	if !decodeRequest(w, r, &body) {
		return
	}
	item, err := h.personas.CreateVersion(r.Context(), currentUser(r).ID, chi.URLParam(r, "personaID"), body.Config)
	if err != nil {
		writeAIError(w, r, err)
		return
	}
	h.record(r, "persona.version.create", "persona_version", item.ID, chi.URLParam(r, "personaID"))
	writeJSON(w, http.StatusCreated, item)
}

func (h aiHandler) listPersonaVersions(w http.ResponseWriter, r *http.Request) {
	items, err := h.personas.ListVersions(r.Context(), chi.URLParam(r, "personaID"))
	if err != nil {
		writeAIError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (h aiHandler) publishPersonaVersion(w http.ResponseWriter, r *http.Request) {
	item, err := h.personas.PublishVersion(r.Context(), chi.URLParam(r, "personaID"), chi.URLParam(r, "versionID"))
	if err != nil {
		writeAIError(w, r, err)
		return
	}
	h.record(r, "persona.version.publish", "persona_version", item.ID, chi.URLParam(r, "personaID"))
	writeJSON(w, http.StatusOK, item)
}

func (h aiHandler) disablePersonaVersion(w http.ResponseWriter, r *http.Request) {
	item, err := h.personas.DisableVersion(r.Context(), chi.URLParam(r, "personaID"), chi.URLParam(r, "versionID"))
	if err != nil {
		writeAIError(w, r, err)
		return
	}
	h.record(r, "persona.version.disable", "persona_version", item.ID, chi.URLParam(r, "personaID"))
	writeJSON(w, http.StatusOK, item)
}

func (h aiHandler) testProvider(w http.ResponseWriter, r *http.Request) {
	var body testProviderRequest
	if !decodeRequest(w, r, &body) {
		return
	}
	started := time.Now()
	item, err := h.llms.TestConnection(r.Context(), body.ScopeID, chi.URLParam(r, "providerID"), body.ModelName, body.Stream)
	if err != nil {
		if recorder, ok := h.llms.(providerConnectionTestRecorder); ok {
			_ = recorder.RecordConnectionTest(r.Context(), chi.URLParam(r, "providerID"), "failed", safeAIConnectionError(err), time.Since(started).Milliseconds(), time.Now())
		}
		h.recordConnectionCheck(r, chi.URLParam(r, "providerID"), "failed", safeAIConnectionError(err), time.Since(started).Milliseconds(), nil)
		writeAIError(w, r, err)
		return
	}
	h.recordConnectionCheck(r, item.ProviderID, item.Status, item.Message, item.LatencyMS, nil)
	if recorder, ok := h.llms.(providerConnectionTestRecorder); ok {
		_ = recorder.RecordConnectionTest(r.Context(), item.ProviderID, item.Status, item.Message, item.LatencyMS, time.Now())
	}
	h.record(r, "provider.connection.test", "provider", item.ProviderID, body.ScopeID)
	writeJSON(w, http.StatusOK, item)
}

func (h aiHandler) recordConnectionCheck(r *http.Request, resourceID, status, message string, latencyMS int64, capabilities []connector.Capability) {
	if h.checks == nil || strings.TrimSpace(resourceID) == "" {
		return
	}
	actorID := currentUser(r).ID
	_, _ = h.checks.RecordCheck(r.Context(), connector.Check{
		ResourceID:   resourceID,
		Status:       status,
		Message:      message,
		LatencyMS:    latencyMS,
		Capabilities: capabilities,
		CheckedBy:    &actorID,
		CheckedAt:    time.Now(),
	})
}

func (h aiHandler) testDraftProvider(w http.ResponseWriter, r *http.Request) {
	var body testDraftProviderRequest
	if !decodeRequest(w, r, &body) {
		return
	}
	item, err := h.llms.TestDraftConnection(r.Context(), llm.DraftConnection{
		ScopeID: body.ScopeID, ProviderType: body.ProviderType, BaseURL: body.BaseURL,
		ModelName: body.ModelName, APIKey: body.APIKey, TimeoutSeconds: body.TimeoutSeconds, ContextWindow: body.ContextWindow,
		Temperature:  body.Temperature,
		Capabilities: body.Capabilities,
	}, body.Stream)
	if err != nil {
		writeAIConnectionError(w, r, err)
		return
	}
	h.record(r, "llm.draft.connection.test", "scope", body.ScopeID, body.ScopeID)
	writeJSON(w, http.StatusOK, item)
}

// writeAIConnectionError keeps the draft connection check useful to an
// operator while keeping the broader AI execution endpoints intentionally
// generic. Upstream providers commonly return the actionable reason in the
// error value (for example, an invalid model or a 401 response).
func writeAIConnectionError(w http.ResponseWriter, r *http.Request, err error) {
	var llmValidation *llm.ValidationError
	switch {
	case errors.As(err, &llmValidation):
		writeError(w, r, http.StatusBadRequest, "invalid_request", llmValidation.Message)
	case errors.Is(err, authorization.ErrForbidden):
		writeError(w, r, http.StatusForbidden, "forbidden", "You do not have permission for this operation")
	case errors.Is(err, context.DeadlineExceeded):
		writeError(w, r, http.StatusGatewayTimeout, "timeout", "模型连接测试超时，请检查地址和网络")
	default:
		message := safeAIConnectionError(err)
		if message == "" {
			message = "模型连接测试失败，请检查地址、凭证和模型名称"
		}
		writeError(w, r, http.StatusBadGateway, "ai_connection_error", message)
	}
}

func safeAIConnectionError(err error) string {
	if err == nil {
		return ""
	}
	message := strings.TrimSpace(err.Error())
	if message == "" {
		return ""
	}
	// Do not echo credentials if an SDK or provider accidentally includes them
	// in an error string. The adapter already limits upstream response bodies.
	message = aiSensitiveValuePattern.ReplaceAllString(message, "$1=<redacted>")
	if len([]rune(message)) > 600 {
		message = string([]rune(message)[:600]) + "..."
	}
	return message
}

var aiSensitiveValuePattern = regexp.MustCompile(`(?i)(authorization|api[- ]?key|token|secret|password|bearer)(\s*(?:[:=]|\s)\s*)[^\s,;\)\]}]+`)

func (h aiHandler) createVersion(w http.ResponseWriter, r *http.Request) {
	var body createSkillVersionRequest
	if !decodeRequest(w, r, &body) {
		return
	}
	item, err := h.skills.CreateVersion(r.Context(), currentUser(r).ID, skill.CreateVersionInput{SkillID: chi.URLParam(r, "skillID"), Manifest: body.Manifest, InputSchema: body.InputSchema, OutputSchema: body.OutputSchema, Tools: body.Tools, RiskLevel: body.RiskLevel})
	if err != nil {
		writeAIError(w, r, err)
		return
	}
	h.record(r, "skill.version.create", "skill_version", item.ID, "")
	writeJSON(w, http.StatusCreated, item)
}
func (h aiHandler) listVersions(w http.ResponseWriter, r *http.Request) {
	items, err := h.skills.ListVersions(r.Context(), chi.URLParam(r, "skillID"))
	if err != nil {
		writeAIError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}
func (h aiHandler) publishVersion(w http.ResponseWriter, r *http.Request) {
	item, err := h.skills.Publish(r.Context(), chi.URLParam(r, "skillID"), chi.URLParam(r, "versionID"))
	if err != nil {
		writeAIError(w, r, err)
		return
	}
	h.record(r, "skill.version.publish", "skill_version", item.ID, "")
	writeJSON(w, http.StatusOK, item)
}
func (h aiHandler) disableVersion(w http.ResponseWriter, r *http.Request) {
	item, err := h.skills.Disable(r.Context(), chi.URLParam(r, "skillID"), chi.URLParam(r, "versionID"))
	if err != nil {
		writeAIError(w, r, err)
		return
	}
	h.record(r, "skill.version.disable", "skill_version", item.ID, "")
	writeJSON(w, http.StatusOK, item)
}
func (h aiHandler) setSkillDefault(w http.ResponseWriter, r *http.Request) {
	var body setSkillDefaultRequest
	if !decodeRequest(w, r, &body) {
		return
	}
	item, err := h.skills.SetDefault(r.Context(), currentUser(r).ID, body.ScopeID, body.SkillID, body.SkillVersionID)
	if err != nil {
		writeAIError(w, r, err)
		return
	}
	h.record(r, "skill.default.set", "scope", body.ScopeID, body.ScopeID)
	writeJSON(w, http.StatusOK, item)
}
func (h aiHandler) resolveSkillDefault(w http.ResponseWriter, r *http.Request) {
	item, err := h.skills.Resolve(r.Context(), r.URL.Query().Get("scope_id"), r.URL.Query().Get("skill_id"), r.URL.Query().Get("skill_version_id"))
	if err != nil {
		writeAIError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}
func (h aiHandler) record(r *http.Request, action, targetType, targetID, scopeID string) {
	if h.auditor == nil {
		return
	}
	_ = h.auditor.Record(r.Context(), audit.Event{ActorUserID: currentUser(r).ID, Action: action, TargetType: targetType, TargetID: targetID, ScopeID: scopeID, Result: "success", RequestID: middleware.GetReqID(r.Context()), ClientIP: requestClientIP(r)})
}

func writeAIError(w http.ResponseWriter, r *http.Request, err error) {
	var llmValidation *llm.ValidationError
	var skillValidation *skill.ValidationError
	var resourceValidation *resource.ValidationError
	switch {
	case errors.As(err, &llmValidation):
		writeError(w, r, http.StatusBadRequest, "invalid_request", llmValidation.Message)
	case errors.As(err, &skillValidation):
		writeError(w, r, http.StatusBadRequest, "invalid_request", skillValidation.Message)
	case errors.As(err, &resourceValidation):
		writeError(w, r, http.StatusBadRequest, "invalid_request", resourceValidation.Message)
	case errors.Is(err, authorization.ErrForbidden):
		writeError(w, r, http.StatusForbidden, "forbidden", "You do not have permission for this operation")
	case errors.Is(err, llm.ErrNotFound), errors.Is(err, skill.ErrNotFound), errors.Is(err, resource.ErrNotFound):
		writeError(w, r, http.StatusNotFound, "not_found", "Requested AI configuration was not found")
	case errors.Is(err, llm.ErrConflict), errors.Is(err, skill.ErrConflict):
		writeError(w, r, http.StatusConflict, "conflict", "AI configuration conflicts with existing data")
	case errors.Is(err, context.DeadlineExceeded):
		writeError(w, r, http.StatusGatewayTimeout, "timeout", "AI execution timed out")
	default:
		writeError(w, r, http.StatusBadGateway, "ai_runtime_error", "AI runtime request failed")
	}
}
