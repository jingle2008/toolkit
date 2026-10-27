//nolint:paralleltest // NewRootCmd uses cobra global state and viper singleton
package cli

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jingle2008/toolkit/internal/configloader"
	"github.com/jingle2008/toolkit/pkg/models"
)

// stubRepoEnvironments points the env check at a repo declaring envs.
func stubRepoEnvironments(t *testing.T, envs ...models.Environment) {
	t.Helper()
	t.Setenv("TOOLKIT_REPO_PATH", t.TempDir())
	t.Cleanup(swap(&loadEnvironmentsFn, func(context.Context, string) ([]models.Environment, error) { return envs, nil }))
}

// A command refuses an env the repo doesn't declare, before it loads or
// acts on anything; the realm is matched in any case.
func TestEnvironmentCheck_RefusesUnknownEnv(t *testing.T) {
	stageMutationEnv(t) // dev / us-ashburn-1 / oc1
	stubRepoEnvironments(t, models.Environment{Realm: "oc1", Type: "dev", Region: "us-ashburn-1"})

	out, err := runRootCmd(t, []string{"cordon", "node-a", "-n", "-m", "oc99"})
	require.ErrorIs(t, err, configloader.ErrUnknownEnvironment)
	assert.Contains(t, err.Error(), `realm "oc99"`)
	assert.NotContains(t, out, "DRY-RUN")

	_, err = runRootCmd(t, []string{"get", "gpunode", "-m", "oc99"})
	require.ErrorIs(t, err, configloader.ErrUnknownEnvironment)

	out, err = runRootCmd(t, []string{"cordon", "node-a", "-n", "-m", "OC1"})
	require.NoError(t, err)
	assert.Contains(t, out, "DRY-RUN")
}

// A repo whose environments don't load isn't grounds to refuse: the
// command's loaders report the repo's state themselves.
func TestEnvironmentCheck_UnloadableRepoPasses(t *testing.T) {
	stageMutationEnv(t)
	t.Setenv("TOOLKIT_REPO_PATH", t.TempDir())
	defer swap(&loadEnvironmentsFn, func(context.Context, string) ([]models.Environment, error) {
		return nil, errors.New("no service tenancies")
	})()
	out, err := runRootCmd(t, []string{"cordon", "node-a", "-n", "-m", "oc99"})
	require.NoError(t, err)
	assert.Contains(t, out, "DRY-RUN")
}
