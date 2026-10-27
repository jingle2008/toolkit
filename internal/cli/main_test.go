package cli

import (
	"fmt"
	"os"
	"testing"
)

// TestMain points the default log dir at a scratch directory, so tests
// that run logging commands without --log-dir never write session files
// into the developer's ~/.local/state.
func TestMain(m *testing.M) {
	state, err := os.MkdirTemp("", "toolkit-cli-test-state-")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	_ = os.Setenv("XDG_STATE_HOME", state)
	code := m.Run()
	_ = os.RemoveAll(state)
	os.Exit(code)
}
