package uistate

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad_MissingIsZero(t *testing.T) {
	t.Parallel()
	s, err := Load(filepath.Join(t.TempDir(), "nope.yaml"))
	require.NoError(t, err)
	assert.Equal(t, State{}, s)
}

func TestSaveLoad_RoundTrip(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "sub", "ui.yaml")
	require.NoError(t, State{KeymapVersion: 2}.Save(path))
	s, err := Load(path)
	require.NoError(t, err)
	assert.Equal(t, 2, s.KeymapVersion)
}

func TestLoad_CorruptIsZeroWithError(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "ui.yaml")
	require.NoError(t, os.WriteFile(path, []byte("keymapVersion: [oops"), 0o600))
	s, err := Load(path)
	require.Error(t, err)
	assert.Equal(t, State{}, s)
}
