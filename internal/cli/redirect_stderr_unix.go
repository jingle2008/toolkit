//go:build unix

package cli

import (
	"errors"
	"fmt"
	"io/fs"
	"os"

	"golang.org/x/sys/unix"
)

// redirectStderr points the process's fd 2 at the given file for the
// duration of the returned closure's lifetime. It's used only by the
// TUI path: bubbletea's alt-screen has exclusive ownership of the
// terminal, so any bytes written to fd 2 by client-go's exec auth
// plugins (oci-cli prints "Abort:" on a non-tty prompt failure) or
// runtime panic stacks would otherwise interleave with bubbletea's
// frame writes and corrupt the rendered display.
//
// The fd is swapped at the kernel level via unix.Dup2 — not via
// reassigning os.Stderr — so child processes spawned by exec.Cmd
// inherit the redirected fd. unix.Dup3-based on linux/arm64,
// dup2-based on darwin and the other unix targets.
func redirectStderr(path string) (func(), error) {
	f, err := openStderrSink(path)
	if err != nil {
		return nil, err
	}
	orig, err := unix.Dup(int(os.Stderr.Fd()))
	if err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("dup stderr: %w", err)
	}
	if err := unix.Dup2(int(f.Fd()), int(os.Stderr.Fd())); err != nil {
		_ = f.Close()
		_ = unix.Close(orig)
		return nil, fmt.Errorf("redirect stderr: %w", err)
	}
	return func() {
		// Restore so any post-exit writes (e.g., log-file close errors,
		// deferred cleanups) reach the user's actual terminal. Order
		// matters: restore fd 2 before closing f so any stderr write
		// between the dup2 and the close lands on the real terminal,
		// not a closed fd.
		_ = unix.Dup2(orig, int(os.Stderr.Fd()))
		_ = unix.Close(orig)
		// Most sessions capture nothing; don't leave an empty file
		// behind for each one. A shared --log-file means another
		// process may hold the same sink: its shared lock makes the
		// exclusive one fail, and the file stays.
		if unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB) == nil {
			if fi, err := f.Stat(); err == nil && fi.Size() == 0 {
				_ = os.Remove(path)
			}
		}
		_ = f.Close()
	}, nil
}

// openStderrSink opens path for appending under a shared flock, which
// tells an exiting process sharing the sink not to remove it. If that
// process removed it between this open and the lock, the fd is an
// unlinked file, so open again.
func openStderrSink(path string) (*os.File, error) {
	for {
		// 0600: the sink can capture exec auth-plugin output and panic stacks, so
		// keep it private rather than world-readable, consistent with metadata/config.
		//nolint:gosec // G304: path is the log file plus ".stderr", under the user's log dir or --log-file.
		f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o600)
		if err != nil {
			return nil, fmt.Errorf("open stderr sink %q: %w", path, err)
		}
		if err := unix.Flock(int(f.Fd()), unix.LOCK_SH); err != nil {
			_ = f.Close()
			return nil, fmt.Errorf("lock stderr sink %q: %w", path, err)
		}
		held, err := f.Stat()
		if err != nil {
			_ = f.Close()
			return nil, fmt.Errorf("stat stderr sink %q: %w", path, err)
		}
		cur, err := os.Stat(path)
		if err == nil && os.SameFile(held, cur) {
			return f, nil
		}
		_ = f.Close()
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("stat stderr sink %q: %w", path, err)
		}
	}
}
