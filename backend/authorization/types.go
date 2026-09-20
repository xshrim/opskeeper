package authorization

import "context"

type Permission string

const (
	OrganizationRead  Permission = "organization:read"
	TeamManage        Permission = "team:manage"
	ProjectManage     Permission = "project:manage"
	MemberGrant       Permission = "member:grant"
	ResourceRead      Permission = "resource:read"
	ResourceCreate    Permission = "resource:create"
	ResourceUpdate    Permission = "resource:update"
	ResourceDelete    Permission = "resource:delete"
	ResourceUse       Permission = "resource:use"
	ProviderRead      Permission = "provider:read"
	ProviderManage    Permission = "provider:manage"
	ProviderUse       Permission = "provider:use"
	EngineRead        Permission = "engine:read"
	EngineManage      Permission = "engine:manage"
	EngineUse         Permission = "engine:use"
	SkillRead         Permission = "skill:read"
	SkillManage       Permission = "skill:manage"
	SkillExecute      Permission = "skill:execute"
	PersonaRead       Permission = "persona:read"
	PersonaManage     Permission = "persona:manage"
	PersonaUse        Permission = "persona:use"
	RelationManage    Permission = "relation:manage"
	DiagnosisStart    Permission = "diagnosis:start"
	DiagnosisRead     Permission = "diagnosis:read"
	InspectionManage  Permission = "inspection:manage"
	InspectionExecute Permission = "inspection:execute"
	AuditRead         Permission = "audit:read"
)

type Subject struct {
	UserID string
}

type ScopeFilter struct {
	SubjectID  string
	Permission Permission
	ScopeIDs   []string
}

type ResourceFilter struct {
	SubjectID   string
	Permission  Permission
	ScopeIDs    []string
	ResourceIDs []string
}

func (f ResourceFilter) Allows(scopeID, resourceID string) bool {
	for _, allowed := range f.ScopeIDs {
		if allowed == scopeID {
			return true
		}
	}
	for _, allowed := range f.ResourceIDs {
		if allowed == resourceID {
			return true
		}
	}
	return false
}

func (f ScopeFilter) Allows(scopeID string) bool {
	for _, allowed := range f.ScopeIDs {
		if allowed == scopeID {
			return true
		}
	}
	return false
}

type contextKey struct{}
type resourceContextKey struct{}

func WithScopeFilter(ctx context.Context, filter ScopeFilter) context.Context {
	return context.WithValue(ctx, contextKey{}, filter)
}

func ScopeFilterFromContext(ctx context.Context) (ScopeFilter, bool) {
	filter, ok := ctx.Value(contextKey{}).(ScopeFilter)
	return filter, ok
}

func WithResourceFilter(ctx context.Context, filter ResourceFilter) context.Context {
	ctx = WithScopeFilter(ctx, ScopeFilter{SubjectID: filter.SubjectID, Permission: filter.Permission, ScopeIDs: filter.ScopeIDs})
	return context.WithValue(ctx, resourceContextKey{}, filter)
}

func ResourceFilterFromContext(ctx context.Context) (ResourceFilter, bool) {
	filter, ok := ctx.Value(resourceContextKey{}).(ResourceFilter)
	return filter, ok
}
