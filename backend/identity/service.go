package identity

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"opskeeper/backend/audit"
)

const (
	minimumPasswordLength = 12
	minimumEmailLength    = 3
	tokenByteLength       = 32
)

type Store interface {
	BootstrapAdmin(context.Context, string, string, string) (User, error)
	FindByEmail(context.Context, string) (User, string, error)
	UpdatePasswordHash(context.Context, string, string, string) error
	CreateSession(context.Context, string, []byte, []byte, time.Time, time.Time, SessionMetadata) error
	RotateSession(context.Context, []byte, []byte, []byte, time.Time, time.Time, time.Time, time.Time, SessionMetadata) (User, error)
	Authenticate(context.Context, []byte, time.Time) (User, error)
	RevokeSession(context.Context, []byte, []byte, time.Time) error
	RevokeAllSessions(context.Context, string, time.Time) error
}

type UserManagementStore interface {
	CreateUser(context.Context, string, string, string) (User, error)
	ListUsers(context.Context) ([]User, error)
	GetUser(context.Context, string) (User, error)
	UpdateUser(context.Context, string, UpdateUserInput) (User, error)
}

type Service struct {
	store      Store
	accessTTL  time.Duration
	refreshTTL time.Duration
	now        func() time.Time
	auditor    audit.Logger
}

func NewService(store Store, accessTTL, refreshTTL time.Duration, auditors ...audit.Logger) *Service {
	var auditor audit.Logger
	if len(auditors) > 0 {
		auditor = auditors[0]
	}
	return &Service{store: store, accessTTL: accessTTL, refreshTTL: refreshTTL, now: time.Now, auditor: auditor}
}

func (s *Service) BootstrapAdmin(ctx context.Context, input BootstrapInput) (User, error) {
	input.Email = normalizeEmail(input.Email)
	input.DisplayName = strings.TrimSpace(input.DisplayName)
	if err := validateEmail(input.Email); err != nil {
		return User{}, err
	}
	if input.DisplayName == "" {
		input.DisplayName = input.Email
	}
	if len([]rune(input.DisplayName)) > 120 {
		return User{}, invalid("display_name must be at most 120 characters")
	}
	if len([]rune(input.Password)) < minimumPasswordLength {
		return User{}, invalid(fmt.Sprintf("password must be at least %d characters", minimumPasswordLength))
	}
	hash, err := hashPassword(input.Password)
	if err != nil {
		return User{}, err
	}
	return s.store.BootstrapAdmin(ctx, input.Email, input.DisplayName, hash)
}

func (s *Service) Login(ctx context.Context, email, password string, metadata SessionMetadata) (User, SessionTokens, error) {
	email = normalizeEmail(email)
	if err := validateEmail(email); err != nil || password == "" {
		_ = s.recordAudit(ctx, audit.Event{Action: "auth.login", Result: "failure", RequestID: metadata.RequestID, ClientIP: metadata.ClientIP, Details: map[string]any{"reason": "invalid_credentials"}})
		return User{}, SessionTokens{}, ErrInvalidCredentials
	}
	user, hash, err := s.store.FindByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, ErrInvalidCredentials) {
			_ = s.recordAudit(ctx, audit.Event{Action: "auth.login", Result: "failure", RequestID: metadata.RequestID, ClientIP: metadata.ClientIP, Details: map[string]any{"reason": "invalid_credentials"}})
			return User{}, SessionTokens{}, ErrInvalidCredentials
		}
		return User{}, SessionTokens{}, err
	}
	if user.Status != StatusActive || !verifyPassword(hash, password) {
		_ = s.recordAudit(ctx, audit.Event{ActorUserID: user.ID, Action: "auth.login", Result: "failure", RequestID: metadata.RequestID, ClientIP: metadata.ClientIP, Details: map[string]any{"reason": "invalid_credentials"}})
		return User{}, SessionTokens{}, ErrInvalidCredentials
	}
	if passwordNeedsUpgrade(hash) {
		upgraded, hashErr := hashPassword(password)
		if hashErr != nil {
			return User{}, SessionTokens{}, hashErr
		}
		if hashErr = s.store.UpdatePasswordHash(ctx, user.ID, hash, upgraded); hashErr != nil {
			return User{}, SessionTokens{}, hashErr
		}
	}
	resultUser, tokens, err := s.issueSession(ctx, user, metadata)
	if err != nil {
		return User{}, SessionTokens{}, err
	}
	if err := s.recordAudit(ctx, audit.Event{ActorUserID: user.ID, Action: "auth.login", Result: "success", RequestID: metadata.RequestID, ClientIP: metadata.ClientIP, Details: map[string]any{"user_agent": metadata.UserAgent}}); err != nil {
		return User{}, SessionTokens{}, err
	}
	return resultUser, tokens, nil
}

func (s *Service) Refresh(ctx context.Context, refreshToken string, metadata SessionMetadata) (User, SessionTokens, error) {
	if refreshToken == "" {
		_ = s.recordAudit(ctx, audit.Event{Action: "auth.refresh", Result: "failure", RequestID: metadata.RequestID, ClientIP: metadata.ClientIP, Details: map[string]any{"reason": "invalid_session"}})
		return User{}, SessionTokens{}, ErrInvalidSession
	}
	now := s.now()
	accessToken, err := newToken()
	if err != nil {
		return User{}, SessionTokens{}, err
	}
	newRefreshToken, err := newToken()
	if err != nil {
		return User{}, SessionTokens{}, err
	}
	accessExpiresAt := now.Add(s.accessTTL)
	refreshExpiresAt := now.Add(s.refreshTTL)
	user, err := s.store.RotateSession(ctx, digest(refreshToken), digest(accessToken), digest(newRefreshToken), now, accessExpiresAt, refreshExpiresAt, now, metadata)
	if err != nil {
		if errors.Is(err, ErrInvalidSession) {
			_ = s.recordAudit(ctx, audit.Event{Action: "auth.refresh", Result: "failure", RequestID: metadata.RequestID, ClientIP: metadata.ClientIP, Details: map[string]any{"reason": "invalid_session"}})
			return User{}, SessionTokens{}, ErrInvalidSession
		}
		return User{}, SessionTokens{}, err
	}
	tokens := SessionTokens{AccessToken: accessToken, RefreshToken: newRefreshToken, AccessExpiresAt: accessExpiresAt, RefreshExpiresAt: refreshExpiresAt}
	if err := s.recordAudit(ctx, audit.Event{ActorUserID: user.ID, Action: "auth.refresh", Result: "success", RequestID: metadata.RequestID, ClientIP: metadata.ClientIP, Details: map[string]any{}}); err != nil {
		return User{}, SessionTokens{}, err
	}
	return user, tokens, nil
}

func (s *Service) Authenticate(ctx context.Context, accessToken string) (User, error) {
	if accessToken == "" {
		return User{}, ErrInvalidSession
	}
	user, err := s.store.Authenticate(ctx, digest(accessToken), s.now())
	if err != nil {
		if errors.Is(err, ErrInvalidSession) {
			return User{}, ErrInvalidSession
		}
		return User{}, err
	}
	return user, nil
}

func (s *Service) Logout(ctx context.Context, accessToken, refreshToken string) error {
	if accessToken == "" && refreshToken == "" {
		return nil
	}
	return s.store.RevokeSession(ctx, digest(accessToken), digest(refreshToken), s.now())
}

func (s *Service) LogoutAll(ctx context.Context, userID string) error {
	if err := s.store.RevokeAllSessions(ctx, userID, s.now()); err != nil {
		return err
	}
	return s.recordAudit(ctx, audit.Event{ActorUserID: userID, Action: "auth.logout_all", Result: "success", Details: map[string]any{}})
}

func (s *Service) recordAudit(ctx context.Context, event audit.Event) error {
	if s.auditor == nil {
		return nil
	}
	return s.auditor.Record(ctx, event)
}

func (s *Service) ListUsers(ctx context.Context) ([]User, error) {
	store, ok := s.store.(UserManagementStore)
	if !ok {
		return nil, errors.New("user management is unavailable")
	}
	return store.ListUsers(ctx)
}

func (s *Service) CreateUser(ctx context.Context, input CreateUserInput) (User, error) {
	store, ok := s.store.(UserManagementStore)
	if !ok {
		return User{}, errors.New("user management is unavailable")
	}
	input.Email = normalizeEmail(input.Email)
	input.DisplayName = strings.TrimSpace(input.DisplayName)
	if err := validateEmail(input.Email); err != nil {
		return User{}, err
	}
	if input.DisplayName == "" {
		input.DisplayName = input.Email
	}
	if len([]rune(input.DisplayName)) > 120 {
		return User{}, invalid("display_name must be at most 120 characters")
	}
	if len([]rune(input.Password)) < minimumPasswordLength {
		return User{}, invalid(fmt.Sprintf("password must be at least %d characters", minimumPasswordLength))
	}
	hash, err := hashPassword(input.Password)
	if err != nil {
		return User{}, err
	}
	return store.CreateUser(ctx, input.Email, input.DisplayName, hash)
}

func (s *Service) GetUser(ctx context.Context, userID string) (User, error) {
	store, ok := s.store.(UserManagementStore)
	if !ok {
		return User{}, errors.New("user management is unavailable")
	}
	return store.GetUser(ctx, userID)
}

func (s *Service) UpdateUser(ctx context.Context, userID string, input UpdateUserInput) (User, error) {
	if input.DisplayName != nil {
		name := strings.TrimSpace(*input.DisplayName)
		if name == "" || len([]rune(name)) > 120 {
			return User{}, invalid("display_name must contain 1 to 120 characters")
		}
		input.DisplayName = &name
	}
	if input.Status != nil && *input.Status != StatusActive && *input.Status != StatusDisabled && *input.Status != StatusLocked {
		return User{}, invalid("status must be active, disabled, or locked")
	}
	store, ok := s.store.(UserManagementStore)
	if !ok {
		return User{}, errors.New("user management is unavailable")
	}
	return store.UpdateUser(ctx, userID, input)
}

func (s *Service) issueSession(ctx context.Context, user User, metadata SessionMetadata) (User, SessionTokens, error) {
	now := s.now()
	accessToken, err := newToken()
	if err != nil {
		return User{}, SessionTokens{}, err
	}
	refreshToken, err := newToken()
	if err != nil {
		return User{}, SessionTokens{}, err
	}
	accessExpiresAt := now.Add(s.accessTTL)
	refreshExpiresAt := now.Add(s.refreshTTL)
	if err := s.store.CreateSession(ctx, user.ID, digest(accessToken), digest(refreshToken), accessExpiresAt, refreshExpiresAt, metadata); err != nil {
		return User{}, SessionTokens{}, err
	}
	return user, SessionTokens{AccessToken: accessToken, RefreshToken: refreshToken, AccessExpiresAt: accessExpiresAt, RefreshExpiresAt: refreshExpiresAt}, nil
}

func newToken() (string, error) {
	bytes := make([]byte, tokenByteLength)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("generate session token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(bytes), nil
}

func digest(token string) []byte {
	sum := sha256.Sum256([]byte(token))
	return sum[:]
}

func normalizeEmail(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func validateEmail(value string) error {
	if len(value) < minimumEmailLength || len(value) > 254 || strings.Count(value, "@") != 1 {
		return invalid("email must be a valid address")
	}
	parts := strings.Split(value, "@")
	if parts[0] == "" || parts[1] == "" || strings.ContainsAny(value, " \t\r\n") || !strings.Contains(parts[1], ".") {
		return invalid("email must be a valid address")
	}
	return nil
}
