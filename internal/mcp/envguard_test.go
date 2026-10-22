//nolint:paralleltest // replaces the package-level mcpLoadEnvironmentsFn seam
package mcp

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jingle2008/toolkit/pkg/models"
)

// stubEnvironments makes the repo declare envs, counting the loads.
func stubEnvironments(t *testing.T, envs ...models.Environment) *int {
	t.Helper()
	loads := 0
	orig := mcpLoadEnvironmentsFn
	t.Cleanup(func() { mcpLoadEnvironmentsFn = orig })
	mcpLoadEnvironmentsFn = func(context.Context, string) ([]models.Environment, error) {
		loads++
		return envs, nil
	}
	return &loads
}

var ashburnDev = models.Environment{Realm: "oc1", Type: "dev", Region: "us-ashburn-1"}

// An override naming an environment the repo doesn't declare is refused
// before the handler runs, instead of loading the startup cluster's rows
// under it.
func TestEnvGuard_RefusesUnknownOverride(t *testing.T) {
	stubEnvironments(t, ashburnDev, models.Environment{Realm: "oc1", Type: "dev", Region: "us-phoenix-1"})
	for _, args := range []map[string]any{
		{"env_realm": "oc99"},
		{"env_type": "bogus"},
		{"env_region": "us-chicago-1"},
	} {
		res := callTool(t, "list_gpu_nodes", args)
		require.True(t, res.IsError, "%v", args)
		assert.Contains(t, resultText(t, res), "not an environment the repo declares", "%v", args)
	}

	res := callTool(t, "list_gpu_pools", map[string]any{"env_region": "xyz"})
	require.True(t, res.IsError)
	assert.Contains(t, resultText(t, res), `unknown region code "xyz"`)
}

// A declared environment passes, in any case and as a short code.
func TestEnvGuard_AcceptsKnownOverride(t *testing.T) {
	stubEnvironments(t, ashburnDev, models.Environment{Realm: "oc1", Type: "dev", Region: "us-phoenix-1"})
	for _, args := range []map[string]any{
		{"env_realm": "OC1"},
		{"env_region": "phx"},
		{"env_type": "DEV", "env_region": "us-phoenix-1", "env_realm": " oc1 "},
	} {
		res := callTool(t, "list_gpu_nodes", args)
		require.False(t, res.IsError, "%v: %s", args, resultText(t, res))
	}

	res := callTool(t, "list_gpu_nodes", nil)
	require.False(t, res.IsError)
}

// Mutation tools ignore overrides without --mutation-env-override-allowed,
// so the guard doesn't refuse them either.
func TestEnvGuard_SkipsIgnoredMutationOverrides(t *testing.T) {
	loads := stubEnvironments(t, ashburnDev)
	performed := stubPerformSeams(t)
	res := callTool(t, "cordon_node", with(mutationArgs["cordon_node"], "env_realm", "oc99", "dry_run", true))
	require.False(t, res.IsError, resultText(t, res))
	assert.Zero(t, *loads)
	assert.Empty(t, *performed)
}

// When the repo's environments don't load, nothing is refused: the
// tool's own loaders report the repo's state.
func TestEnvGuard_UnloadableEnvironmentsPass(t *testing.T) {
	orig := mcpLoadEnvironmentsFn
	t.Cleanup(func() { mcpLoadEnvironmentsFn = orig })
	mcpLoadEnvironmentsFn = func(context.Context, string) ([]models.Environment, error) {
		return nil, errors.New("no repo")
	}
	res := callTool(t, "list_gpu_nodes", map[string]any{"env_realm": "oc99"})
	require.False(t, res.IsError, resultText(t, res))
}

// mutationTools names every mutating tool, so the guard knows which
// overrides are ignored.
func TestMutationTools_ListedForEnvGuard(t *testing.T) {
	for _, tool := range listedTools(t) {
		mutating := tool.Annotations == nil || !tool.Annotations.ReadOnlyHint
		assert.Equal(t, mutating, mutationTools[tool.Name], tool.Name)
	}
}
