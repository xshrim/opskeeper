package llm

import "errors"

var (
	ErrNotFound = errors.New("LLM configuration not found")
	ErrConflict = errors.New("LLM configuration conflicts with existing data")
)

type ValidationError struct{ Message string }

func (e *ValidationError) Error() string { return e.Message }

func invalid(message string) error { return &ValidationError{Message: message} }
