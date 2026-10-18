// Package uistate persists small TUI preferences across launches: today,
// the keymap version the user has last been told about.
package uistate

import (
	"errors"
	"fmt"
	"os"

	"sigs.k8s.io/yaml"

	"github.com/jingle2008/toolkit/internal/fileutil"
)

// State is the persisted UI state.
type State struct {
	// KeymapVersion is the TUI keymap version whose changes the user has
	// seen; a lower value than the binary's shows the keys-changed hint.
	KeymapVersion int `json:"keymapVersion"`
}

// Load reads the state from path. A missing file yields the zero State;
// a malformed one yields the zero State and an error, so the hint shows
// once and the next save repairs the file.
func Load(path string) (State, error) {
	data, err := os.ReadFile(path) //nolint:gosec // path is the app-owned state file
	if errors.Is(err, os.ErrNotExist) {
		return State{}, nil
	}
	if err != nil {
		return State{}, fmt.Errorf("read ui state: %w", err)
	}
	var s State
	if err := yaml.Unmarshal(data, &s); err != nil {
		return State{}, fmt.Errorf("parse ui state: %w", err)
	}
	return s, nil
}

// Save writes the state to path atomically, creating parent directories.
func (s State) Save(path string) error {
	data, err := yaml.Marshal(s)
	if err != nil {
		return fmt.Errorf("marshal ui state: %w", err)
	}
	if err := fileutil.WriteFileAtomic(path, data, 0o600); err != nil {
		return fmt.Errorf("write ui state: %w", err)
	}
	return nil
}
