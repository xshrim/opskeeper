package httpapi

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"opskeeper/backend/identity"
)

const (
	accessCookieName  = "opsk_access"
	refreshCookieName = "opsk_refresh"
)

type identityService interface {
	Login(context.Context, string, string, identity.SessionMetadata) (identity.User, identity.SessionTokens, error)
	Refresh(context.Context, string, identity.SessionMetadata) (identity.User, identity.SessionTokens, error)
	Authenticate(context.Context, string) (identity.User, error)
	Logout(context.Context, string, string) error
	LogoutAll(context.Context, string) error
}

type authHandler struct {
	service      identityService
	cookiePath   string
	cookieSecure bool
}

type authRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type authenticatedUserContextKey struct{}

func registerAuthRoutes(router chi.Router, service identityService, basePath string, cookieSecure bool) {
	handler := authHandler{service: service, cookiePath: basePath, cookieSecure: cookieSecure}
	router.Route("/auth", func(router chi.Router) {
		router.Post("/login", handler.login)
		router.Post("/refresh", handler.refresh)
		router.Post("/logout", handler.logout)
		router.With(handler.requireAuth).Get("/me", handler.me)
		router.With(handler.requireAuth).Post("/logout-all", handler.logoutAll)
	})
}

func (h authHandler) login(writer http.ResponseWriter, request *http.Request) {
	var body authRequest
	if !decodeRequest(writer, request, &body) {
		return
	}
	user, tokens, err := h.service.Login(request.Context(), body.Email, body.Password, sessionMetadata(request))
	if err != nil {
		writeIdentityError(writer, request, err)
		return
	}
	h.setSessionCookies(writer, tokens)
	writeJSON(writer, http.StatusOK, user)
}

func (h authHandler) refresh(writer http.ResponseWriter, request *http.Request) {
	refreshToken := cookieValue(request, refreshCookieName)
	user, tokens, err := h.service.Refresh(request.Context(), refreshToken, sessionMetadata(request))
	if err != nil {
		h.clearSessionCookies(writer)
		writeIdentityError(writer, request, err)
		return
	}
	h.setSessionCookies(writer, tokens)
	writeJSON(writer, http.StatusOK, user)
}

func (h authHandler) logout(writer http.ResponseWriter, request *http.Request) {
	if err := h.service.Logout(request.Context(), cookieValue(request, accessCookieName), cookieValue(request, refreshCookieName)); err != nil {
		writeIdentityError(writer, request, err)
		return
	}
	h.clearSessionCookies(writer)
	writer.WriteHeader(http.StatusNoContent)
}

func (h authHandler) me(writer http.ResponseWriter, request *http.Request) {
	user, ok := request.Context().Value(authenticatedUserContextKey{}).(identity.User)
	if !ok {
		writeError(writer, request, http.StatusUnauthorized, "invalid_session", "Authentication is required")
		return
	}
	writeJSON(writer, http.StatusOK, user)
}

func (h authHandler) logoutAll(writer http.ResponseWriter, request *http.Request) {
	user, ok := request.Context().Value(authenticatedUserContextKey{}).(identity.User)
	if !ok {
		writeError(writer, request, http.StatusUnauthorized, "invalid_session", "Authentication is required")
		return
	}
	if err := h.service.LogoutAll(request.Context(), user.ID); err != nil {
		writeIdentityError(writer, request, err)
		return
	}
	h.clearSessionCookies(writer)
	writer.WriteHeader(http.StatusNoContent)
}

func (h authHandler) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		token := cookieValue(request, accessCookieName)
		if token == "" {
			token = bearerToken(request)
		}
		user, err := h.service.Authenticate(request.Context(), token)
		if err != nil {
			writeIdentityError(writer, request, err)
			return
		}
		ctx := context.WithValue(request.Context(), authenticatedUserContextKey{}, user)
		next.ServeHTTP(writer, request.WithContext(ctx))
	})
}

func sessionMetadata(request *http.Request) identity.SessionMetadata {
	return identity.SessionMetadata{
		UserAgent: truncateMetadata(request.UserAgent(), 512),
		ClientIP:  truncateMetadata(requestClientIP(request), 255),
	}
}

func truncateMetadata(value string, max int) string {
	if len(value) <= max {
		return value
	}
	return value[:max]
}

func cookieValue(request *http.Request, name string) string {
	cookie, err := request.Cookie(name)
	if err != nil {
		return ""
	}
	return cookie.Value
}

func bearerToken(request *http.Request) string {
	value := strings.TrimSpace(request.Header.Get("Authorization"))
	if len(value) < len("Bearer ")+1 || !strings.EqualFold(value[:len("Bearer ")], "Bearer ") {
		return ""
	}
	return strings.TrimSpace(value[len("Bearer "):])
}

func (h authHandler) setSessionCookies(writer http.ResponseWriter, tokens identity.SessionTokens) {
	for _, cookie := range []struct {
		name    string
		value   string
		expires time.Time
		maxAge  int
	}{
		{name: accessCookieName, value: tokens.AccessToken, expires: tokens.AccessExpiresAt, maxAge: cookieMaxAge(tokens.AccessExpiresAt)},
		{name: refreshCookieName, value: tokens.RefreshToken, expires: tokens.RefreshExpiresAt, maxAge: cookieMaxAge(tokens.RefreshExpiresAt)},
	} {
		http.SetCookie(writer, &http.Cookie{
			Name:     cookie.name,
			Value:    cookie.value,
			Path:     h.cookiePath,
			Expires:  cookie.expires,
			MaxAge:   cookie.maxAge,
			HttpOnly: true,
			Secure:   h.cookieSecure,
			SameSite: http.SameSiteLaxMode,
		})
	}
}

func (h authHandler) clearSessionCookies(writer http.ResponseWriter) {
	for _, name := range []string{accessCookieName, refreshCookieName} {
		http.SetCookie(writer, &http.Cookie{
			Name:     name,
			Value:    "",
			Path:     h.cookiePath,
			MaxAge:   -1,
			Expires:  time.Unix(1, 0),
			HttpOnly: true,
			Secure:   h.cookieSecure,
			SameSite: http.SameSiteLaxMode,
		})
	}
}

func cookieMaxAge(expires time.Time) int {
	seconds := int(time.Until(expires).Seconds())
	if seconds < 1 {
		return 1
	}
	return seconds
}

func writeIdentityError(writer http.ResponseWriter, request *http.Request, err error) {
	var validationError *identity.ValidationError
	switch {
	case errors.As(err, &validationError):
		writeError(writer, request, http.StatusBadRequest, "invalid_request", validationError.Message)
	case errors.Is(err, identity.ErrBootstrapComplete):
		writeError(writer, request, http.StatusConflict, "bootstrap_complete", "Bootstrap administrator already exists")
	case errors.Is(err, identity.ErrInvalidCredentials):
		writeError(writer, request, http.StatusUnauthorized, "invalid_credentials", "Email or password is incorrect")
	case errors.Is(err, identity.ErrInvalidSession):
		writeError(writer, request, http.StatusUnauthorized, "invalid_session", "Session is invalid or expired")
	default:
		writeError(writer, request, http.StatusInternalServerError, "internal_error", "Internal server error")
	}
}
