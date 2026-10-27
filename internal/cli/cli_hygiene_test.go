package cli

import (
	"bytes"
	"context"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jingle2008/toolkit/internal/logging"
	"github.com/jingle2008/toolkit/internal/mutation"
)

// logs ls -o accepts any case, as every other -o does.
func TestWriteSessions_FormatAnyCase(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	require.NoError(t, writeSessions(&buf, []logging.Session{{ID: "s1"}}, "JSON"))
	assert.Contains(t, buf.String(), `"id": "s1"`)
}

// A command's warnings go to its stderr, where SetErr points them.
func TestWarnf_GoesToCommandStderr(t *testing.T) {
	t.Parallel()
	var stderr bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	cmd.SetErr(&stderr)
	setWarnOut(cmd, nil)

	warnf(cmd.Context(), "tenant names unresolved: %s", "boom")
	assert.Equal(t, "warning: tenant names unresolved: boom\n", stderr.String())
}

// --yes says it is required exactly for an irreversible action.
func TestAddGateFlags_YesHelpFollowsTier(t *testing.T) {
	t.Parallel()
	for a, required := range map[mutation.Action]bool{
		mutation.TerminateNode: true,
		mutation.DeleteDAC:     true,
		mutation.CordonNode:    false,
		mutation.ScaleGPUPool:  false,
	} {
		var g cliGate
		cmd := &cobra.Command{}
		addGateFlags(cmd, a, &g)
		assert.Equal(t, required, cmd.Flags().Lookup("yes").Usage == "Required: this action has no interactive prompt", a.Verb)

		require.NoError(t, cmd.Flags().Parse([]string{"-n", "-y"}))
		assert.True(t, g.DryRun && g.Yes, "the flags bind to the gate")
	}
}
