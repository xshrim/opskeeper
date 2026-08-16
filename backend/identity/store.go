package identity

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

const bootstrapAdvisoryLockID int64 = 0x4f50534b42535452

type store struct {
	pool *pgxpool.Pool
}

var _ Store = (*store)(nil)

func NewStore(pool *pgxpool.Pool) Store {
	return &store{pool: pool}
}

func (s *store) BootstrapAdmin(ctx context.Context, email, displayName, passwordHash string) (User, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return User{}, fmt.Errorf("begin bootstrap administrator: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock($1)", bootstrapAdvisoryLockID); err != nil {
		return User{}, fmt.Errorf("lock bootstrap administrator: %w", err)
	}
	var userCount int
	if err := tx.QueryRow(ctx, "SELECT count(*) FROM users WHERE deleted_at IS NULL").Scan(&userCount); err != nil {
		return User{}, fmt.Errorf("count users: %w", err)
	}
	if userCount != 0 {
		return User{}, ErrBootstrapComplete
	}

	user, err := insertUser(ctx, tx, email, displayName)
	if err != nil {
		return User{}, mapBootstrapStoreError(err)
	}
	if _, err := tx.Exec(ctx, "INSERT INTO credentials (user_id, password_hash) VALUES ($1::uuid, $2)", user.ID, passwordHash); err != nil {
		return User{}, mapBootstrapStoreError(err)
	}
	if err := tx.Commit(ctx); err != nil {
		return User{}, fmt.Errorf("commit bootstrap administrator: %w", err)
	}
	return user, nil
}

func (s *store) FindByEmail(ctx context.Context, email string) (User, string, error) {
	var user User
	var passwordHash string
	err := s.pool.QueryRow(ctx, `
		SELECT u.id::text, u.email, u.display_name, u.status, u.created_at, u.updated_at, c.password_hash
		  FROM users u
		  JOIN credentials c ON c.user_id = u.id
		 WHERE u.deleted_at IS NULL AND lower(u.email) = lower($1)`, email).
		Scan(&user.ID, &user.Email, &user.DisplayName, &user.Status, &user.CreatedAt, &user.UpdatedAt, &passwordHash)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return User{}, "", ErrInvalidCredentials
		}
		return User{}, "", fmt.Errorf("find user by email: %w", err)
	}
	return user, passwordHash, nil
}

func (s *store) UpdatePasswordHash(ctx context.Context, userID, previousHash, newHash string) error {
	command, err := s.pool.Exec(ctx, `
		UPDATE credentials
		   SET password_hash = $3, password_changed_at = now(), updated_at = now()
		 WHERE user_id = $1::uuid AND password_hash = $2`, userID, previousHash, newHash)
	if err != nil {
		return fmt.Errorf("upgrade password hash: %w", err)
	}
	if command.RowsAffected() != 1 {
		return errors.New("upgrade password hash: credential changed concurrently")
	}
	return nil
}

func (s *store) CreateSession(ctx context.Context, userID string, accessHash, refreshHash []byte, accessExpiresAt, refreshExpiresAt time.Time, metadata SessionMetadata) error {
	command, err := s.pool.Exec(ctx, `
		INSERT INTO sessions (user_id, access_token_hash, refresh_token_hash, access_expires_at, refresh_expires_at, user_agent, client_ip)
		SELECT $1::uuid, $2, $3, $4, $5, $6, $7
		 WHERE EXISTS (SELECT 1 FROM users WHERE id = $1::uuid AND status = 'active' AND deleted_at IS NULL)`,
		userID, accessHash, refreshHash, accessExpiresAt, refreshExpiresAt, metadata.UserAgent, metadata.ClientIP)
	if err != nil {
		return mapStoreError(err)
	}
	if command.RowsAffected() != 1 {
		return ErrUserInactive
	}
	return nil
}

func (s *store) RotateSession(ctx context.Context, oldRefreshHash, accessHash, refreshHash []byte, now, accessExpiresAt, refreshExpiresAt, lastSeenAt time.Time, metadata SessionMetadata) (User, error) {
	var user User
	err := s.pool.QueryRow(ctx, `
		UPDATE sessions AS session
		   SET access_token_hash = $2,
		       refresh_token_hash = $3,
		       access_expires_at = $5,
		       refresh_expires_at = $6,
		       last_seen_at = $7,
		       user_agent = $8,
		       client_ip = $9
		  FROM users AS usr
		 WHERE session.user_id = usr.id
		   AND session.refresh_token_hash = $1
		   AND session.revoked_at IS NULL
		   AND session.refresh_expires_at > $4
		   AND usr.status = 'active'
		   AND usr.deleted_at IS NULL
		RETURNING usr.id::text, usr.email, usr.display_name, usr.status, usr.created_at, usr.updated_at`,
		oldRefreshHash, accessHash, refreshHash, now, accessExpiresAt, refreshExpiresAt, lastSeenAt, metadata.UserAgent, metadata.ClientIP).
		Scan(&user.ID, &user.Email, &user.DisplayName, &user.Status, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return User{}, ErrInvalidSession
		}
		return User{}, fmt.Errorf("rotate session: %w", err)
	}
	return user, nil
}

func (s *store) Authenticate(ctx context.Context, accessHash []byte, now time.Time) (User, error) {
	var user User
	err := s.pool.QueryRow(ctx, `
		UPDATE sessions AS session
		   SET last_seen_at = $2
		  FROM users AS usr
		 WHERE session.user_id = usr.id
		   AND session.access_token_hash = $1
		   AND session.revoked_at IS NULL
		   AND session.access_expires_at > $2
		   AND usr.status = 'active'
		   AND usr.deleted_at IS NULL
		RETURNING usr.id::text, usr.email, usr.display_name, usr.status, usr.created_at, usr.updated_at`,
		accessHash, now).
		Scan(&user.ID, &user.Email, &user.DisplayName, &user.Status, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return User{}, ErrInvalidSession
		}
		return User{}, fmt.Errorf("authenticate session: %w", err)
	}
	return user, nil
}

func (s *store) RevokeSession(ctx context.Context, accessHash, refreshHash []byte, now time.Time) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE sessions
		   SET revoked_at = COALESCE(revoked_at, $3), last_seen_at = $3
		 WHERE revoked_at IS NULL AND (access_token_hash = $1 OR refresh_token_hash = $2)`, accessHash, refreshHash, now)
	if err != nil {
		return fmt.Errorf("revoke session: %w", err)
	}
	return nil
}

func (s *store) RevokeAllSessions(ctx context.Context, userID string, now time.Time) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE sessions
		   SET revoked_at = COALESCE(revoked_at, $2), last_seen_at = $2
		 WHERE user_id = $1::uuid AND revoked_at IS NULL`, userID, now)
	if err != nil {
		return fmt.Errorf("revoke all sessions: %w", err)
	}
	return nil
}

func (s *store) ListUsers(ctx context.Context) ([]User, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id::text, email, display_name, status, created_at, updated_at
		  FROM users WHERE deleted_at IS NULL ORDER BY created_at, id`)
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	defer rows.Close()
	users := make([]User, 0)
	for rows.Next() {
		var user User
		if err := rows.Scan(&user.ID, &user.Email, &user.DisplayName, &user.Status, &user.CreatedAt, &user.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan user: %w", err)
		}
		users = append(users, user)
	}
	return users, rows.Err()
}

func (s *store) CreateUser(ctx context.Context, email, displayName, passwordHash string) (User, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return User{}, fmt.Errorf("begin create user: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	user, err := insertUser(ctx, tx, email, displayName)
	if err != nil {
		return User{}, mapStoreError(err)
	}
	if _, err := tx.Exec(ctx, "INSERT INTO credentials (user_id, password_hash) VALUES ($1::uuid, $2)", user.ID, passwordHash); err != nil {
		return User{}, mapStoreError(err)
	}
	if err := tx.Commit(ctx); err != nil {
		return User{}, fmt.Errorf("commit create user: %w", err)
	}
	return user, nil
}

func (s *store) GetUser(ctx context.Context, userID string) (User, error) {
	var user User
	err := s.pool.QueryRow(ctx, `
		SELECT id::text, email, display_name, status, created_at, updated_at
		  FROM users WHERE id = $1::uuid AND deleted_at IS NULL`, userID).
		Scan(&user.ID, &user.Email, &user.DisplayName, &user.Status, &user.CreatedAt, &user.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrNotFound
	}
	if err != nil {
		return User{}, fmt.Errorf("get user: %w", err)
	}
	return user, nil
}

func (s *store) UpdateUser(ctx context.Context, userID string, input UpdateUserInput) (User, error) {
	current, err := s.GetUser(ctx, userID)
	if err != nil {
		return User{}, err
	}
	if input.DisplayName != nil {
		current.DisplayName = *input.DisplayName
	}
	if input.Status != nil {
		current.Status = *input.Status
	}
	var user User
	err = s.pool.QueryRow(ctx, `
		UPDATE users SET display_name = $2, status = $3, updated_at = now()
		 WHERE id = $1::uuid AND deleted_at IS NULL
		RETURNING id::text, email, display_name, status, created_at, updated_at`,
		userID, current.DisplayName, current.Status).
		Scan(&user.ID, &user.Email, &user.DisplayName, &user.Status, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		return User{}, fmt.Errorf("update user: %w", err)
	}
	return user, nil
}

type queryer interface {
	QueryRow(context.Context, string, ...any) pgx.Row
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
}

func insertUser(ctx context.Context, query queryer, email, displayName string) (User, error) {
	var user User
	err := query.QueryRow(ctx, `
		INSERT INTO users (email, display_name)
		VALUES ($1, $2)
		RETURNING id::text, email, display_name, status, created_at, updated_at`, email, displayName).
		Scan(&user.ID, &user.Email, &user.DisplayName, &user.Status, &user.CreatedAt, &user.UpdatedAt)
	return user, err
}

func mapStoreError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return ErrConflict
	}
	return err
}

func mapBootstrapStoreError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return ErrBootstrapComplete
	}
	return err
}
