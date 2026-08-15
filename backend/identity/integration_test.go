//go:build integration

package identity

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"opskeeper/backend/migrations"
)

const integrationPassword = "T03 acceptance password"

func TestIdentityLifecycle(t *testing.T) {
	pool := identityIntegrationPool(t)
	service := NewService(NewStore(pool), 15*time.Minute, 7*24*time.Hour)
	admin := bootstrapTestAdmin(t, service, "admin@example.com")

	if _, err := service.BootstrapAdmin(context.Background(), BootstrapInput{Email: "other@example.com", Password: integrationPassword}); !errors.Is(err, ErrBootstrapComplete) {
		t.Fatalf("second BootstrapAdmin() error = %v, want ErrBootstrapComplete", err)
	}
	var passwordHash string
	if err := pool.QueryRow(context.Background(), "SELECT password_hash FROM credentials WHERE user_id = $1::uuid", admin.ID).Scan(&passwordHash); err != nil {
		t.Fatalf("read password hash: %v", err)
	}
	if !strings.HasPrefix(passwordHash, "$argon2id$") || strings.Contains(passwordHash, integrationPassword) {
		t.Fatalf("stored password hash is not safe: %q", passwordHash)
	}

	if _, _, err := service.Login(context.Background(), admin.Email, "wrong password value", SessionMetadata{}); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("Login(wrong password) error = %v", err)
	}
	_, tokens, err := service.Login(context.Background(), strings.ToUpper(admin.Email), integrationPassword, SessionMetadata{UserAgent: "integration-test", ClientIP: "192.0.2.10"})
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	if _, err := service.Authenticate(context.Background(), tokens.AccessToken); err != nil {
		t.Fatalf("Authenticate() error = %v", err)
	}
	var storedAccessHash, storedRefreshHash []byte
	if err := pool.QueryRow(context.Background(), "SELECT access_token_hash, refresh_token_hash FROM sessions WHERE user_id = $1::uuid", admin.ID).Scan(&storedAccessHash, &storedRefreshHash); err != nil {
		t.Fatalf("read session hashes: %v", err)
	}
	if string(storedAccessHash) == tokens.AccessToken || string(storedRefreshHash) == tokens.RefreshToken {
		t.Fatal("database stored a plaintext session token")
	}

	_, rotated, err := service.Refresh(context.Background(), tokens.RefreshToken, SessionMetadata{UserAgent: "integration-test", ClientIP: "192.0.2.10"})
	if err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}
	if _, err := service.Authenticate(context.Background(), tokens.AccessToken); !errors.Is(err, ErrInvalidSession) {
		t.Fatalf("Authenticate(old access token) error = %v", err)
	}
	if _, _, err := service.Refresh(context.Background(), tokens.RefreshToken, SessionMetadata{}); !errors.Is(err, ErrInvalidSession) {
		t.Fatalf("Refresh(replayed token) error = %v", err)
	}
	if err := service.Logout(context.Background(), rotated.AccessToken, rotated.RefreshToken); err != nil {
		t.Fatalf("Logout() error = %v", err)
	}
	if _, err := service.Authenticate(context.Background(), rotated.AccessToken); !errors.Is(err, ErrInvalidSession) {
		t.Fatalf("Authenticate(logged out token) error = %v", err)
	}

	_, first, err := service.Login(context.Background(), admin.Email, integrationPassword, SessionMetadata{})
	if err != nil {
		t.Fatal(err)
	}
	_, second, err := service.Login(context.Background(), admin.Email, integrationPassword, SessionMetadata{})
	if err != nil {
		t.Fatal(err)
	}
	if err := service.LogoutAll(context.Background(), admin.ID); err != nil {
		t.Fatalf("LogoutAll() error = %v", err)
	}
	for _, token := range []string{first.AccessToken, second.AccessToken} {
		if _, err := service.Authenticate(context.Background(), token); !errors.Is(err, ErrInvalidSession) {
			t.Fatalf("Authenticate(after logout-all) error = %v", err)
		}
	}
}

func TestConcurrentBootstrapCreatesOneAdministrator(t *testing.T) {
	pool := identityIntegrationPool(t)
	service := NewService(NewStore(pool), 15*time.Minute, 7*24*time.Hour)
	start := make(chan struct{})
	results := make(chan error, 2)
	var wait sync.WaitGroup
	for index := range 2 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			<-start
			_, err := service.BootstrapAdmin(context.Background(), BootstrapInput{
				Email:       fmt.Sprintf("admin-%d@example.com", index),
				DisplayName: "Concurrent Admin",
				Password:    integrationPassword,
			})
			results <- err
		}()
	}
	close(start)
	wait.Wait()
	close(results)

	succeeded := 0
	rejected := 0
	for result := range results {
		switch {
		case result == nil:
			succeeded++
		case errors.Is(result, ErrBootstrapComplete):
			rejected++
		default:
			t.Fatalf("BootstrapAdmin() error = %v", result)
		}
	}
	if succeeded != 1 || rejected != 1 {
		t.Fatalf("concurrent bootstrap succeeded=%d rejected=%d", succeeded, rejected)
	}
	var users int
	if err := pool.QueryRow(context.Background(), "SELECT count(*) FROM users").Scan(&users); err != nil || users != 1 {
		t.Fatalf("user count = %d, error = %v", users, err)
	}
}

func TestDisabledAndLockedUsersCannotUseSessions(t *testing.T) {
	pool := identityIntegrationPool(t)
	service := NewService(NewStore(pool), 15*time.Minute, 7*24*time.Hour)
	admin := bootstrapTestAdmin(t, service, "status@example.com")
	_, tokens, err := service.Login(context.Background(), admin.Email, integrationPassword, SessionMetadata{})
	if err != nil {
		t.Fatal(err)
	}

	for _, status := range []string{StatusDisabled, StatusLocked} {
		if _, err := pool.Exec(context.Background(), "UPDATE users SET status = $2, updated_at = now() WHERE id = $1::uuid", admin.ID, status); err != nil {
			t.Fatalf("set user status: %v", err)
		}
		if _, err := service.Authenticate(context.Background(), tokens.AccessToken); !errors.Is(err, ErrInvalidSession) {
			t.Fatalf("Authenticate(%s user) error = %v", status, err)
		}
		if _, _, err := service.Refresh(context.Background(), tokens.RefreshToken, SessionMetadata{}); !errors.Is(err, ErrInvalidSession) {
			t.Fatalf("Refresh(%s user) error = %v", status, err)
		}
		if _, _, err := service.Login(context.Background(), admin.Email, integrationPassword, SessionMetadata{}); !errors.Is(err, ErrInvalidCredentials) {
			t.Fatalf("Login(%s user) error = %v", status, err)
		}
	}
}

func TestSessionExpiry(t *testing.T) {
	pool := identityIntegrationPool(t)
	service := NewService(NewStore(pool), time.Minute, 10*time.Minute)
	now := time.Now().UTC()
	service.now = func() time.Time { return now }
	admin := bootstrapTestAdmin(t, service, "expiry@example.com")
	_, tokens, err := service.Login(context.Background(), admin.Email, integrationPassword, SessionMetadata{})
	if err != nil {
		t.Fatal(err)
	}

	now = now.Add(2 * time.Minute)
	if _, err := service.Authenticate(context.Background(), tokens.AccessToken); !errors.Is(err, ErrInvalidSession) {
		t.Fatalf("Authenticate(expired access token) error = %v", err)
	}
	if _, _, err := service.Refresh(context.Background(), tokens.RefreshToken, SessionMetadata{}); err != nil {
		t.Fatalf("Refresh(valid refresh token) error = %v", err)
	}

	_, expiredTokens, err := service.Login(context.Background(), admin.Email, integrationPassword, SessionMetadata{})
	if err != nil {
		t.Fatal(err)
	}
	now = now.Add(11 * time.Minute)
	if _, _, err := service.Refresh(context.Background(), expiredTokens.RefreshToken, SessionMetadata{}); !errors.Is(err, ErrInvalidSession) {
		t.Fatalf("Refresh(expired token) error = %v", err)
	}
}

func TestConcurrentRefreshAllowsOneWinner(t *testing.T) {
	pool := identityIntegrationPool(t)
	service := NewService(NewStore(pool), 15*time.Minute, 7*24*time.Hour)
	admin := bootstrapTestAdmin(t, service, "refresh@example.com")
	_, tokens, err := service.Login(context.Background(), admin.Email, integrationPassword, SessionMetadata{})
	if err != nil {
		t.Fatal(err)
	}

	start := make(chan struct{})
	results := make(chan error, 2)
	var wait sync.WaitGroup
	for range 2 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			<-start
			_, _, refreshErr := service.Refresh(context.Background(), tokens.RefreshToken, SessionMetadata{})
			results <- refreshErr
		}()
	}
	close(start)
	wait.Wait()
	close(results)

	succeeded := 0
	rejected := 0
	for result := range results {
		switch {
		case result == nil:
			succeeded++
		case errors.Is(result, ErrInvalidSession):
			rejected++
		default:
			t.Fatalf("Refresh() error = %v", result)
		}
	}
	if succeeded != 1 || rejected != 1 {
		t.Fatalf("concurrent refresh succeeded=%d rejected=%d", succeeded, rejected)
	}
}

func bootstrapTestAdmin(t *testing.T, service *Service, email string) User {
	t.Helper()
	user, err := service.BootstrapAdmin(context.Background(), BootstrapInput{Email: email, DisplayName: "Test Admin", Password: integrationPassword})
	if err != nil {
		t.Fatalf("BootstrapAdmin() error = %v", err)
	}
	return user
}

func identityIntegrationPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	databaseURL := os.Getenv("OPSK_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("OPSK_TEST_DATABASE_URL is required")
	}

	ctx := context.Background()
	adminPool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect integration database: %v", err)
	}
	schema := fmt.Sprintf("identity_test_%d", time.Now().UnixNano())
	if _, err := adminPool.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		adminPool.Close()
		t.Fatalf("create integration schema: %v", err)
	}

	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		adminPool.Close()
		t.Fatalf("parse integration database config: %v", err)
	}
	config.ConnConfig.RuntimeParams["search_path"] = schema + ",public"
	config.MaxConns = 6
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		adminPool.Close()
		t.Fatalf("connect integration schema: %v", err)
	}
	if err := migrations.Apply(ctx, pool); err != nil {
		pool.Close()
		_, _ = adminPool.Exec(ctx, "DROP SCHEMA "+schema+" CASCADE")
		adminPool.Close()
		t.Fatalf("apply migrations: %v", err)
	}

	t.Cleanup(func() {
		pool.Close()
		_, _ = adminPool.Exec(context.Background(), "DROP SCHEMA "+schema+" CASCADE")
		adminPool.Close()
	})
	return pool
}
