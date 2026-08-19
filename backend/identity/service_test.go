package identity

import (
	"context"
	"errors"
	"testing"
	"time"
)

type stubStore struct {
	user                User
	passwordHash        string
	findError           error
	createSessionError  error
	rotateError         error
	updatedPasswordHash string
	createdAccessHash   []byte
	createdRefreshHash  []byte
	bootstrapInput      BootstrapInput
	changedCurrent      string
	changedHash         string
}

func (s *stubStore) BootstrapAdmin(_ context.Context, username, email, phone, displayName, passwordHash string) (User, error) {
	s.bootstrapInput = BootstrapInput{Username: username, Email: email, Phone: phone, DisplayName: displayName, Password: passwordHash}
	return s.user, nil
}

func (s *stubStore) FindByIdentifier(context.Context, string, string) (User, string, error) {
	return s.user, s.passwordHash, s.findError
}

func (s *stubStore) UpdatePasswordHash(_ context.Context, _ string, _ string, newHash string) error {
	s.updatedPasswordHash = newHash
	return nil
}

func (s *stubStore) CreateSession(_ context.Context, _ string, accessHash, refreshHash []byte, _ time.Time, _ time.Time, _ SessionMetadata) error {
	s.createdAccessHash = append([]byte(nil), accessHash...)
	s.createdRefreshHash = append([]byte(nil), refreshHash...)
	return s.createSessionError
}

func (s *stubStore) RotateSession(context.Context, []byte, []byte, []byte, time.Time, time.Time, time.Time, time.Time, SessionMetadata) (User, error) {
	return s.user, s.rotateError
}

func (s *stubStore) Authenticate(context.Context, []byte, time.Time) (User, error) {
	return s.user, nil
}

func (s *stubStore) RevokeSession(context.Context, []byte, []byte, time.Time) error {
	return nil
}

func (s *stubStore) RevokeAllSessions(context.Context, string, time.Time) error {
	return nil
}

func (s *stubStore) ChangePassword(_ context.Context, _ string, currentPassword, newHash string) error {
	s.changedCurrent = currentPassword
	s.changedHash = newHash
	return nil
}

func (s *stubStore) CreateUser(_ context.Context, username, email, phone, displayName, _ string) (User, error) {
	s.user.Username = username
	s.user.Email = email
	s.user.Phone = phone
	s.user.DisplayName = displayName
	return s.user, nil
}

func (s *stubStore) ListUsers(context.Context) ([]User, error) { return nil, nil }

func (s *stubStore) GetUser(context.Context, string) (User, error) { return s.user, nil }

func (s *stubStore) UpdateUser(context.Context, string, UpdateUserInput) (User, error) {
	return s.user, nil
}

func TestBootstrapAdminNormalizesInputAndHashesPassword(t *testing.T) {
	store := &stubStore{user: User{ID: "user-1"}}
	service := NewService(store, 15*time.Minute, 24*time.Hour)

	_, err := service.BootstrapAdmin(context.Background(), BootstrapInput{
		Username:    " Platform.Admin ",
		Email:       "  ADMIN@Example.COM ",
		DisplayName: "  Platform Admin  ",
		Password:    "strong password value",
	})
	if err != nil {
		t.Fatalf("BootstrapAdmin() error = %v", err)
	}
	if store.bootstrapInput.Username != "platform.admin" || store.bootstrapInput.Email != "admin@example.com" || store.bootstrapInput.DisplayName != "Platform Admin" {
		t.Fatalf("BootstrapAdmin() input = %#v", store.bootstrapInput)
	}
	if store.bootstrapInput.Password == "strong password value" || !verifyPassword(store.bootstrapInput.Password, "strong password value") {
		t.Fatal("BootstrapAdmin() did not store an Argon2id password hash")
	}
}

func TestBootstrapAdminRejectsWeakPassword(t *testing.T) {
	service := NewService(&stubStore{}, 15*time.Minute, 24*time.Hour)
	_, err := service.BootstrapAdmin(context.Background(), BootstrapInput{Username: "admin", Password: "short"})
	var validationError *ValidationError
	if !errors.As(err, &validationError) {
		t.Fatalf("BootstrapAdmin() error = %v, want ValidationError", err)
	}
}

func TestLoginIssuesOpaqueTokens(t *testing.T) {
	hash, err := hashPassword("strong password value")
	if err != nil {
		t.Fatal(err)
	}
	store := &stubStore{user: User{ID: "user-1", Status: StatusActive}, passwordHash: hash}
	service := NewService(store, 15*time.Minute, 24*time.Hour)

	_, tokens, err := service.Login(context.Background(), "admin@example.com", "strong password value", SessionMetadata{})
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	if tokens.AccessToken == "" || tokens.RefreshToken == "" || tokens.AccessToken == tokens.RefreshToken {
		t.Fatalf("Login() tokens = %#v", tokens)
	}
	if string(store.createdAccessHash) == tokens.AccessToken || string(store.createdRefreshHash) == tokens.RefreshToken {
		t.Fatal("Store received plaintext session token")
	}
}

func TestLoginAcceptsUsername(t *testing.T) {
	hash, err := hashPassword("strong password value")
	if err != nil {
		t.Fatal(err)
	}
	store := &stubStore{user: User{ID: "user-1", Username: "admin", Email: "admin@example.com", Status: StatusActive}, passwordHash: hash}
	service := NewService(store, 15*time.Minute, 24*time.Hour)

	if _, _, err := service.Login(context.Background(), "admin", "strong password value", SessionMetadata{}); err != nil {
		t.Fatalf("Login(username) error = %v", err)
	}
}

func TestLoginRejectsInactiveUser(t *testing.T) {
	hash, err := hashPassword("strong password value")
	if err != nil {
		t.Fatal(err)
	}
	service := NewService(&stubStore{user: User{Status: StatusLocked}, passwordHash: hash}, 15*time.Minute, 24*time.Hour)
	_, _, err = service.Login(context.Background(), "admin@example.com", "strong password value", SessionMetadata{})
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("Login() error = %v, want ErrInvalidCredentials", err)
	}
}

func TestCreateUserWithOneTimePasswordGeneratesPassword(t *testing.T) {
	store := &stubStore{user: User{ID: "user-1", Status: StatusActive}}
	service := NewService(store, 15*time.Minute, 24*time.Hour)
	result, err := service.CreateUserWithOneTimePassword(context.Background(), CreateUserInput{Username: "operator", DisplayName: "Operator"})
	if err != nil {
		t.Fatalf("CreateUserWithOneTimePassword() error = %v", err)
	}
	if !result.User.MustChangePassword || len(result.OneTimePassword) < minimumPasswordLength {
		t.Fatalf("CreateUserWithOneTimePassword() = %#v", result)
	}
}

func TestChangePasswordValidatesAndHashesNewPassword(t *testing.T) {
	store := &stubStore{}
	service := NewService(store, 15*time.Minute, 24*time.Hour)
	if err := service.ChangePassword(context.Background(), "user-1", "old password", "new strong password"); err != nil {
		t.Fatalf("ChangePassword() error = %v", err)
	}
	if store.changedCurrent != "old password" || !verifyPassword(store.changedHash, "new strong password") {
		t.Fatalf("ChangePassword() store values = current=%q hash=%q", store.changedCurrent, store.changedHash)
	}
}

func TestRefreshPreservesInternalError(t *testing.T) {
	want := errors.New("database unavailable")
	service := NewService(&stubStore{rotateError: want}, 15*time.Minute, 24*time.Hour)
	_, _, err := service.Refresh(context.Background(), "refresh-token", SessionMetadata{})
	if !errors.Is(err, want) {
		t.Fatalf("Refresh() error = %v, want internal error", err)
	}
}
