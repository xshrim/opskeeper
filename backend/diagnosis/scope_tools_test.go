package diagnosis

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	appdomain "opskeeper/backend/application"
	"opskeeper/backend/authorization"
)

type scopeToolApplications struct{}

func (scopeToolApplications) Get(context.Context, string) (appdomain.Application, error) {
	return appdomain.Application{}, nil
}
func (scopeToolApplications) ContextResourceIDs(context.Context, string, string) ([]string, error) {
	return nil, nil
}
func (scopeToolApplications) DiagnosticContext(context.Context, string, string) (appdomain.DiagnosticContext, error) {
	return appdomain.DiagnosticContext{}, nil
}
func (scopeToolApplications) DiagnosticScopeTeams(context.Context, string) ([]appdomain.DiagnosticScopeTeam, error) {
	return []appdomain.DiagnosticScopeTeam{{Name: "Payments", Status: "active", ProjectCount: 2}}, nil
}
func (scopeToolApplications) DiagnosticScopeProjects(context.Context, string, string) ([]appdomain.DiagnosticScopeProject, error) {
	return []appdomain.DiagnosticScopeProject{{Name: "Checkout", Code: "checkout", Status: "active", ApplicationCount: 1}}, nil
}
func (scopeToolApplications) DiagnosticScopeApplications(context.Context, string, string, string, int, int) (appdomain.DiagnosticScopeApplicationPage, error) {
	return appdomain.DiagnosticScopeApplicationPage{Items: []appdomain.DiagnosticScopeApplication{{Name: "Orders", Code: "orders", RuntimeKind: appdomain.RuntimeDocker}}, Total: 1, Limit: 100}, nil
}
func (scopeToolApplications) DiagnosticApplicationContextByLocator(context.Context, string, string, string) (appdomain.DiagnosticContext, error) {
	return appdomain.DiagnosticContext{Application: appdomain.DiagnosticApplication{ID: "internal-app-id", Name: "Orders"}, Resources: []appdomain.DiagnosticResource{{ID: "internal-resource-id", Name: "Postgres", Kind: "postgresql"}}}, nil
}

type missingScopeToolApplications struct{ scopeToolApplications }

func (missingScopeToolApplications) DiagnosticApplicationContextByLocator(context.Context, string, string, string) (appdomain.DiagnosticContext, error) {
	return appdomain.DiagnosticContext{}, appdomain.ErrNotFound
}

func TestDiagnosticScopeToolsAreBoundedAndStripInternalIDs(t *testing.T) {
	service := &Service{applications: scopeToolApplications{}}
	tools := service.diagnosticScopeTools(context.Background(), "scope-1")
	if len(tools) != 4 {
		t.Fatalf("tool count = %d", len(tools))
	}
	for _, item := range tools {
		definition := item.Definition()
		if definition.Source != scopeToolSource || !definition.ReadOnly {
			t.Fatalf("definition = %#v", definition)
		}
	}
	result, err := tools[3].Invoke(context.Background(), map[string]any{"application_code": "orders"})
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(result.Output)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "internal-app-id") || strings.Contains(string(encoded), "internal-resource-id") {
		t.Fatalf("internal IDs leaked from scope tool: %v", result.Output)
	}
}

func TestDiagnosticApplicationContextMissDoesNotTerminateObjectResolution(t *testing.T) {
	service := &Service{applications: missingScopeToolApplications{}}
	tools := service.diagnosticScopeTools(context.Background(), "scope-1")
	definition := tools[3].Definition()
	if !strings.Contains(definition.Description, "selected_resources") || !strings.Contains(string(definition.InputSchema), "application_name") {
		t.Fatalf("application lookup definition is ambiguous: %#v", definition)
	}
	result, err := tools[3].Invoke(context.Background(), map[string]any{"application_name": "git"})
	if err != nil {
		t.Fatalf("not-found application should be a recoverable result: %v", err)
	}
	encoded, err := json.Marshal(result.Output)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(encoded), `"found":false`) || !strings.Contains(string(encoded), "resource-specific tools") {
		t.Fatalf("application miss did not direct the model to continue object resolution: %s", encoded)
	}
}

func TestDiagnosticScopeApplicationPageSerializationStaysBounded(t *testing.T) {
	page := appdomain.DiagnosticScopeApplicationPage{Items: make([]appdomain.DiagnosticScopeApplication, 0, 100), Total: 50000, Limit: 100}
	for index := 0; index < 100; index++ {
		page.Items = append(page.Items, appdomain.DiagnosticScopeApplication{
			TeamName: fmt.Sprintf("team-%d", index%50), ProjectName: fmt.Sprintf("project-%d", index), ProjectCode: fmt.Sprintf("project-%d", index),
			Name: fmt.Sprintf("application-%d", index), Code: fmt.Sprintf("app-%d", index), RuntimeKind: appdomain.RuntimeDocker, Status: "active", InstanceCount: 2,
		})
	}
	encoded, err := json.Marshal(page)
	if err != nil {
		t.Fatal(err)
	}
	if len(encoded) >= 50000 {
		t.Fatalf("one bounded page is too large: %d bytes", len(encoded))
	}
}

func TestWithAuthorizationContextPreservesServerFilters(t *testing.T) {
	source := authorization.WithScopeFilter(context.Background(), authorization.ScopeFilter{SubjectID: "user-1", ScopeIDs: []string{"scope-1"}})
	source = authorization.WithResourceFilter(source, authorization.ResourceFilter{SubjectID: "user-1", ScopeIDs: []string{"scope-1"}, ResourceIDs: []string{"resource-1"}})
	source = authorization.WithResourceUseFilter(source, authorization.ResourceFilter{SubjectID: "user-1", ScopeIDs: []string{"scope-1"}, ResourceIDs: []string{"resource-2"}})
	source = authorization.WithResourceReadFilter(source, authorization.ResourceFilter{SubjectID: "user-1", ScopeIDs: []string{"scope-1"}, ResourceIDs: []string{"resource-3"}})
	got := withAuthorizationContext(context.Background(), source)
	if filter, ok := authorization.ScopeFilterFromContext(got); !ok || !filter.Allows("scope-1") {
		t.Fatalf("scope filter was not preserved: %#v, %v", filter, ok)
	}
	if filter, ok := authorization.ResourceFilterFromContext(got); !ok || !filter.Allows("", "resource-1") {
		t.Fatalf("resource filter was not preserved: %#v, %v", filter, ok)
	}
	if filter, ok := authorization.ResourceUseFilterFromContext(got); !ok || !filter.Allows("", "resource-2") {
		t.Fatalf("resource-use filter was not preserved: %#v, %v", filter, ok)
	}
	if filter, ok := authorization.ResourceReadFilterFromContext(got); !ok || !filter.Allows("", "resource-3") {
		t.Fatalf("resource-read filter was not preserved: %#v, %v", filter, ok)
	}
}
