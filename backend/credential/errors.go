package credential

import "errors"

var (
	ErrNotFound = errors.New("credential not found")
	ErrConflict = errors.New("credential conflicts with existing data")
)

type ValidationError struct{ Message string }

func (e *ValidationError) Error() string { return e.Message }

func invalid(message string) error { return &ValidationError{Message: message} }
