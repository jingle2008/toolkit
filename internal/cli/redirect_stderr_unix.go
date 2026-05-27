//go:build unix

package cli

import (
	"fmt"
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
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o644) //nolint:gosec // path is the user-configured log-file location (cfg.LogFile + ".stderr"); user choice is the design.
	if err != nil {
		return nil, fmt.Errorf("open stderr sink %q: %w", path, err)
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
		// Restore so any post-exit writes (e.g., logger Sync errors,
		// deferred cleanups) reach the user's actual terminal. Order
		// matters: restore fd 2 before closing f so any stderr write
		// between the dup2 and the close lands on the real terminal,
		// not a closed fd.
		_ = unix.Dup2(orig, int(os.Stderr.Fd()))
		_ = unix.Close(orig)
		_ = f.Close()
	}, nil
}
