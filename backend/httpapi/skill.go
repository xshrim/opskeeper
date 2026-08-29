package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"opskeeper/backend/aiengine"
	"opskeeper/backend/audit"
	"opskeeper/backend/authorization"
	"opskeeper/backend/llm"
	"opskeeper/backend/resource"
	"opskeeper/backend/skill"
)

type llmService interface {
	TestConnection(context.Context, string, string, string, bool) (llm.ConnectionResult, error)
	TestDraftConnection(context.Context, llm.DraftConnection, bool) (llm.ConnectionResult, error)
}

type skillService interface {
	CreateVersion(context.Context, string, skill.CreateVersionInput) (skill.Version, error)
	ListVersions(context.Context, string) ([]skill.Version, error)
	Publish(context.Context, string, string) (skill.Version, error)
	Disable(context.Context, string, string) (skill.Version, error)
	SetDefault(context.Context, string, string, string, string) (skill.Default, error)
	Resolve(context.Context, string, string, string) (skill.Version, error)
	GetExecution(context.Context, string) (skill.Execution, error)
	ListExecutions(context.Context, string, int) ([]skill.Execution, error)
}

type agentProfileService interface {
	CreateVersion(context.Context, string, string, map[string]any) (skill.AgentProfileVersion, error)
	ListVersions(context.Context, string) ([]skill.AgentProfileVersion, error)
	PublishVersion(context.Context, string, string) (skill.AgentProfileVersion, error)
	DisableVersion(context.Context, string, string) (skill.AgentProfileVersion, error)
}

type skillRunner interface {
	Run(context.Context, skill.RunInput) (skill.RunResult, error)
}

type aiHandler struct {
	llms          llmService
	skills        skillService
	runner        skillRunner
	engine        aiengine.Engine
	authorization authorizationService
	auditor       audit.Logger
	agentProfiles agentProfileService
}

type testAIProviderRequest struct {
	ScopeID   string `json:"scope_id"`
	ModelName string `json:"model_name"`
	Stream    bool   `json:"stream"`
}
type testDraftAIProviderRequest struct {
	ScopeID       string   `json:"scope_id"`
	ProviderType  string   `json:"provider_type"`
	BaseURL       string   `json:"base_url"`
	ModelName     string   `json:"model_name"`
	APIKey        string   `json:"api_key"`
	ContextWindow int      `json:"context_window"`
	Temperature   float64  `json:"temperature"`
	Capabilities  []string `json:"capabilities"`
	Stream        bool     `json:"stream"`
}
type createSkillVersionRequest struct {
	Manifest     skill.Manifest   `json:"manifest"`
	InputSchema  json.RawMessage  `json:"input_schema"`
	OutputSchema json.RawMessage  `json:"output_schema"`
	Tools        []skill.ToolSpec `json:"tools"`
	RiskLevel    string           `json:"risk_level"`
}
type setSkillDefaultRequest struct {
	ScopeID         string `json:"scope_id"`
	SkillResourceID string `json:"skill_resource_id"`
	SkillVersionID  string `json:"skill_version_id"`
}
type createAgentProfileVersionRequest struct {
	Config map[string]any `json:"config"`
}
type executeSkillRequest struct {
	ScopeID              string         `json:"scope_id"`
	TargetResourceID     string         `json:"target_resource_id"`
	AIProviderResourceID string         `json:"ai_provider_resource_id,omitempty"`
	ModelName            string         `json:"model_name,omitempty"`
	SkillResourceID      string         `json:"skill_resource_id,omitempty"`
	SkillVersionID       string         `json:"skill_version_id,omitempty"`
	AgentProfileID       string         `json:"agent_profile_id,omitempty"`
	Input                map[string]any `json:"input"`
	MaxToolCalls         int            `json:"max_tool_calls,omitempty"`
	MaxTokens            int64          `json:"max_tokens,omitempty"`
	MaxOutputBytes       int            `json:"max_output_bytes,omitempty"`
	TimeoutSeconds       int            `json:"timeout_seconds,omitempty"`
	Stream               bool           `json:"stream,omitempty"`
}

func registerAIRoutes(router chi.Router, llms llmService, skills skillService, agentProfiles agentProfileService, runnerService skillRunner, engine aiengine.Engine, authorizer authorizationService, auditor audit.Logger, requirePermission func(authorization.Permission) func(http.Handler) http.Handler) {
	h := aiHandler{llms: llms, skills: skills, agentProfiles: agentProfiles, runner: runnerService, engine: engine, authorization: authorizer, auditor: auditor}
	if bindings, ok := llms.(aiProviderBindingService); ok {
		registerAIProviderBindingRoutes(router, bindings, requirePermission)
	}
	if availability, ok := llms.(aiProviderAvailabilityService); ok {
		registerAIProviderAvailabilityRoute(router, availability, requirePermission)
	}
	guard := func(permission authorization.Permission) func(http.Handler) http.Handler {
		if requirePermission == nil {
			return func(next http.Handler) http.Handler { return next }
		}
		return requirePermission(permission)
	}
	if llms != nil {
		router.With(guard(authorization.ResourceUse)).Post("/ai-providers/{providerID}/test", h.testAIProvider)
		router.With(guard(authorization.ResourceUpdate)).Post("/ai-providers/test-draft", h.testDraftAIProvider)
	}
	if skills != nil {
		router.With(guard(authorization.ResourceUpdate)).Post("/skills/{skillID}/versions", h.createVersion)
		router.With(guard(authorization.ResourceRead)).Get("/skills/{skillID}/versions", h.listVersions)
		router.With(guard(authorization.ResourceUpdate)).Post("/skills/{skillID}/versions/{versionID}/publish", h.publishVersion)
		router.With(guard(authorization.ResourceUpdate)).Post("/skills/{skillID}/versions/{versionID}/disable", h.disableVersion)
		router.With(guard(authorization.ResourceUpdate)).Put("/skill-defaults", h.setSkillDefault)
		router.With(guard(authorization.ResourceRead)).Get("/skill-defaults", h.resolveSkillDefault)
		router.With(guard(authorization.ResourceRead)).Get("/skill-executions", h.listExecutions)
		router.With(guard(authorization.ResourceRead)).Get("/skill-executions/{executionID}", h.getExecution)
	}
	if agentProfiles != nil {
		router.With(guard(authorization.ResourceUpdate)).Post("/agent-profiles/{profileID}/versions", h.createAgentProfileVersion)
		router.With(guard(authorization.ResourceRead)).Get("/agent-profiles/{profileID}/versions", h.listAgentProfileVersions)
		router.With(guard(authorization.ResourceUpdate)).Post("/agent-profiles/{profileID}/versions/{versionID}/publish", h.publishAgentProfileVersion)
		router.With(guard(authorization.ResourceUpdate)).Post("/agent-profiles/{profileID}/versions/{versionID}/disable", h.disableAgentProfileVersion)
	}
	if runnerService != nil || engine != nil {
		router.Post("/skill-executions", h.executeSkill)
	}
}

func (h aiHandler) createAgentProfileVersion(w http.ResponseWriter, r *http.Request) {
	var body createAgentProfileVersionRequest
	if !decodeRequest(w, r, &body) {
		return
	}
	item, err := h.agentProfiles.CreateVersion(r.Context(), currentUser(r).ID, chi.URLParam(r, "profileID"), body.Config)
	if err != nil {
		writeAIError(w, r, err)
		return
	}
	h.record(r, "agent_profile.version.create", "agent_profile_version", item.ID, chi.URLParam(r, "profileID"))
	writeJSON(w, http.StatusCreated, item)
}

func (h aiHandler) listAgentProfileVersions(w http.ResponseWriter, r *http.Request) {
	items, err := h.agentProfiles.ListVersions(r.Context(), chi.URLParam(r, "profileID"))
	if err != nil {
		writeAIError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (h aiHandler) publishAgentProfileVersion(w http.ResponseWriter, r *http.Request) {
	item, err := h.agentProfiles.PublishVersion(r.Context(), chi.URLParam(r, "profileID"), chi.URLParam(r, "versionID"))
	if err != nil {
		writeAIError(w, r, err)
		return
	}
	h.record(r, "agent_profile.version.publish", "agent_profile_version", item.ID, chi.URLParam(r, "profileID"))
	writeJSON(w, http.StatusOK, item)
}

func (h aiHandler) disableAgentProfileVersion(w http.ResponseWriter, r *http.Request) {
	item, err := h.agentProfiles.DisableVersion(r.Context(), chi.URLParam(r, "profileID"), chi.URLParam(r, "versionID"))
	if err != nil {
		writeAIError(w, r, err)
		return
	}
	h.record(r, "agent_profile.version.disable", "agent_profile_version", item.ID, chi.URLParam(r, "profileID"))
	writeJSON(w, http.StatusOK, item)
}

func (h aiHandler) testAIProvider(w http.ResponseWriter, r *http.Request) {
	var body testAIProviderRequest
	if !decodeRequest(w, r, &body) {
		return
	}
	item, err := h.llms.TestConnection(r.Context(), body.ScopeID, chi.URLParam(r, "providerID"), body.ModelName, body.Stream)
	if err != nil {
		writeAIError(w, r, err)
		return
	}
	h.record(r, "ai_provider.connection.test", "resource", item.ProviderResourceID, body.ScopeID)
	writeJSON(w, http.StatusOK, item)
}

func (h aiHandler) testDraftAIProvider(w http.ResponseWriter, r *http.Request) {
	var body testDraftAIProviderRequest
	if !decodeRequest(w, r, &body) {
		return
	}
	item, err := h.llms.TestDraftConnection(r.Context(), llm.DraftConnection{
		ScopeID: body.ScopeID, ProviderType: body.ProviderType, BaseURL: body.BaseURL,
		ModelName: body.ModelName, APIKey: body.APIKey, ContextWindow: body.ContextWindow,
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
	item, err := h.skills.CreateVersion(r.Context(), currentUser(r).ID, skill.CreateVersionInput{SkillResourceID: chi.URLParam(r, "skillID"), Manifest: body.Manifest, InputSchema: body.InputSchema, OutputSchema: body.OutputSchema, Tools: body.Tools, RiskLevel: body.RiskLevel})
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
	item, err := h.skills.SetDefault(r.Context(), currentUser(r).ID, body.ScopeID, body.SkillResourceID, body.SkillVersionID)
	if err != nil {
		writeAIError(w, r, err)
		return
	}
	h.record(r, "skill.default.set", "scope", body.ScopeID, body.ScopeID)
	writeJSON(w, http.StatusOK, item)
}
func (h aiHandler) resolveSkillDefault(w http.ResponseWriter, r *http.Request) {
	item, err := h.skills.Resolve(r.Context(), r.URL.Query().Get("scope_id"), r.URL.Query().Get("skill_resource_id"), r.URL.Query().Get("skill_version_id"))
	if err != nil {
		writeAIError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}
func (h aiHandler) executeSkill(w http.ResponseWriter, r *http.Request) {
	if h.authorization == nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "Authorization service is unavailable")
		return
	}
	var body executeSkillRequest
	if !decodeRequest(w, r, &body) {
		return
	}
	user := currentUser(r)
	subject := authorization.Subject{UserID: user.ID}
	resourceFilter, err := h.authorization.ResourceFilter(r.Context(), subject, authorization.ResourceUse)
	if err != nil {
		writeAIError(w, r, err)
		return
	}
	ctx := authorization.WithResourceFilter(r.Context(), resourceFilter)
	if h.engine != nil {
		contextRequest := aiengine.ContextRequest{}
		if strings.TrimSpace(body.TargetResourceID) != "" {
			contextRequest.ResourceIDs = []string{body.TargetResourceID}
		}
		result, err := h.engine.Execute(ctx, aiengine.Request{
			ActorID: user.ID, ScopeID: body.ScopeID, AIProviderResourceID: body.AIProviderResourceID,
			ModelName: body.ModelName, Purpose: aiengine.PurposeDefault,
			Profile: aiengine.ProfileSkill, Input: body.Input, Context: contextRequest,
			SkillResourceID: body.SkillResourceID, SkillVersionID: body.SkillVersionID,
			AgentProfileID: body.AgentProfileID,
			Budget:         aiengine.Budget{MaxToolCalls: body.MaxToolCalls, MaxTokens: body.MaxTokens, MaxOutputBytes: body.MaxOutputBytes, Timeout: time.Duration(body.TimeoutSeconds) * time.Second},
		})
		if err != nil {
			writeAIError(w, r, err)
			return
		}
		h.record(r, "ai_engine.execute", "ai_execution", result.ExecutionID, body.ScopeID)
		writeJSON(w, http.StatusCreated, result)
		return
	}
	if h.runner == nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "AI execution service is unavailable")
		return
	}
	result, err := h.runner.Run(ctx, skill.RunInput{ActorID: user.ID, ScopeID: body.ScopeID, TargetResourceID: body.TargetResourceID, SkillResourceID: body.SkillResourceID, SkillVersionID: body.SkillVersionID, AgentProfileID: body.AgentProfileID, AIProviderResourceID: body.AIProviderResourceID, ModelName: body.ModelName, Purpose: aiengine.PurposeDefault, Input: body.Input, MaxToolCalls: body.MaxToolCalls, MaxTokens: body.MaxTokens, MaxOutputBytes: body.MaxOutputBytes, Timeout: time.Duration(body.TimeoutSeconds) * time.Second, Stream: body.Stream})
	if err != nil {
		writeAIError(w, r, err)
		return
	}
	h.record(r, "skill.execute", "skill_execution", result.Execution.ID, body.ScopeID)
	writeJSON(w, http.StatusCreated, result)
}
func (h aiHandler) listExecutions(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	items, err := h.skills.ListExecutions(r.Context(), r.URL.Query().Get("scope_id"), limit)
	if err != nil {
		writeAIError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}
func (h aiHandler) getExecution(w http.ResponseWriter, r *http.Request) {
	item, err := h.skills.GetExecution(r.Context(), chi.URLParam(r, "executionID"))
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
