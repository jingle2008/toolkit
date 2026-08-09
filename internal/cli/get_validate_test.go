package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jingle2008/toolkit/internal/config"
	"github.com/jingle2008/toolkit/internal/domain"
)

func fullGetConfig(kubeCfg string) config.Config {
	return config.Config{
		RepoPath:   "/tmp",
		EnvType:    "dev",
		EnvRegion:  "us-ashburn-1",
		EnvRealm:   "oc1",
		KubeConfig: kubeCfg,
	}
}

func TestValidateGetConfig_AliasNeedsNothing(t *testing.T) {
	t.Parallel()
	// Alias is a static enum dump, so it must not be gated on repo_path
	// or the env triple.
	require.NoError(t, validateGetConfig(config.Config{}, domain.Alias))
}

func TestValidateGetConfig_ReportsMissingSettings(t *testing.T) {
	t.Parallel()
	err := validateGetConfig(config.Config{}, domain.Tenant)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing required setting(s)")
	// The message must name the remediation paths, since this is the
	// first thing a new user hits.
	assert.Contains(t, err.Error(), "TOOLKIT_")
	assert.Contains(t, err.Error(), "toolkit init")
}

func TestValidateGetConfig_MissingKubeConfigForClusterCategory(t *testing.T) {
	t.Parallel()
	require.True(t, domain.GPUNode.NeedsKubeConfig(), "precondition: GPUNode is cluster-derived")

	err := validateGetConfig(fullGetConfig(""), domain.GPUNode)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--kubeconfig")
}

func TestValidateGetConfig_UnreadableKubeConfigFailsFast(t *testing.T) {
	t.Parallel()
	// A set-but-missing kubeconfig is stat'ed up front so the user gets a
	// clear message instead of a deep client-go error.
	err := validateGetConfig(fullGetConfig(filepath.Join(t.TempDir(), "no-kubeconfig")), domain.GPUNode)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not readable")
}

func TestValidateGetConfig_ClusterCategoryWithReadableKubeConfig(t *testing.T) {
	t.Parallel()
	kubeCfg := filepath.Join(t.TempDir(), "kubeconfig")
	require.NoError(t, os.WriteFile(kubeCfg, []byte("apiVersion: v1\n"), 0o600))

	require.NoError(t, validateGetConfig(fullGetConfig(kubeCfg), domain.GPUNode))
}

func TestValidateGetConfig_RepoOnlyCategoryIgnoresKubeConfig(t *testing.T) {
	t.Parallel()
	require.False(t, domain.Tenant.NeedsKubeConfig(), "precondition: Tenant is repo-derived")

	// No kubeconfig needed, so a blank one must not block a repo-only read.
	require.NoError(t, validateGetConfig(fullGetConfig(""), domain.Tenant))
}
