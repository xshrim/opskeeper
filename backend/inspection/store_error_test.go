package inspection

import (
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestMapErrorMapsConflictConstraints(t *testing.T) {
	for _, code := range []string{"23505", "23514"} {
		if err := mapError(&pgconn.PgError{Code: code}); !errors.Is(err, ErrConflict) {
			t.Errorf("mapError(%s) = %v, want conflict", code, err)
		}
	}
}
