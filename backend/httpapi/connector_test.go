package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"opskeeper/backend/authorization"
	"opskeeper/backend/connector"
	"opskeeper/backend/health"
	"opskeeper/backend/identity"
	"opskeeper/backend/resource"
)

type stubConnectorService struct {
	check      connector.Check
	err        error
	actorID    string
	resourceID string
	calls      int
}

type stubApplicationDiscoveryConnectorService struct {
	stubConnectorService
	hostKeyword    string
	hostResource   string
	hostPID        int
	discoveryCalls int
}

func (s *stubApplicationDiscoveryConnectorService) DiscoverApplicationHostProcesses(_ context.Context, resourceID, keyword string, _ int) (connector.ApplicationHostProcesses, error) {
	s.discoveryCalls++
	s.hostResource, s.hostKeyword = resourceID, keyword
	return connector.ApplicationHostProcesses{Keyword: keyword}, nil
}
func (s *stubApplicationDiscoveryConnectorService) ValidateApplicationHostProcess(_ context.Context, resourceID, keyword string, pid int) (connector.ApplicationHostProcessValidation, error) {
	s.discoveryCalls++
	s.hostResource, s.hostKeyword, s.hostPID = resourceID, keyword, pid
	return connector.ApplicationHostProcessValidation{Keyword: keyword, PID: pid, Valid: true}, nil
}
func (s *stubApplicationDiscoveryConnectorService) DiscoverApplicationDockerContainers(context.Context, string, string, int) (connector.ApplicationDockerContainers, error) {
	s.discoveryCalls++
	return connector.ApplicationDockerContainers{}, nil
}
func (s *stubApplicationDiscoveryConnectorService) DiscoverApplicationKubernetesNamespaces(context.Context, string, bool, int) (connector.ApplicationKubernetesNamespaces, error) {
	s.discoveryCalls++
	return connector.ApplicationKubernetesNamespaces{}, nil
}
func (s *stubApplicationDiscoveryConnectorService) DiscoverApplicationKubernetesWorkloads(context.Context, string, string, int) (connector.ApplicationKubernetesWorkloads, error) {
	s.discoveryCalls++
	return connector.ApplicationKubernetesWorkloads{}, nil
}

func (s *stubConnectorService) Test(_ context.Context, actorID, resourceID string) (connector.Check, error) {
	s.calls++
	s.actorID = actorID
	s.resourceID = resourceID
	return s.check, s.err
}

func (s *stubConnectorService) Latest(_ context.Context, resourceID string) (connector.Check, error) {
	s.calls++
	s.resourceID = resourceID
	return s.check, s.err
}

type connectorAuthorizationService struct {
	filter      authorization.ResourceFilter
	permissions []authorization.Permission
}

func (s *connectorAuthorizationService) ScopeFilter(context.Context, authorization.Subject, authorization.Permission) (authorization.ScopeFilter, error) {
	return authorization.ScopeFilter{}, nil
}

func (s *connectorAuthorizationService) ResourceFilter(_ context.Context, _ authorization.Subject, permission authorization.Permission) (authorization.ResourceFilter, error) {
	s.permissions = append(s.permissions, permission)
	return s.filter, nil
}

func newConnectorTestRouter(service connectorService, authorizationService authorizationService) http.Handler {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return NewRouter(logger, health.NewService("test-api", time.Second, nil), testBuild, Options{
		BasePath:      "/test",
		Identity:      &stubIdentityService{user: identity.User{ID: handlerTestUUID, Status: identity.StatusActive}},
		Authorization: authorizationService,
		Connectors:    service,
	}, nil, nil)
}

func TestConnectorRoutesUseResourcePermissions(t *testing.T) {
	tests := []struct {
		name           string
		method         string
		path           string
		wantPermission authorization.Permission
	}{
		{name: "test connection", method: http.MethodPost, path: "/test/api/v1/resources/" + handlerTestUUID + "/connection-tests", wantPermission: authorization.ResourceUse},
		{name: "latest result", method: http.MethodGet, path: "/test/api/v1/resources/" + handlerTestUUID + "/connection-tests/latest", wantPermission: authorization.ResourceRead},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service := &stubConnectorService{check: connector.Check{ID: handlerTestUUID, ResourceID: handlerTestUUID, Status: "succeeded", Message: "连接测试通过"}}
			authorizer := &connectorAuthorizationService{filter: authorization.ResourceFilter{ResourceIDs: []string{handlerTestUUID}}}
			request := httptest.NewRequest(test.method, test.path, nil)
			request.AddCookie(&http.Cookie{Name: accessCookieName, Value: "access-token"})
			response := httptest.NewRecorder()
			newConnectorTestRouter(service, authorizer).ServeHTTP(response, request)

			if response.Code != http.StatusOK {
				t.Fatalf("response = %d %s", response.Code, response.Body.String())
			}
			if len(authorizer.permissions) != 1 || authorizer.permissions[0] != test.wantPermission {
				t.Fatalf("permissions = %#v, want %q", authorizer.permissions, test.wantPermission)
			}
			if service.resourceID != handlerTestUUID {
				t.Fatalf("resourceID = %q", service.resourceID)
			}
			if test.method == http.MethodPost && service.actorID != handlerTestUUID {
				t.Fatalf("actorID = %q", service.actorID)
			}
		})
	}
}

func TestConnectorRouteRejectsMissingPermissionBeforeService(t *testing.T) {
	service := &stubConnectorService{}
	authorizer := &connectorAuthorizationService{}
	request := httptest.NewRequest(http.MethodPost, "/test/api/v1/resources/"+handlerTestUUID+"/connection-tests", nil)
	request.AddCookie(&http.Cookie{Name: accessCookieName, Value: "access-token"})
	response := httptest.NewRecorder()
	newConnectorTestRouter(service, authorizer).ServeHTTP(response, request)

	if response.Code != http.StatusForbidden || service.calls != 0 {
		t.Fatalf("response = %d %s, service calls = %d", response.Code, response.Body.String(), service.calls)
	}
}

func TestApplicationDiscoveryRoutesUseResourceUseAndForwardArguments(t *testing.T) {
	service := &stubApplicationDiscoveryConnectorService{}
	authorizer := &connectorAuthorizationService{filter: authorization.ResourceFilter{ResourceIDs: []string{handlerTestUUID}}}
	request := httptest.NewRequest(http.MethodGet, "/test/api/v1/resources/"+handlerTestUUID+"/application-targets/host-processes?keyword=orders&limit=7", nil)
	request.AddCookie(&http.Cookie{Name: accessCookieName, Value: "access-token"})
	response := httptest.NewRecorder()
	newConnectorTestRouter(service, authorizer).ServeHTTP(response, request)
	if response.Code != http.StatusOK || service.discoveryCalls != 1 || service.hostResource != handlerTestUUID || service.hostKeyword != "orders" {
		t.Fatalf("response = %d %s, calls=%d resource=%q keyword=%q", response.Code, response.Body.String(), service.discoveryCalls, service.hostResource, service.hostKeyword)
	}
	if len(authorizer.permissions) != 1 || authorizer.permissions[0] != authorization.ResourceUse {
		t.Fatalf("permissions = %#v", authorizer.permissions)
	}
}

func TestApplicationDiscoveryRouteRejectsMissingPermissionBeforeService(t *testing.T) {
	service := &stubApplicationDiscoveryConnectorService{}
	authorizer := &connectorAuthorizationService{}
	request := httptest.NewRequest(http.MethodGet, "/test/api/v1/resources/"+handlerTestUUID+"/application-targets/host-processes?keyword=orders", nil)
	request.AddCookie(&http.Cookie{Name: accessCookieName, Value: "access-token"})
	response := httptest.NewRecorder()
	newConnectorTestRouter(service, authorizer).ServeHTTP(response, request)
	if response.Code != http.StatusForbidden || service.discoveryCalls != 0 {
		t.Fatalf("response = %d %s, calls = %d", response.Code, response.Body.String(), service.discoveryCalls)
	}
}

func TestApplicationHostValidationRouteDecodesBody(t *testing.T) {
	service := &stubApplicationDiscoveryConnectorService{}
	authorizer := &connectorAuthorizationService{filter: authorization.ResourceFilter{ResourceIDs: []string{handlerTestUUID}}}
	request := httptest.NewRequest(http.MethodPost, "/test/api/v1/resources/"+handlerTestUUID+"/application-targets/host-processes/validate", strings.NewReader(`{"keyword":"orders&java","pid":42}`))
	request.Header.Set("Content-Type", "application/json")
	request.AddCookie(&http.Cookie{Name: accessCookieName, Value: "access-token"})
	response := httptest.NewRecorder()
	newConnectorTestRouter(service, authorizer).ServeHTTP(response, request)
	if response.Code != http.StatusOK || service.hostResource != handlerTestUUID || service.hostKeyword != "orders&java" || service.hostPID != 42 {
		t.Fatalf("response = %d %s, resource=%q keyword=%q pid=%d", response.Code, response.Body.String(), service.hostResource, service.hostKeyword, service.hostPID)
	}
	var result connector.ApplicationHostProcessValidation
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil || !result.Valid {
		t.Fatalf("result = %+v, err=%v", result, err)
	}
}

func TestConnectorRouteMapsSafeErrors(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		status int
		code   string
	}{
		{name: "invalid", err: connector.ErrInvalid, status: http.StatusBadRequest, code: "invalid_request"},
		{name: "resource missing", err: resource.ErrNotFound, status: http.StatusNotFound, code: "not_found"},
		{name: "check missing", err: connector.ErrNotFound, status: http.StatusNotFound, code: "connection_check_not_found"},
		{name: "internal", err: errors.New("database secret"), status: http.StatusInternalServerError, code: "internal_error"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service := &stubConnectorService{err: test.err}
			authorizer := &connectorAuthorizationService{filter: authorization.ResourceFilter{ResourceIDs: []string{handlerTestUUID}}}
			request := httptest.NewRequest(http.MethodGet, "/test/api/v1/resources/"+handlerTestUUID+"/connection-tests/latest", nil)
			request.AddCookie(&http.Cookie{Name: accessCookieName, Value: "access-token"})
			response := httptest.NewRecorder()
			newConnectorTestRouter(service, authorizer).ServeHTTP(response, request)

			if response.Code != test.status || !strings.Contains(response.Body.String(), `"code":"`+test.code+`"`) {
				t.Fatalf("response = %d %s", response.Code, response.Body.String())
			}
			if strings.Contains(response.Body.String(), "database secret") {
				t.Fatalf("response leaked internal error: %s", response.Body.String())
			}
		})
	}
}
