package diagnosis

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"opskeeper/backend/application"
	"opskeeper/backend/authorization"
	"opskeeper/backend/engine"
)

type scopeQueryReader interface {
	DiagnosticScopeTeams(context.Context, string) ([]application.DiagnosticScopeTeam, error)
	DiagnosticScopeProjects(context.Context, string, string) ([]application.DiagnosticScopeProject, error)
	DiagnosticScopeApplications(context.Context, string, string, string, int, int) (application.DiagnosticScopeApplicationPage, error)
	DiagnosticApplicationContextByLocator(context.Context, string, string, string) (application.DiagnosticContext, error)
}

const scopeToolSource = "diagnosis-scope"

func (s *Service) diagnosticScopeTools(authorizationContext context.Context, scopeID string) []engine.Tool {
	reader, ok := s.applications.(scopeQueryReader)
	if !ok || strings.TrimSpace(scopeID) == "" {
		return nil
	}
	return []engine.Tool{
		engine.ToolFunc{Def: engine.ToolDefinition{
			Name: "diagnosis_scope_teams", Source: scopeToolSource, ReadOnly: true,
			Description: "列出当前诊断范围内可见的团队及其项目数量。平台范围使用此工具开始了解组织结构；不要把数据库 ID 当作参数。",
			InputSchema: json.RawMessage(`{"type":"object","properties":{},"additionalProperties":false}`),
		}, Fn: func(ctx context.Context, _ map[string]any) (engine.ToolResult, error) {
			items, err := reader.DiagnosticScopeTeams(withAuthorizationContext(ctx, authorizationContext), scopeID)
			if err != nil {
				return engine.ToolResult{}, err
			}
			return engine.ToolResult{Output: map[string]any{"teams": items}, Untrusted: true}, nil
		}},
		engine.ToolFunc{Def: engine.ToolDefinition{
			Name: "diagnosis_scope_projects", Source: scopeToolSource, ReadOnly: true,
			Description: "列出当前诊断范围内可见的项目及应用数量。可用 team_name 缩小到一个团队；不要传内部 ID。",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"team_name":{"type":"string","maxLength":200}},"additionalProperties":false}`),
		}, Fn: func(ctx context.Context, args map[string]any) (engine.ToolResult, error) {
			items, err := reader.DiagnosticScopeProjects(withAuthorizationContext(ctx, authorizationContext), scopeID, scopeToolString(args, "team_name"))
			if err != nil {
				return engine.ToolResult{}, err
			}
			return engine.ToolResult{Output: map[string]any{"projects": items}, Untrusted: true}, nil
		}},
		engine.ToolFunc{Def: engine.ToolDefinition{
			Name: "diagnosis_scope_applications", Source: scopeToolSource, ReadOnly: true,
			Description: "按需分页列出当前范围内的应用摘要。可用 team_name 和 project_code 过滤；limit 最大 100，返回 total 以便继续分页。",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"team_name":{"type":"string","maxLength":200},"project_code":{"type":"string","maxLength":100},"offset":{"type":"integer","minimum":0,"maximum":1000000},"limit":{"type":"integer","minimum":1,"maximum":100}},"additionalProperties":false}`),
		}, Fn: func(ctx context.Context, args map[string]any) (engine.ToolResult, error) {
			page, err := reader.DiagnosticScopeApplications(withAuthorizationContext(ctx, authorizationContext), scopeID, scopeToolString(args, "team_name"), scopeToolString(args, "project_code"), scopeToolInt(args, "offset", 0), scopeToolInt(args, "limit", 100))
			if err != nil {
				return engine.ToolResult{}, err
			}
			return engine.ToolResult{Output: page, Untrusted: true}, nil
		}},
		engine.ToolFunc{Def: engine.ToolDefinition{
			Name: "diagnosis_application_context", Source: scopeToolSource, ReadOnly: true,
			Description: "读取一个应用的诊断配置快照，包括实例、承载运行时、selector、业务资源关联和应用上下游关系。必须使用应用名称或应用编码，不能使用数据库 ID；实时状态仍需调用运行时或资源工具核验。",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"name":{"type":"string","maxLength":200},"code":{"type":"string","maxLength":100}},"anyOf":[{"required":["name"]},{"required":["code"]}],"additionalProperties":false}`),
		}, Fn: func(ctx context.Context, args map[string]any) (engine.ToolResult, error) {
			name, code := scopeToolString(args, "name"), scopeToolString(args, "code")
			if name == "" && code == "" {
				return engine.ToolResult{}, fmt.Errorf("application name or code is required")
			}
			item, err := reader.DiagnosticApplicationContextByLocator(withAuthorizationContext(ctx, authorizationContext), scopeID, name, code)
			if err != nil {
				return engine.ToolResult{}, err
			}
			raw, err := json.Marshal(item)
			if err != nil {
				return engine.ToolResult{}, err
			}
			sanitized, err := sanitizeModelJSON(raw)
			if err != nil {
				return engine.ToolResult{}, err
			}
			return engine.ToolResult{Output: sanitized, Untrusted: true}, nil
		}},
	}
}

// The model SDK owns the tool context and is not required to preserve HTTP
// context values. Reattach only server-resolved authorization filters while
// retaining the SDK context's cancellation and deadline.
func withAuthorizationContext(ctx, source context.Context) context.Context {
	if filter, ok := authorization.ResourceFilterFromContext(source); ok {
		ctx = authorization.WithResourceFilter(ctx, filter)
	}
	if filter, ok := authorization.ResourceUseFilterFromContext(source); ok {
		ctx = authorization.WithResourceUseFilter(ctx, filter)
	}
	if filter, ok := authorization.ResourceReadFilterFromContext(source); ok {
		ctx = authorization.WithResourceReadFilter(ctx, filter)
	}
	if filter, ok := authorization.ScopeFilterFromContext(source); ok {
		ctx = authorization.WithScopeFilter(ctx, filter)
	}
	return ctx
}

func scopeToolString(args map[string]any, key string) string {
	value, _ := args[key].(string)
	return strings.TrimSpace(value)
}

func scopeToolInt(args map[string]any, key string, fallback int) int {
	switch value := args[key].(type) {
	case int:
		return value
	case int64:
		return int(value)
	case float64:
		return int(value)
	default:
		return fallback
	}
}
