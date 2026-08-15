package authorization

import "errors"

var (
	ErrForbidden           = errors.New("authorization denied")
	ErrInvalidSubject      = errors.New("invalid authorization subject")
	ErrBootstrapNotAllowed = errors.New("bootstrap administrator binding is not allowed")
)
