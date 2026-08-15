package identity

import "errors"

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrInvalidSession     = errors.New("invalid session")
	ErrBootstrapComplete  = errors.New("bootstrap administrator already exists")
	ErrUserInactive       = errors.New("user is inactive")
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
