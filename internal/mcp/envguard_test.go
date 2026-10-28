package mcp

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jingle2008/toolkit/internal/logging"
	"github.com/jingle2008/toolkit/internal/mutation/ops"
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
//
//nolint:paralleltest // swaps package-level seams
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
//
//nolint:paralleltest // swaps package-level seams
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
//
//nolint:paralleltest // swaps package-level seams
func TestEnvGuard_SkipsIgnoredMutationOverrides(t *testing.T) {
	d := &ops.Deps{}
	loads := stubEnvironments(t, ashburnDev)
	performed := stubPerformSeams(d)
	res := callToolWith(t, d, "cordon_node", with(mutationArgs["cordon_node"], "env_realm", "oc99", "dry_run", true))
	require.False(t, res.IsError, resultText(t, res))
	assert.Zero(t, *loads)
	assert.Empty(t, *performed)
}

// When the repo's environments don't load, nothing is refused: the
// tool's own loaders report the repo's state.
//
//nolint:paralleltest // swaps package-level seams
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
	t.Parallel()
	for _, tool := range listedTools(t) {
		mutating := tool.Annotations == nil || !tool.Annotations.ReadOnlyHint
		assert.Equal(t, mutating, mutationTools[tool.Name], tool.Name)
	}
}

// A reload for an override the cache lacks doesn't hold up another
// call's override that the cache has.
//
//nolint:paralleltest // swaps package-level seams
func TestCheckEnvOverride_ReloadDoesNotBlockCachedOverrides(t *testing.T) {
	release := make(chan struct{})
	reloading := make(chan struct{})
	orig := mcpLoadEnvironmentsFn
	t.Cleanup(func() { mcpLoadEnvironmentsFn = orig })
	mcpLoadEnvironmentsFn = func(context.Context, string) ([]models.Environment, error) {
		close(reloading)
		<-release
		return []models.Environment{ashburnDev}, nil
	}
	s := &Server{logger: logging.Discard()}
	s.envCache.envs = []models.Environment{ashburnDev}

	done := make(chan error)
	go func() {
		done <- s.checkEnvOverride(context.Background(), envOverride{EnvRealm: "oc99", EnvType: "dev", EnvRegion: "iad"})
	}()
	<-reloading
	require.NoError(t, s.checkEnvOverride(context.Background(), envOverride{EnvRealm: "oc1", EnvType: "dev", EnvRegion: "iad"}),
		"answered from the cache while the reload runs")
	close(release)
	require.ErrorContains(t, <-done, "not an environment the repo declares")
}

// A reload that started earlier but finishes later doesn't overwrite
// the newer list a later-started reload already cached.
//
//nolint:paralleltest // swaps package-level seams
func TestCheckEnvOverride_StaleReloadDoesNotOverwrite(t *testing.T) {
	phxDev := models.Environment{Realm: "oc1", Type: "dev", Region: "us-phoenix-1"}
	release := make(chan struct{})
	slowStarted := make(chan struct{})
	calls := 0
	orig := mcpLoadEnvironmentsFn
	t.Cleanup(func() { mcpLoadEnvironmentsFn = orig })
	mcpLoadEnvironmentsFn = func(context.Context, string) ([]models.Environment, error) {
		calls++
		if calls == 1 {
			close(slowStarted)
			<-release
			return []models.Environment{ashburnDev}, nil
		}
		return []models.Environment{ashburnDev, phxDev}, nil
	}
	s := &Server{logger: logging.Discard()}

	done := make(chan error)
	go func() {
		done <- s.checkEnvOverride(context.Background(), envOverride{EnvRealm: "oc1", EnvType: "dev", EnvRegion: "iad"})
	}()
	<-slowStarted
	require.NoError(t, s.checkEnvOverride(context.Background(), envOverride{EnvRealm: "oc1", EnvType: "dev", EnvRegion: "phx"}))
	close(release)
	require.NoError(t, <-done)
	assert.Equal(t, []models.Environment{ashburnDev, phxDev}, s.envCache.envs)
}
