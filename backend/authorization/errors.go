package authorization

import "errors"

var (
	ErrForbidden           = errors.New("authorization denied")
	ErrInvalidSubject      = errors.New("invalid authorization subject")
	ErrBootstrapNotAllowed = errors.New("bootstrap administrator binding is not allowed")
	ErrNotFound            = errors.New("authorization object not found")
	ErrConflict            = errors.New("authorization object conflicts with existing data")
	ErrInvalidRole         = errors.New("invalid role")
	ErrGrantNotAllowed     = errors.New("role grant is outside the actor authority")
	ErrInvalidInput        = errors.New("invalid authorization input")
)
