package mcp

import (
	"context"
	"strings"
	"testing"
	"time"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jingle2008/toolkit/internal/config"
)

// toolHint is the behavior a tool advertises to MCP clients, which use
// it to decide what may run without asking the operator.
type toolHint int

const (
	hintReadOnly    toolHint = iota // no side effects
	hintIdempotent                  // mutates, safe to repeat
	hintRecoverable                 // mutates, can be undone or retried
	hintDestructive                 // mutates irreversibly
)

// wantHints lists every tool the server registers. A tool missing here
// fails the test, so a new tool cannot ship without annotations.
var wantHints = map[string]toolHint{
	"list_tenants":              hintReadOnly,
	"list_base_models":          hintReadOnly,
	"list_serving_runtimes":     hintReadOnly,
	"list_imported_models":      hintReadOnly,
	"list_gpu_pools":            hintReadOnly,
	"list_gpu_nodes":            hintReadOnly,
	"list_gpu_workloads":        hintReadOnly,
	"list_dacs":                 hintReadOnly,
	"list_inference_services":   hintReadOnly,
	"list_environments":         hintReadOnly,
	"list_service_tenancies":    hintReadOnly,
	"list_model_artifacts":      hintReadOnly,
	"list_definitions":          hintReadOnly,
	"list_tenancy_overrides":    hintReadOnly,
	"list_regional_overrides":   hintReadOnly,
	"list_related":              hintReadOnly,
	"list_aliases":              hintReadOnly,
	"get_drain_status":          hintReadOnly,
	"cordon_node":               hintIdempotent,
	"uncordon_node":             hintIdempotent,
	"drain_node":                hintIdempotent,
	"scale_gpu_pool":            hintIdempotent,
	"set_tenant":                hintIdempotent,
	"restart_node":              hintRecoverable,
	"restart_inference_service": hintRecoverable,
	"terminate_node":            hintDestructive,
	"delete_dac":                hintDestructive,
	"delete_inference_service":  hintDestructive,
}

func listedTools(t *testing.T) []*sdk.Tool {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	res, err := newTestPair(ctx, t, stubLoader{}).ListTools(ctx, &sdk.ListToolsParams{})
	require.NoError(t, err)
	return res.Tools
}

func TestTools_Annotations(t *testing.T) {
	t.Parallel()
	seen := make(map[string]bool)
	for _, tool := range listedTools(t) {
		seen[tool.Name] = true
		want, ok := wantHints[tool.Name]
		if !assert.True(t, ok, "tool %q has no entry in wantHints", tool.Name) {
			continue
		}
		a := tool.Annotations
		if !assert.NotNil(t, a, "tool %q has no annotations", tool.Name) {
			continue
		}
		assert.Equal(t, want == hintReadOnly, a.ReadOnlyHint, "%s readOnlyHint", tool.Name)
		if want == hintReadOnly {
			continue
		}
		// The spec defaults destructiveHint to true, so mutating tools
		// must say false explicitly.
		if assert.NotNil(t, a.DestructiveHint, "%s destructiveHint unset", tool.Name) {
			assert.Equal(t, want == hintDestructive, *a.DestructiveHint, "%s destructiveHint", tool.Name)
		}
		assert.Equal(t, want == hintIdempotent, a.IdempotentHint, "%s idempotentHint", tool.Name)
	}
	for name := range wantHints {
		assert.True(t, seen[name], "wantHints lists %q, which is not registered", name)
	}
}

// The env-override footer names the real flag, so an agent relaying it
// to the operator gives a flag that exists.
func TestMutationTools_FooterNamesRealFlag(t *testing.T) {
	t.Parallel()
	flag := "--" + config.FlagMutationEnvOverrideAllowed
	for _, tool := range listedTools(t) {
		if strings.Contains(tool.Description, "env_type/env_region/env_realm, but per-call overrides") {
			assert.Contains(t, tool.Description, flag, tool.Name)
		}
	}
	assert.Contains(t, mutationToolFooter, flag)
}
