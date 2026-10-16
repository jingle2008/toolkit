package configloader

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// An in-place edit that keeps the file's size still changes the
// fingerprint (via its modification time), as do added files; an
// untouched tree fingerprints the same.
func TestTenancyOverrideFingerprint(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	dir := filepath.Join(getLimitsRoot(repo), limitsKey+tenancyOverridesKey, regionalValuesDir, "oc1", "t1")
	require.NoError(t, os.MkdirAll(dir, 0o750))
	file := filepath.Join(dir, "a.json")
	require.NoError(t, os.WriteFile(file, []byte(`{"x":1}`), 0o600))

	base, err := TenancyOverrideFingerprint(repo, "oc1")
	require.NoError(t, err)
	same, err := TenancyOverrideFingerprint(repo, "oc1")
	require.NoError(t, err)
	assert.Equal(t, base, same)

	require.NoError(t, os.WriteFile(file, []byte(`{"x":2}`), 0o600))
	later := time.Now().Add(time.Second)
	require.NoError(t, os.Chtimes(file, later, later))
	edited, err := TenancyOverrideFingerprint(repo, "oc1")
	require.NoError(t, err)
	assert.NotEqual(t, base, edited, "same-size in-place edit")

	require.NoError(t, os.WriteFile(filepath.Join(dir, "b.json"), []byte(`{}`), 0o600))
	added, err := TenancyOverrideFingerprint(repo, "oc1")
	require.NoError(t, err)
	assert.NotEqual(t, edited, added, "added file")

	other, err := TenancyOverrideFingerprint(repo, "oc19")
	require.NoError(t, err)
	assert.NotEqual(t, added, other, "realms differ")
}
