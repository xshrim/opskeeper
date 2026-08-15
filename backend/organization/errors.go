package organization

import "errors"

var (
	ErrNotFound       = errors.New("organization not found")
	ErrConflict       = errors.New("organization conflicts with existing data")
	ErrParentInactive = errors.New("parent organization is inactive")
)

type ValidationError struct {
	Message string
}

func (e *ValidationError) Error() string {
	return e.Message
}

func invalid(message string) error {
	return &ValidationError{Message: message}
}
