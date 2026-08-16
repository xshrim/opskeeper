package resource

import "errors"

var (
	ErrNotFound       = errors.New("resource not found")
	ErrConflict       = errors.New("resource conflicts with existing data")
	ErrSchemaNotFound = errors.New("resource schema not found")
	ErrRelationCycle  = errors.New("resource relation would create a cycle")
)

type ValidationError struct{ Message string }

func (e *ValidationError) Error() string { return e.Message }

func invalid(message string) error { return &ValidationError{Message: message} }
