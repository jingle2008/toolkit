//go:build unix

package configloader

import (
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
)

// lockFile blocks until it holds an exclusive advisory lock on path,
// creating the file (and its directory) if missing, and returns the
// release. The lock file is never removed: deleting it would let two
// processes lock different inodes under the same name.
func lockFile(path string) (func(), error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o600) //nolint:gosec // the lock next to the user's own metadata file
	if err != nil {
		return nil, err
	}
	for {
		err = unix.Flock(int(f.Fd()), unix.LOCK_EX)
		if err != unix.EINTR { //nolint:errorlint // Flock returns a bare Errno
			break
		}
	}
	if err != nil {
		_ = f.Close()
		return nil, err
	}
	return func() {
		_ = unix.Flock(int(f.Fd()), unix.LOCK_UN)
		_ = f.Close()
	}, nil
}
