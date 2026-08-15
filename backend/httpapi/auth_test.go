package httpapi

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"opskeeper/backend/health"
	"opskeeper/backend/identity"
)

type stubIdentityService struct {
	user               identity.User
	tokens             identity.SessionTokens
	loginError         error
	refreshError       error
	authenticateError  error
	authenticatedToken string
	logoutAllUserID    string
}

func (s *stubIdentityService) Login(context.Context, string, string, identity.SessionMetadata) (identity.User, identity.SessionTokens, error) {
	return s.user, s.tokens, s.loginError
}

func (s *stubIdentityService) Refresh(context.Context, string, identity.SessionMetadata) (identity.User, identity.SessionTokens, error) {
	return s.user, s.tokens, s.refreshError
}

func (s *stubIdentityService) Authenticate(_ context.Context, token string) (identity.User, error) {
	s.authenticatedToken = token
	return s.user, s.authenticateError
}

func (s *stubIdentityService) Logout(context.Context, string, string) error {
	return nil
}

func (s *stubIdentityService) LogoutAll(_ context.Context, userID string) error {
	s.logoutAllUserID = userID
	return nil
}

func newAuthTestRouter(service identityService, secure bool) http.Handler {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return NewRouter(logger, health.NewService("test-api", time.Second, nil), testBuild, Options{
		BasePath:     "/test",
		Identity:     service,
		CookieSecure: secure,
	}, nil, nil)
}

func TestLoginSetsSecureHTTPOnlyCookiesWithoutReturningTokens(t *testing.T) {
	now := time.Now()
	service := &stubIdentityService{
		user: identity.User{ID: handlerTestUUID, Email: "admin@example.com", Status: identity.StatusActive},
		tokens: identity.SessionTokens{
			AccessToken:      "access-secret",
			RefreshToken:     "refresh-secret",
			AccessExpiresAt:  now.Add(15 * time.Minute),
			RefreshExpiresAt: now.Add(24 * time.Hour),
		},
	}
	request := httptest.NewRequest(http.MethodPost, "/test/api/v1/auth/login", strings.NewReader(`{"email":"admin@example.com","password":"secret password"}`))
	response := httptest.NewRecorder()

	newAuthTestRouter(service, true).ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("login status = %d, body = %s", response.Code, response.Body.String())
	}
	if strings.Contains(response.Body.String(), "access-secret") || strings.Contains(response.Body.String(), "refresh-secret") || strings.Contains(response.Body.String(), "secret password") {
		t.Fatalf("login response leaked a secret: %s", response.Body.String())
	}
	cookies := response.Result().Cookies()
	if len(cookies) != 2 {
		t.Fatalf("login cookies = %d, want 2", len(cookies))
	}
	for _, cookie := range cookies {
		if !cookie.HttpOnly || !cookie.Secure || cookie.SameSite != http.SameSiteLaxMode || cookie.Path != "/test" || cookie.MaxAge <= 0 {
			t.Fatalf("login cookie = %#v", cookie)
		}
	}
}

func TestAuthenticationMiddlewareAcceptsCookieAndBearerToken(t *testing.T) {
	service := &stubIdentityService{user: identity.User{ID: handlerTestUUID, Email: "admin@example.com"}}
	router := newAuthTestRouter(service, false)

	cookieRequest := httptest.NewRequest(http.MethodGet, "/test/api/v1/auth/me", nil)
	cookieRequest.AddCookie(&http.Cookie{Name: accessCookieName, Value: "cookie-token"})
	cookieResponse := httptest.NewRecorder()
	router.ServeHTTP(cookieResponse, cookieRequest)
	if cookieResponse.Code != http.StatusOK || service.authenticatedToken != "cookie-token" {
		t.Fatalf("cookie authentication = %d token=%q", cookieResponse.Code, service.authenticatedToken)
	}

	bearerRequest := httptest.NewRequest(http.MethodGet, "/test/api/v1/auth/me", nil)
	bearerRequest.Header.Set("Authorization", "Bearer bearer-token")
	bearerResponse := httptest.NewRecorder()
	router.ServeHTTP(bearerResponse, bearerRequest)
	if bearerResponse.Code != http.StatusOK || service.authenticatedToken != "bearer-token" {
		t.Fatalf("bearer authentication = %d token=%q", bearerResponse.Code, service.authenticatedToken)
	}
}

func TestAuthenticationMiddlewareRejectsInvalidSession(t *testing.T) {
	service := &stubIdentityService{authenticateError: identity.ErrInvalidSession}
	request := httptest.NewRequest(http.MethodGet, "/test/api/v1/auth/me", nil)
	response := httptest.NewRecorder()

	newAuthTestRouter(service, false).ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized || !strings.Contains(response.Body.String(), `"code":"invalid_session"`) {
		t.Fatalf("authentication response = %d %s", response.Code, response.Body.String())
	}
}

func TestRefreshRejectsReplayAndClearsCookies(t *testing.T) {
	service := &stubIdentityService{refreshError: identity.ErrInvalidSession}
	request := httptest.NewRequest(http.MethodPost, "/test/api/v1/auth/refresh", nil)
	request.AddCookie(&http.Cookie{Name: refreshCookieName, Value: "replayed-token"})
	response := httptest.NewRecorder()

	newAuthTestRouter(service, true).ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("refresh status = %d, body = %s", response.Code, response.Body.String())
	}
	for _, cookie := range response.Result().Cookies() {
		if cookie.MaxAge != -1 || !cookie.HttpOnly || !cookie.Secure {
			t.Fatalf("cleared cookie = %#v", cookie)
		}
	}
}

func TestLogoutAllUsesAuthenticatedUser(t *testing.T) {
	service := &stubIdentityService{user: identity.User{ID: handlerTestUUID}}
	request := httptest.NewRequest(http.MethodPost, "/test/api/v1/auth/logout-all", nil)
	request.AddCookie(&http.Cookie{Name: accessCookieName, Value: "access-token"})
	response := httptest.NewRecorder()

	newAuthTestRouter(service, false).ServeHTTP(response, request)

	if response.Code != http.StatusNoContent || service.logoutAllUserID != handlerTestUUID {
		t.Fatalf("logout-all response = %d user=%q", response.Code, service.logoutAllUserID)
	}
}

func TestBootstrapIsNotExposedOverHTTP(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/test/api/v1/auth/bootstrap", strings.NewReader(`{}`))
	response := httptest.NewRecorder()

	newAuthTestRouter(&stubIdentityService{}, false).ServeHTTP(response, request)

	if response.Code != http.StatusNotFound {
		t.Fatalf("bootstrap status = %d, want 404", response.Code)
	}
}

func TestOrganizationRoutesRequireIdentityWhenAuthenticationIsEnabled(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	identityStub := &stubIdentityService{authenticateError: identity.ErrInvalidSession}
	router := NewRouter(logger, health.NewService("test-api", time.Second, nil), testBuild, Options{
		BasePath: "/test",
		Identity: identityStub,
	}, &stubOrganizationService{}, nil)
	request := httptest.NewRequest(http.MethodGet, "/test/api/v1/platform", nil)
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("organization route status = %d, body = %s", response.Code, response.Body.String())
	}
}

func TestLoginMapsInvalidCredentials(t *testing.T) {
	service := &stubIdentityService{loginError: identity.ErrInvalidCredentials}
	request := httptest.NewRequest(http.MethodPost, "/test/api/v1/auth/login", strings.NewReader(`{"email":"admin@example.com","password":"wrong"}`))
	response := httptest.NewRecorder()

	newAuthTestRouter(service, false).ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized || !strings.Contains(response.Body.String(), `"code":"invalid_credentials"`) {
		t.Fatalf("login response = %d %s", response.Code, response.Body.String())
	}
}

func TestLoginMapsInternalErrorWithoutDetails(t *testing.T) {
	service := &stubIdentityService{loginError: errors.New("database password leaked here")}
	request := httptest.NewRequest(http.MethodPost, "/test/api/v1/auth/login", strings.NewReader(`{"email":"admin@example.com","password":"secret"}`))
	response := httptest.NewRecorder()

	newAuthTestRouter(service, false).ServeHTTP(response, request)

	if response.Code != http.StatusInternalServerError || strings.Contains(response.Body.String(), "database password") {
		t.Fatalf("login response = %d %s", response.Code, response.Body.String())
	}
}
