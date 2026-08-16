package skill

import "errors"

var (
	ErrNotFound = errors.New("Skill data not found")
	ErrConflict = errors.New("Skill data conflicts with existing data")
	ErrBudget   = errors.New("Skill execution budget exceeded")
)

type ValidationError struct{ Message string }

func (e *ValidationError) Error() string { return e.Message }

func invalid(message string) error { return &ValidationError{Message: message} }
