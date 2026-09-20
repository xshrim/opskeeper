package persona

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgconn"
	"opskeeper/backend/authorization"
)

var (
	ErrNotFound = errors.New("Persona data not found")
	ErrConflict = errors.New("Persona data conflicts with existing data")
)

type ValidationError struct{ Message string }

func (e *ValidationError) Error() string { return e.Message }

func invalid(message string) error { return &ValidationError{Message: message} }

func allowsScope(ctx context.Context, scopeID string) bool {
	filter, ok := authorization.ScopeFilterFromContext(ctx)
	return !ok || filter.Allows(scopeID)
}

func mapStoreError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23503", "23514", "22P02":
			return invalid("Persona references invalid or unavailable data")
		case "23505":
			return ErrConflict
		}
	}
	return fmt.Errorf("store Persona data: %w", err)
}
