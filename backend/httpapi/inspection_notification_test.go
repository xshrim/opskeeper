package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"opskeeper/backend/authorization"
	"opskeeper/backend/inspection"
	"opskeeper/backend/notification"
)

type inspectionNotificationFake struct {
	inspectionService
	notificationAdminService
	notificationCatalogHTTPService
	created inspection.NotificationTemplateRecord
	listed  []inspection.NotificationTemplateRecord
	denied  bool
}

func (f *inspectionNotificationFake) CreateNotificationTemplate(_ context.Context, scopeID, name, _ string, _ bool, draft notification.TemplateDraft) (inspection.NotificationTemplateRecord, error) {
	if f.denied {
		return inspection.NotificationTemplateRecord{}, authorization.ErrForbidden
	}
	f.created = inspection.NotificationTemplateRecord{ID: "template-1", ScopeID: scopeID, Name: name}
	return f.created, nil
}

func (f *inspectionNotificationFake) ListNotificationTemplates(context.Context, string) ([]inspection.NotificationTemplateRecord, error) {
	if f.denied {
		return nil, authorization.ErrForbidden
	}
	return f.listed, nil
}

func (f *inspectionNotificationFake) TestNotificationRule(context.Context, string, string) (int, error) {
	return 2, nil
}

func notificationTestRouter(service inspectionService, guard func(authorization.Permission) func(http.Handler) http.Handler) http.Handler {
	router := chi.NewRouter()
	registerInspectionRoutes(router, service, nil, guard)
	return router
}

func TestNotificationTemplateAPIValidatesAndReturnsScopeSafeResponse(t *testing.T) {
	fake := &inspectionNotificationFake{}
	permissions := make([]authorization.Permission, 0)
	router := notificationTestRouter(fake, func(permission authorization.Permission) func(http.Handler) http.Handler {
		permissions = append(permissions, permission)
		return func(next http.Handler) http.Handler { return next }
	})
	request := httptest.NewRequest(http.MethodPost, "/notification-templates", strings.NewReader(`{"scope_id":"scope-1","name":"Incident","format":"markdown","body_template":"{{.finding_summary}}","variables":[{"name":"finding_summary","required":true}]}`))
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusCreated || !strings.Contains(response.Body.String(), `"id":"template-1"`) {
		t.Fatalf("create response = %d %s", response.Code, response.Body.String())
	}
	if !containsPermission(permissions, authorization.NotificationManage) {
		t.Fatalf("template create permission = %v", permissions)
	}

	fake.listed = []inspection.NotificationTemplateRecord{{ID: "template-1", ScopeID: "scope-1", Name: "Incident"}}
	request = httptest.NewRequest(http.MethodGet, "/notification-templates?scope_id=scope-1", nil)
	response = httptest.NewRecorder()
	router.ServeHTTP(response, request)
	var listed []inspection.NotificationTemplateRecord
	if err := json.Unmarshal(response.Body.Bytes(), &listed); err != nil || response.Code != http.StatusOK || len(listed) != 1 {
		t.Fatalf("list response = %d %s, %v", response.Code, response.Body.String(), err)
	}
}

func TestNotificationTemplateAPIHandlesBadInputForbiddenAndPermissionDenial(t *testing.T) {
	fake := &inspectionNotificationFake{}
	validationRouter := notificationTestRouter(fake, nil)
	request := httptest.NewRequest(http.MethodPost, "/notification-templates", strings.NewReader("{"))
	response := httptest.NewRecorder()
	validationRouter.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("malformed JSON status = %d, want 400: %s", response.Code, response.Body.String())
	}
	router := notificationTestRouter(fake, func(permission authorization.Permission) func(http.Handler) http.Handler {
		return func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				if permission == authorization.NotificationRead {
					next.ServeHTTP(writer, request)
					return
				}
				writeError(writer, request, http.StatusUnauthorized, "invalid_session", "Authentication is required")
			})
		}
	})
	request = httptest.NewRequest(http.MethodPost, "/notification-templates", strings.NewReader("{"))
	response = httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated malformed request status = %d, want 401", response.Code)
	}
	request = httptest.NewRequest(http.MethodPost, "/notification-templates", strings.NewReader(`{"scope_id":"scope-1","name":"Incident","format":"text"}`))
	response = httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("manage permission denial status = %d, want 401", response.Code)
	}

	fake.denied = true
	request = httptest.NewRequest(http.MethodGet, "/notification-templates?scope_id=scope-1", nil)
	response = httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("Scope denial status = %d, want 403", response.Code)
	}
}

func TestNotificationRuleTestUsesDedicatedPermission(t *testing.T) {
	fake := &inspectionNotificationFake{}
	permissions := make([]authorization.Permission, 0)
	router := notificationTestRouter(fake, func(permission authorization.Permission) func(http.Handler) http.Handler {
		permissions = append(permissions, permission)
		return func(next http.Handler) http.Handler { return next }
	})
	request := httptest.NewRequest(http.MethodPost, "/notification-rules/rule-1/test?scope_id=scope-1", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"routes_tested":2`) {
		t.Fatalf("rule test response = %d %s", response.Code, response.Body.String())
	}
	if !containsPermission(permissions, authorization.NotificationTest) {
		t.Fatalf("rule test permission = %v", permissions)
	}
}

func TestNotificationRoutesUseDedicatedPermissions(t *testing.T) {
	tests := []struct {
		method     string
		path       string
		permission authorization.Permission
	}{
		{http.MethodGet, "/notification-channels?scope_id=scope-1", authorization.NotificationRead},
		{http.MethodPost, "/notification-channels", authorization.NotificationManage},
		{http.MethodPost, "/notification-channels/channel-1/test", authorization.NotificationTest},
		{http.MethodGet, "/notification-templates?scope_id=scope-1", authorization.NotificationRead},
		{http.MethodPost, "/notification-rules", authorization.NotificationManage},
		{http.MethodPost, "/notification-rules/rule-1/test?scope_id=scope-1", authorization.NotificationTest},
		{http.MethodGet, "/notification-deliveries?scope_id=scope-1", authorization.NotificationRead},
		{http.MethodPost, "/notification-deliveries/delivery-1/retry", authorization.NotificationRetry},
	}

	for _, test := range tests {
		t.Run(test.method+" "+test.path, func(t *testing.T) {
			var used authorization.Permission
			router := notificationTestRouter(&inspectionNotificationFake{}, func(permission authorization.Permission) func(http.Handler) http.Handler {
				return func(http.Handler) http.Handler {
					return http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
						used = permission
						writer.WriteHeader(http.StatusNoContent)
					})
				}
			})
			request := httptest.NewRequest(test.method, test.path, nil)
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			if response.Code != http.StatusNoContent || used != test.permission {
				t.Fatalf("status=%d permission=%q, want %q", response.Code, used, test.permission)
			}
		})
	}
}

func containsPermission(values []authorization.Permission, target authorization.Permission) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
