package httpapi

import (
	"bytes"
	"context"
	"errors"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"opskeeper/backend/identity"
)

const (
	accessCookieName  = "opsk_access"
	refreshCookieName = "opsk_refresh"
	maxAvatarBytes    = 1 << 20
)

type identityService interface {
	Login(context.Context, string, string, identity.SessionMetadata) (identity.User, identity.SessionTokens, error)
	Refresh(context.Context, string, identity.SessionMetadata) (identity.User, identity.SessionTokens, error)
	Authenticate(context.Context, string) (identity.User, error)
	Logout(context.Context, string, string) error
	LogoutAll(context.Context, string) error
	ChangePassword(context.Context, string, string, string) error
}

type authHandler struct {
	service       identityService
	platformAdmin platformAdminService
	cookiePath    string
	cookieSecure  bool
}

type platformAdminService interface {
	IsPlatformAdmin(context.Context, string) (bool, error)
}

type sessionContextResponse struct {
	PlatformAdmin bool `json:"platform_admin"`
}

type authRequest struct {
	Identifier string `json:"identifier"`
	Email      string `json:"email"`
	Password   string `json:"password"`
}

type updateProfileRequest struct {
	DisplayName *string `json:"display_name"`
	Email       *string `json:"email"`
	Phone       *string `json:"phone"`
}

type changePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

type profileService interface {
	UpdateProfile(context.Context, string, identity.UpdateUserInput) (identity.User, error)
}

type preferencesService interface {
	Preferences(context.Context, string) (identity.Preferences, error)
	UpdatePreferences(context.Context, string, identity.UpdatePreferencesInput) (identity.Preferences, error)
	UpdateAvatar(context.Context, string, string, []byte) (identity.Preferences, error)
	Avatar(context.Context, string) ([]byte, string, time.Time, error)
}

type updatePreferencesRequest struct {
	Theme            string `json:"theme"`
	SidebarMode      string `json:"sidebar_mode"`
	SidebarCollapsed bool   `json:"sidebar_collapsed"`
}

type authenticatedUserContextKey struct{}

func registerAuthRoutes(router chi.Router, service identityService, basePath string, cookieSecure bool, platformAdmin platformAdminService) {
	handler := authHandler{service: service, platformAdmin: platformAdmin, cookiePath: basePath, cookieSecure: cookieSecure}
	router.Route("/auth", func(router chi.Router) {
		router.Post("/login", handler.login)
		router.Post("/refresh", handler.refresh)
		router.Post("/logout", handler.logout)
		router.With(handler.requireAuth).Get("/me", handler.me)
		router.With(handler.requireAuth).Get("/me/context", handler.sessionContext)
		router.With(handler.requireAuth).Patch("/me", handler.updateProfile)
		router.With(handler.requireAuth).Post("/me/password", handler.changePassword)
		router.With(handler.requireAuth).Get("/me/preferences", handler.preferences)
		router.With(handler.requireAuth).Put("/me/preferences", handler.updatePreferences)
		router.With(handler.requireAuth).Get("/me/avatar", handler.avatar)
		router.With(handler.requireAuth).Put("/me/avatar", handler.updateAvatar)
		router.With(handler.requireAuth).Post("/logout-all", handler.logoutAll)
	})
}

func (h authHandler) login(writer http.ResponseWriter, request *http.Request) {
	var body authRequest
	if !decodeRequest(writer, request, &body) {
		return
	}
	identifier := strings.TrimSpace(body.Identifier)
	if identifier == "" {
		identifier = body.Email
	}
	user, tokens, err := h.service.Login(request.Context(), identifier, body.Password, sessionMetadata(request))
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

func (h authHandler) sessionContext(writer http.ResponseWriter, request *http.Request) {
	user, ok := request.Context().Value(authenticatedUserContextKey{}).(identity.User)
	if !ok {
		writeError(writer, request, http.StatusUnauthorized, "invalid_session", "Authentication is required")
		return
	}
	if h.platformAdmin == nil {
		writeJSON(writer, http.StatusOK, sessionContextResponse{})
		return
	}
	isPlatformAdmin, err := h.platformAdmin.IsPlatformAdmin(request.Context(), user.ID)
	if err != nil {
		writeError(writer, request, http.StatusInternalServerError, "internal_error", "Unable to load session context")
		return
	}
	writeJSON(writer, http.StatusOK, sessionContextResponse{PlatformAdmin: isPlatformAdmin})
}

func (h authHandler) updateProfile(writer http.ResponseWriter, request *http.Request) {
	user, ok := request.Context().Value(authenticatedUserContextKey{}).(identity.User)
	if !ok {
		writeError(writer, request, http.StatusUnauthorized, "invalid_session", "Authentication is required")
		return
	}
	updater, ok := h.service.(profileService)
	if !ok {
		writeError(writer, request, http.StatusNotImplemented, "profile_unavailable", "Profile updates are unavailable")
		return
	}
	var body updateProfileRequest
	if !decodeRequest(writer, request, &body) {
		return
	}
	updated, err := updater.UpdateProfile(request.Context(), user.ID, identity.UpdateUserInput{
		DisplayName: body.DisplayName,
		Email:       body.Email,
		Phone:       body.Phone,
	})
	if err != nil {
		writeIdentityError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, updated)
}

func (h authHandler) changePassword(writer http.ResponseWriter, request *http.Request) {
	user, ok := request.Context().Value(authenticatedUserContextKey{}).(identity.User)
	if !ok {
		writeError(writer, request, http.StatusUnauthorized, "invalid_session", "Authentication is required")
		return
	}
	var body changePasswordRequest
	if !decodeRequest(writer, request, &body) {
		return
	}
	if err := h.service.ChangePassword(request.Context(), user.ID, body.CurrentPassword, body.NewPassword); err != nil {
		writeIdentityError(writer, request, err)
		return
	}
	user.MustChangePassword = false
	writeJSON(writer, http.StatusOK, user)
}

func (h authHandler) preferences(writer http.ResponseWriter, request *http.Request) {
	user, ok := request.Context().Value(authenticatedUserContextKey{}).(identity.User)
	if !ok {
		writeError(writer, request, http.StatusUnauthorized, "invalid_session", "Authentication is required")
		return
	}
	service, ok := h.service.(preferencesService)
	if !ok {
		writeError(writer, request, http.StatusNotImplemented, "preferences_unavailable", "User preferences are unavailable")
		return
	}
	preferences, err := service.Preferences(request.Context(), user.ID)
	if err != nil {
		writeIdentityError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, preferences)
}

func (h authHandler) updatePreferences(writer http.ResponseWriter, request *http.Request) {
	user, ok := request.Context().Value(authenticatedUserContextKey{}).(identity.User)
	if !ok {
		writeError(writer, request, http.StatusUnauthorized, "invalid_session", "Authentication is required")
		return
	}
	service, ok := h.service.(preferencesService)
	if !ok {
		writeError(writer, request, http.StatusNotImplemented, "preferences_unavailable", "User preferences are unavailable")
		return
	}
	var body updatePreferencesRequest
	if !decodeRequest(writer, request, &body) {
		return
	}
	preferences, err := service.UpdatePreferences(request.Context(), user.ID, identity.UpdatePreferencesInput{
		Theme:            body.Theme,
		SidebarMode:      body.SidebarMode,
		SidebarCollapsed: body.SidebarCollapsed,
	})
	if err != nil {
		writeIdentityError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, preferences)
}

func (h authHandler) avatar(writer http.ResponseWriter, request *http.Request) {
	user, ok := request.Context().Value(authenticatedUserContextKey{}).(identity.User)
	if !ok {
		writeError(writer, request, http.StatusUnauthorized, "invalid_session", "Authentication is required")
		return
	}
	service, ok := h.service.(preferencesService)
	if !ok {
		writeError(writer, request, http.StatusNotImplemented, "preferences_unavailable", "User preferences are unavailable")
		return
	}
	content, contentType, updatedAt, err := service.Avatar(request.Context(), user.ID)
	if errors.Is(err, identity.ErrNotFound) {
		writeError(writer, request, http.StatusNotFound, "avatar_not_found", "Avatar is not configured")
		return
	}
	if err != nil {
		writeIdentityError(writer, request, err)
		return
	}
	writer.Header().Set("Content-Type", contentType)
	writer.Header().Set("Cache-Control", "private, max-age=300")
	writer.Header().Set("Last-Modified", updatedAt.UTC().Format(http.TimeFormat))
	_, _ = writer.Write(content)
}

func (h authHandler) updateAvatar(writer http.ResponseWriter, request *http.Request) {
	user, ok := request.Context().Value(authenticatedUserContextKey{}).(identity.User)
	if !ok {
		writeError(writer, request, http.StatusUnauthorized, "invalid_session", "Authentication is required")
		return
	}
	service, ok := h.service.(preferencesService)
	if !ok {
		writeError(writer, request, http.StatusNotImplemented, "preferences_unavailable", "User preferences are unavailable")
		return
	}
	request.Body = http.MaxBytesReader(writer, request.Body, maxAvatarBytes)
	content, err := io.ReadAll(request.Body)
	if err != nil {
		writeError(writer, request, http.StatusRequestEntityTooLarge, "avatar_too_large", "Avatar must be at most 1 MiB")
		return
	}
	configuration, format, err := image.DecodeConfig(bytes.NewReader(content))
	if err != nil || configuration.Width < 1 || configuration.Height < 1 || configuration.Width > 2048 || configuration.Height > 2048 {
		writeError(writer, request, http.StatusBadRequest, "invalid_avatar", "Avatar must be a PNG or JPEG image up to 2048 pixels per side")
		return
	}
	contentType := ""
	switch format {
	case "jpeg":
		contentType = "image/jpeg"
	case "png":
		contentType = "image/png"
	default:
		writeError(writer, request, http.StatusBadRequest, "invalid_avatar", "Avatar must be a PNG or JPEG image")
		return
	}
	preferences, err := service.UpdateAvatar(request.Context(), user.ID, contentType, content)
	if err != nil {
		writeIdentityError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, preferences)
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
		if user.MustChangePassword && !passwordChangeAllowed(request) {
			writeError(writer, request, http.StatusForbidden, "password_change_required", "You must change your one-time password before continuing")
			return
		}
		ctx := context.WithValue(request.Context(), authenticatedUserContextKey{}, user)
		next.ServeHTTP(writer, request.WithContext(ctx))
	})
}

func passwordChangeAllowed(request *http.Request) bool {
	path := strings.TrimSuffix(request.URL.Path, "/")
	return request.Method == http.MethodGet && strings.HasSuffix(path, "/api/v1/auth/me") ||
		request.Method == http.MethodPost && (strings.HasSuffix(path, "/api/v1/auth/me/password") || strings.HasSuffix(path, "/api/v1/auth/logout"))
}

func sessionMetadata(request *http.Request) identity.SessionMetadata {
	return identity.SessionMetadata{
		UserAgent: truncateMetadata(request.UserAgent(), 512),
		ClientIP:  truncateMetadata(requestClientIP(request), 255),
		RequestID: truncateMetadata(middleware.GetReqID(request.Context()), 128),
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
