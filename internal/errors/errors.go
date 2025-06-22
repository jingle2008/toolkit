package errors

import (
	"fmt"
)

// Sentinel error example
var ErrUnknown = fmt.Errorf("unknown error")

// Wrap returns an error with consistent phrasing: "action: %w"
func Wrap(action string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", action, err)
}
