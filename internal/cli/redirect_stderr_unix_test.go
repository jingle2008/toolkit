//go:build unix

//nolint:paralleltest // swaps the process's fd 2
package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRedirectStderr_KeepsOnlyNonEmptyCapture(t *testing.T) {
	dir := t.TempDir()

	empty := filepath.Join(dir, "empty.stderr")
	restore, err := redirectStderr(empty)
	if err != nil {
		t.Fatal(err)
	}
	restore()
	if _, err := os.Stat(empty); !os.IsNotExist(err) {
		t.Errorf("empty capture left behind: %v", err)
	}

	noisy := filepath.Join(dir, "noisy.stderr")
	restore, err = redirectStderr(noisy)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = os.Stderr.WriteString("plugin noise\n")
	restore()
	data, err := os.ReadFile(noisy) //nolint:gosec // G304: test temp path
	if err != nil || string(data) != "plugin noise\n" {
		t.Errorf("capture = %q, %v", data, err)
	}
}
