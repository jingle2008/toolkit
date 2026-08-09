package cli

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheckConfigFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	existing := filepath.Join(dir, "config.yaml")
	require.NoError(t, os.WriteFile(existing, []byte("repo_path: /tmp\n"), 0o600))

	t.Run("empty means file load disabled", func(t *testing.T) {
		t.Parallel()
		r := checkConfigFile("")
		assert.Equal(t, statusSkip, r.Status)
		assert.Contains(t, r.Detail, "disables file load")
	})

	t.Run("existing file passes", func(t *testing.T) {
		t.Parallel()
		r := checkConfigFile(existing)
		assert.Equal(t, statusPass, r.Status)
		assert.Equal(t, existing, r.Detail)
	})

	t.Run("missing file fails with an init hint", func(t *testing.T) {
		t.Parallel()
		r := checkConfigFile(filepath.Join(dir, "nope.yaml"))
		assert.Equal(t, statusFail, r.Status)
		assert.Contains(t, r.Detail, "does not exist")
		assert.Contains(t, r.Hint, "toolkit init")
	})
}

func TestCheckMetadataFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	existing := filepath.Join(dir, "metadata.yaml")
	require.NoError(t, os.WriteFile(existing, []byte("tenants: []\n"), 0o600))

	t.Run("unset skips", func(t *testing.T) {
		t.Parallel()
		r := checkMetadataFile("")
		assert.Equal(t, statusSkip, r.Status)
		assert.Equal(t, "not set", r.Detail)
	})

	t.Run("existing file passes", func(t *testing.T) {
		t.Parallel()
		r := checkMetadataFile(existing)
		assert.Equal(t, statusPass, r.Status)
		assert.Equal(t, existing, r.Detail)
	})

	t.Run("absent file skips rather than fails", func(t *testing.T) {
		t.Parallel()
		// The default points at ~/.config/toolkit/metadata.yaml, which most
		// installs won't have — a fresh install must not report a failure.
		r := checkMetadataFile(filepath.Join(dir, "absent.yaml"))
		assert.Equal(t, statusSkip, r.Status)
		assert.Contains(t, r.Detail, "optional")
	})
}

func TestCheckMetadataFile_StatErrorFails(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix permission semantics")
	}
	if os.Geteuid() == 0 {
		t.Skip("root bypasses directory permissions")
	}
	t.Parallel()

	// A stat error that isn't "not exist" — here an unsearchable parent
	// directory — is a real misconfiguration and must FAIL, not SKIP.
	dir := t.TempDir()
	blocked := filepath.Join(dir, "blocked")
	require.NoError(t, os.Mkdir(blocked, 0o700)) //nolint:gosec // G302: a directory needs its execute bit to be searchable; 0o700 is owner-only
	require.NoError(t, os.WriteFile(filepath.Join(blocked, "metadata.yaml"), []byte("x"), 0o600))
	require.NoError(t, os.Chmod(blocked, 0o000))
	//nolint:gosec // G302: a directory needs its execute bit to be searchable; 0o700 is owner-only
	t.Cleanup(func() { _ = os.Chmod(blocked, 0o700) })

	r := checkMetadataFile(filepath.Join(blocked, "metadata.yaml"))
	assert.Equal(t, statusFail, r.Status)
	assert.Contains(t, r.Hint, "permissions")
}

func TestCheckPath(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	t.Run("required and unset fails", func(t *testing.T) {
		t.Parallel()
		r := checkPath("repo-path", "", true, "set repo-path")
		assert.Equal(t, statusFail, r.Status)
		assert.Equal(t, "not set", r.Detail)
		assert.Equal(t, "set repo-path", r.Hint)
	})

	t.Run("optional and unset skips", func(t *testing.T) {
		t.Parallel()
		// kubeconfig is optional for repo-only categories.
		r := checkPath("kubeconfig", "", false, "set kubeconfig")
		assert.Equal(t, statusSkip, r.Status)
		assert.Empty(t, r.Hint)
	})

	t.Run("existing path passes", func(t *testing.T) {
		t.Parallel()
		r := checkPath("repo-path", dir, true, "hint")
		assert.Equal(t, statusPass, r.Status)
		assert.Equal(t, dir, r.Detail)
	})

	t.Run("missing path fails with the hint", func(t *testing.T) {
		t.Parallel()
		r := checkPath("repo-path", filepath.Join(dir, "nope"), true, "hint")
		assert.Equal(t, statusFail, r.Status)
		assert.Equal(t, "hint", r.Hint)
	})
}

func TestCountFails(t *testing.T) {
	t.Parallel()
	assert.Equal(t, 0, countFails(nil))
	assert.Equal(t, 0, countFails([]checkResult{{Status: statusPass}, {Status: statusSkip}}))
	assert.Equal(t, 2, countFails([]checkResult{
		{Status: statusFail}, {Status: statusPass}, {Status: statusFail},
	}))
}
