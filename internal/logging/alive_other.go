//go:build !unix

package logging

import "os"

// ProcessAlive reports whether pid names a live process. On Windows
// FindProcess opens a handle and fails for a pid that doesn't exist.
func ProcessAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	p, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	_ = p.Release()
	return true
}
