/*
Package errors provides error helpers and sentinel errors for the toolkit application.
*/
package errors

import (
	"fmt"
)

/*
ErrUnknown is a generic sentinel error for unknown error cases.
*/
var ErrUnknown = fmt.Errorf("unknown error")

// Wrap returns an error with consistent phrasing: "action: %w"
func Wrap(action string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", action, err)
}
