package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// shortNames reads a file laid out as the SDK's common/regions.go.
func TestShortNames(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "regions.go")
	require.NoError(t, os.WriteFile(path, []byte(`package common

const (
	RegionPHX Region = "us-phoenix-1"
	RegionIAD Region = "us-ashburn-1"
)

var shortNameRegion = map[string]Region{
	"phx": RegionPHX,
	"iad": RegionIAD,
}
`), 0o600))
	codes, err := shortNames(path)
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"phx": "us-phoenix-1", "iad": "us-ashburn-1"}, codes)

	src, err := render(codes, "v0")
	require.NoError(t, err)
	assert.Contains(t, string(src), `"iad": "us-ashburn-1",`)
	assert.Contains(t, string(src), "DO NOT EDIT")
}

func TestShortNames_LayoutChanged(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "regions.go")
	require.NoError(t, os.WriteFile(path, []byte("package common\n"), 0o600))
	_, err := shortNames(path)
	require.ErrorContains(t, err, "shortNameRegion not found")
}
