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
	"opskeeper/backend/identity"
	"opskeeper/backend/resource"
)

type stubResourceService struct{}

func (stubResourceService) Create(context.Context, resource.CreateInput) (resource.Resource, error) {
	return resource.Resource{ID: handlerTestUUID, ScopeID: handlerTestUUID, Kind: "Redis", Name: "shared"}, nil
}
func (stubResourceService) List(context.Context, resource.Pagination, string, map[string]string) (resource.Page[resource.Resource], error) {
	return resource.Page[resource.Resource]{Items: []resource.Resource{}}, nil
}
func (stubResourceService) Get(context.Context, string) (resource.Resource, error) {
	return resource.Resource{ID: handlerTestUUID}, nil
}
func (stubResourceService) Update(context.Context, string, resource.UpdateInput) (resource.Resource, error) {
	return resource.Resource{ID: handlerTestUUID}, nil
}
func (stubResourceService) Delete(context.Context, string) error { return nil }
func (stubResourceService) ListSchemas(context.Context) ([]resource.Schema, error) {
	return []resource.Schema{}, nil
}
func (stubResourceService) CreateRelation(context.Context, string, resource.CreateRelationInput) (resource.Relation, error) {
	return resource.Relation{ID: handlerTestUUID}, nil
}
func (stubResourceService) ListRelations(context.Context, string) ([]resource.Relation, error) {
	return []resource.Relation{}, nil
}
func (stubResourceService) DeleteRelation(context.Context, string, string) error { return nil }
func (stubResourceService) Topology(context.Context, string, int, int) ([]resource.TopologyNode, error) {
	return []resource.TopologyNode{}, nil
}
func (stubResourceService) SetDefault(context.Context, string, string, string) (resource.Default, error) {
	return resource.Default{}, nil
}
func (stubResourceService) ResolveDefault(context.Context, string, string) (resource.Resource, error) {
	return resource.Resource{}, nil
}

func TestCredentialRoutesAreRemoved(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	router := NewRouter(logger, health.NewService("test-api", time.Second, nil), testBuild, Options{
		BasePath: "/test",
		Identity: &stubIdentityService{user: identity.User{ID: handlerTestUUID, Status: identity.StatusActive}},
	}, nil, nil)
	request := httptest.NewRequest(http.MethodPost, "/test/api/v1/credentials", strings.NewReader(`{"scope_id":"11111111-1111-4111-8111-111111111111","name":"redis","secret":"do-not-return"}`))
	request.AddCookie(&http.Cookie{Name: accessCookieName, Value: "access-token"})
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusNotFound {
		t.Fatalf("POST credentials status = %d, body = %s", response.Code, response.Body.String())
	}
}
