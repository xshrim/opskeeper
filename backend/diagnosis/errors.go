package diagnosis

import "errors"

var (
	ErrNotFound = errors.New("diagnosis data not found")
	ErrConflict = errors.New("diagnosis data conflicts with existing data")
)

type invalidError struct{ message string }

func (e invalidError) Error() string { return e.message }

func invalid(message string) error { return invalidError{message: message} }

func IsInvalid(err error) bool {
	var target invalidError
	return errors.As(err, &target)
}
