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

	"opskeeper/backend/authorization"
	"opskeeper/backend/health"
	"opskeeper/backend/identity"
)

type stubAuthorizationService struct {
	filter authorization.ScopeFilter
}

func (s *stubAuthorizationService) ScopeFilter(context.Context, authorization.Subject, authorization.Permission) (authorization.ScopeFilter, error) {
	return s.filter, nil
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
