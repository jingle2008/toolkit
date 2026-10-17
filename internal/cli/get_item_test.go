package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jingle2008/toolkit/internal/cli/output"
	"github.com/jingle2008/toolkit/internal/config"
	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/pkg/models"
)

func emitItem(t *testing.T, cat domain.Category, name string, format output.Format) (string, error) {
	t.Helper()
	var buf bytes.Buffer
	err := emitCategory(context.Background(), &buf, emitLoader{}, cat, config.Config{}, models.Environment{},
		"", nil, 0, output.Options{Format: format, Item: name}, nil)
	return buf.String(), err
}

// A name picks one row, written as an object, not a list — flat or
// grouped, every category through the same writers.
func TestGetItem_WritesOneObject(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		cat  domain.Category
		name string
	}{
		{domain.GPUPool, "pool-a"},           // flat
		{domain.GPUNode, "node-a"},           // grouped
		{domain.Tenant, "tenant-a"},          // tenancy group
		{domain.LimitDefinition, "lim-a"},    // from the dataset
		{domain.DedicatedAICluster, "dac-a"}, // tenant-resolved
	} {
		t.Run(tc.cat.String(), func(t *testing.T) {
			t.Parallel()
			out, err := emitItem(t, tc.cat, tc.name, output.FormatJSON)
			require.NoError(t, err)
			var obj map[string]any
			require.NoError(t, json.Unmarshal([]byte(out), &obj), "want one object, got %s", out)
			assert.Equal(t, tc.name, obj["name"])
		})
	}
}

func TestGetItem_TableIsOneRow(t *testing.T) {
	t.Parallel()
	out, err := emitItem(t, domain.GPUNode, "node-a", output.FormatTable)
	require.NoError(t, err)
	lines := strings.Split(strings.TrimSpace(out), "\n")
	require.Len(t, lines, 2, "header and one row:\n%s", out)
	assert.Contains(t, lines[1], "node-a")
}

func TestGetItem_NotFound(t *testing.T) {
	t.Parallel()
	_, err := emitItem(t, domain.GPUNode, "node", output.FormatJSON)
	require.ErrorContains(t, err, `no GPUNode named "node" (the name must match exactly`)
}

//nolint:paralleltest // NewRootCmd uses cobra/viper global state
func TestGetCmd_NameArgs(t *testing.T) {
	_, err := runRootCmd(t, []string{"get", "alias", "tenant"}, "")
	require.ErrorContains(t, err, "`get alias` takes no name")
	_, err = runRootCmd(t, []string{"get", "tenant", "a", "b"}, "")
	require.ErrorContains(t, err, "accepts between 1 and 2 arg(s)")
}
