//go:build unix

package logging

import (
	"errors"
	"syscall"
)

// ProcessAlive reports whether pid names a live process: signal 0
// checks existence without delivering anything, and EPERM means it
// exists under another user. A recycled pid reads as alive, which only
// delays pruning that session.
func ProcessAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	err := syscall.Kill(pid, 0)
	return err == nil || errors.Is(err, syscall.EPERM)
}
