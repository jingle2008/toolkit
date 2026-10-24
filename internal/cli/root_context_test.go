//nolint:paralleltest // NewRootCmd uses cobra global state and viper singleton
package cli

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type rootCtxKey struct{}

// The context handed to ExecuteContext (Execute's signal context in
// production) is the one a command's work runs under, so cancelling it
// reaches in-flight cluster calls.
func TestExecuteContext_ReachesCommandWork(t *testing.T) {
	stageMutationEnv(t)
	ctx, cancel := context.WithCancel(context.WithValue(context.Background(), rootCtxKey{}, "root"))
	var got context.Context
	defer swap(&drainNodeFn, func(c context.Context, _, _, _ string) error {
		got = c
		cancel()
		return nil
	})()

	cmd := NewRootCmd("vtest")
	cmd.SetArgs([]string{"drain", "node-a", "-y"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	require.NoError(t, cmd.ExecuteContext(ctx))
	require.NotNil(t, got)
	assert.Equal(t, "root", got.Value(rootCtxKey{}))
	assert.ErrorIs(t, got.Err(), context.Canceled)
}
