/*
Package errors provides error helpers and sentinel errors for the toolkit application.
*/
package errors

import (
	"errors"
	"fmt"
)

/*
ErrUnknown is a generic sentinel error for unknown error cases.
Document: use errors.Is(err, ErrUnknown) for comparison.
*/
var ErrUnknown = fmt.Errorf("unknown error")

// Unwrap returns the wrapped error if available (for custom error types).
func Unwrap(err error) error {
	type unwrapper interface{ Unwrap() error }
	if u, ok := err.(unwrapper); ok {
		return u.Unwrap()
	}
	return nil
}

// Wrap returns an error with consistent phrasing: "action: %w"
func Wrap(action string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", action, err)
}

// Join combines multiple errors into one (Go 1.20+).
func Join(errs ...error) error {
	return errors.Join(errs...)
}
