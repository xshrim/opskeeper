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

	"opskeeper/backend/audit"
	"opskeeper/backend/authorization"
	"opskeeper/backend/health"
	"opskeeper/backend/identity"
)

type stubAuthorizationService struct {
	filter authorization.ScopeFilter
}

type stubAuditQueryService struct{}

func (stubAuditQueryService) List(context.Context, []string, int) (audit.Page, error) {
	return audit.Page{Items: []audit.Event{}, Total: 0}, nil
}

func (s *stubAuthorizationService) ScopeFilter(context.Context, authorization.Subject, authorization.Permission) (authorization.ScopeFilter, error) {
	return s.filter, nil
}

func (s *stubAuthorizationService) ResourceFilter(context.Context, authorization.Subject, authorization.Permission) (authorization.ResourceFilter, error) {
	return authorization.ResourceFilter{SubjectID: s.filter.SubjectID, Permission: s.filter.Permission, ScopeIDs: s.filter.ScopeIDs}, nil
}

func newAuthorizedTestRouter(filter authorization.ScopeFilter) http.Handler {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	identityStub := &stubIdentityService{user: identity.User{ID: handlerTestUUID, Status: identity.StatusActive}}
	return NewRouter(logger, health.NewService("test-api", time.Second, nil), testBuild, Options{
		BasePath:      "/test",
		Identity:      identityStub,
		Authorization: &stubAuthorizationService{filter: filter},
	}, &stubOrganizationService{}, nil)
}

func TestAuthorizationMiddlewareRejectsEmptyScopeFilter(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/test/api/v1/teams/", nil)
	request.AddCookie(&http.Cookie{Name: accessCookieName, Value: "access-token"})
	response := httptest.NewRecorder()

	newAuthorizedTestRouter(authorization.ScopeFilter{Permission: authorization.OrganizationRead}).ServeHTTP(response, request)

	if response.Code != http.StatusForbidden || !strings.Contains(response.Body.String(), `"code":"forbidden"`) {
		t.Fatalf("authorization response = %d %s", response.Code, response.Body.String())
	}
}

func TestAuthorizationMiddlewarePassesScopeFilterToOrganization(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/test/api/v1/teams/", nil)
	request.AddCookie(&http.Cookie{Name: accessCookieName, Value: "access-token"})
	response := httptest.NewRecorder()

	newAuthorizedTestRouter(authorization.ScopeFilter{Permission: authorization.OrganizationRead, ScopeIDs: []string{handlerTestUUID}}).ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("authorized organization response = %d %s", response.Code, response.Body.String())
	}
}

func TestAuditRouteRequiresAuthenticationBeforeAuthorization(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	router := NewRouter(logger, health.NewService("test-api", time.Second, nil), testBuild, Options{
		BasePath:      "/test",
		Identity:      &stubIdentityService{authenticateError: identity.ErrInvalidSession},
		Authorization: &stubAuthorizationService{filter: authorization.ScopeFilter{ScopeIDs: []string{handlerTestUUID}}},
		AuditLog:      stubAuditQueryService{},
	}, nil, nil)
	request := httptest.NewRequest(http.MethodGet, "/test/api/v1/audit-logs", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("audit route status = %d, body = %s", response.Code, response.Body.String())
	}
}
