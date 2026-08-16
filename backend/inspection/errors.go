package inspection

import "errors"

var (
	ErrNotFound  = errors.New("inspection item not found")
	ErrConflict  = errors.New("inspection conflict")
	ErrForbidden = errors.New("inspection forbidden")
)

type ValidationError struct{ Message string }

func (e *ValidationError) Error() string { return e.Message }

func invalid(message string) error { return &ValidationError{Message: message} }

func IsInvalid(err error) bool { var target *ValidationError; return errors.As(err, &target) }
