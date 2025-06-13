package errors

import (
	"errors"
	"fmt"
)

// Wrap adds context to an error using Go's %w wrapping.
// If err is nil, returns nil.
func Wrap(err error, msg string) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", msg, err)
}

// Join combines multiple errors into one, skipping nils.
func Join(errs ...error) error {
	return errors.Join(errs...)
}
