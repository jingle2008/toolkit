package mcp

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jingle2008/toolkit/internal/config"
	"github.com/jingle2008/toolkit/internal/infra/k8s"
	"github.com/jingle2008/toolkit/internal/mutation"
	"github.com/jingle2008/toolkit/pkg/infra/logging"
	"github.com/jingle2008/toolkit/pkg/models"
)

// toolActions maps every mutating tool to the actions it can perform.
var toolActions = map[string][]mutation.Action{
	"cordon_node":               {mutation.CordonNode},
	"uncordon_node":             {mutation.UncordonNode},
	"drain_node":                {mutation.DrainNode},
	"restart_node":              {mutation.RestartNode},
	"terminate_node":            {mutation.TerminateNode},
	"scale_gpu_pool":            {mutation.ScaleGPUPool},
	"delete_dac":                {mutation.DeleteDAC},
	"restart_inference_service": {mutation.RestartInferenceService},
	"delete_inference_service":  {mutation.DeleteInferenceService, mutation.ForceDeleteInferenceService},
	"set_tenant":                {mutation.SetTenant},
}

// Every action in the shared list has a tool, and a tool takes
// confirm_target exactly when it can perform an Irreversible action.
func TestMutationTools_CoverPolicy(t *testing.T) {
	t.Parallel()
	covered := map[mutation.Action]bool{}
	for _, as := range toolActions {
		for _, a := range as {
			covered[a] = true
		}
	}
	for _, a := range mutation.Actions() {
		assert.True(t, covered[a], "no MCP tool performs %s", a)
	}

	tools := map[string]*sdk.Tool{}
	for _, tool := range listedTools(t) {
		tools[tool.Name] = tool
	}
	for name, as := range toolActions {
		tool, ok := tools[name]
		if !assert.True(t, ok, "%s not registered", name) {
			continue
		}
		raw, err := json.Marshal(tool.InputSchema)
		require.NoError(t, err)
		var schema struct {
			Properties map[string]any `json:"properties"`
		}
		require.NoError(t, json.Unmarshal(raw, &schema))
		irreversible := false
		for _, a := range as {
			irreversible = irreversible || a.Tier == mutation.Irreversible
		}
		assert.Contains(t, schema.Properties, "dry_run", name)
		_, hasTarget := schema.Properties["confirm_target"]
		assert.Equal(t, irreversible, hasTarget, "%s confirm_target", name)
		assert.Equal(t, irreversible, strings.Contains(tool.Description, "IRREVERSIBLE"), "%s description", name)
	}
}

// stubPerformSeams makes every lookup succeed and records which actions
// were performed, so a test can tell a lookup from an action.
func stubPerformSeams(t *testing.T) *[]string {
	t.Helper()
	performed := &[]string{}
	did := func(name string) { *performed = append(*performed, name) }
	_, restore := stubAllMutationSeams()
	t.Cleanup(restore)
	mcpSetCordonFn = func(context.Context, string, string, string, bool) (bool, error) { did("cordon"); return true, nil }
	mcpStartDrainFn = func(context.Context, string, string, string) (k8s.DrainStatus, error) {
		did("drain")
		return k8s.DrainStatus{}, nil
	}
	mcpResolveGPUNodeFn = func(_ context.Context, _ *Server, _ models.Environment, name, _ string) (*models.GPUNode, error) {
		return &models.GPUNode{Name: name, ID: "ocid1.instance.resolved"}, nil
	}
	mcpSoftResetFn = func(context.Context, *models.GPUNode, models.Environment, logging.Logger) error {
		did("reset")
		return nil
	}
	mcpTerminateFn = func(context.Context, *models.GPUNode, models.Environment, logging.Logger) error {
		did("terminate")
		return nil
	}
	mcpResolveGPUPoolFn = func(_ context.Context, _ *Server, _ models.Environment, name string) (*models.GPUPool, error) {
		return &models.GPUPool{Name: name, ID: "ocid1.pool", Size: 4, ActualSize: 2}, nil
	}
	mcpIncreasePoolSizeFn = func(context.Context, *models.GPUPool, models.Environment, logging.Logger) error {
		did("scale")
		return nil
	}
	mcpDeleteDACFn = func(context.Context, *models.DedicatedAICluster, models.Environment, logging.Logger) error {
		did("delete-dac")
		return nil
	}
	mcpUpsertTenantFn = func(*Server, models.TenantMetadata) error { did("set-tenant"); return nil }
	mcpResolveISVCFn = func(context.Context, string, string, string, string) (string, error) { return "ns-a", nil }
	mcpRestartISVCFn = func(context.Context, string, string, string, string) error { did("restart-isvc"); return nil }
	mcpPlanOrphanISVCFn = func(context.Context, string, string, string, string) (k8s.OrphanDeleteOutcome, error) {
		return k8s.OrphanFinalized, nil
	}
	mcpDeleteOrphanISVCFn = func(context.Context, string, string, string, string) (k8s.OrphanDeleteOutcome, error) {
		did("delete-isvc")
		return k8s.OrphanFinalized, nil
	}
	return performed
}

func callTool(t *testing.T, name string, args map[string]any) *sdk.CallToolResult {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	res, err := newTestPair(ctx, t, stubLoader{}).CallTool(ctx, &sdk.CallToolParams{Name: name, Arguments: args})
	require.NoError(t, err)
	return res
}

// mutationArgs are valid arguments for each mutating tool, minus the gate.
var mutationArgs = map[string]map[string]any{
	"cordon_node":               {"node": "node-a"},
	"uncordon_node":             {"node": "node-a"},
	"drain_node":                {"node": "node-a"},
	"restart_node":              {"node": "node-a"},
	"terminate_node":            {"node": "node-a"},
	"scale_gpu_pool":            {"name": "pool-a"},
	"delete_dac":                {"name": "dac-a"},
	"restart_inference_service": {"name": "isvc-a"},
	"delete_inference_service":  {"name": "isvc-a"},
	"set_tenant":                {"ocid": "ocid1.tenancy.oc1..a", "name": "acme"},
}

func with(args map[string]any, kv ...any) map[string]any {
	out := map[string]any{}
	for k, v := range args {
		out[k] = v
	}
	for i := 0; i+1 < len(kv); i += 2 {
		out[kv[i].(string)] = kv[i+1] //nolint:forcetypeassert // keys are strings
	}
	return out
}

// A dry run looks the target up but never acts, with or without confirm.
//
//nolint:paralleltest // replaces package-level seams
func TestMutationTools_DryRunNeverActs(t *testing.T) {
	performed := stubPerformSeams(t)
	for name, args := range mutationArgs {
		for _, confirm := range []bool{false, true} {
			res := callTool(t, name, with(args, "dry_run", true, "confirm", confirm))
			require.False(t, res.IsError, "%s: %s", name, resultText(t, res))
			assert.Equal(t, "DRY_RUN", structured(t, res)["status"], name)
		}
	}
	assert.Empty(t, *performed)
}

// A dry run reports what the lookup found.
//
//nolint:paralleltest // replaces package-level seams
func TestMutationTools_DryRunReportsResolved(t *testing.T) {
	stubPerformSeams(t)

	sc := structured(t, callTool(t, "terminate_node", with(mutationArgs["terminate_node"], "dry_run", true)))
	assert.Equal(t, map[string]any{"instance_id": "ocid1.instance.resolved"}, sc["resolved"])

	sc = structured(t, callTool(t, "delete_inference_service", with(mutationArgs["delete_inference_service"], "dry_run", true)))
	assert.Equal(t, "force-delete", sc["action"], "the planned action")
	assert.Equal(t, "ns-a/isvc-a", sc["target"], "the resolved target")
	assert.Equal(t, map[string]any{"namespace": "ns-a"}, sc["resolved"])

	sc = structured(t, callTool(t, "scale_gpu_pool", with(mutationArgs["scale_gpu_pool"], "dry_run", true)))
	assert.Equal(t, map[string]any{"pool_id": "ocid1.pool", "size": float64(4), "actual_size": float64(2)}, sc["resolved"])
}

// confirm=true doesn't run an irreversible tool: confirm_target must name
// the target, as requested or as resolved.
//
//nolint:paralleltest // replaces package-level seams
func TestIrreversibleTools_RequireConfirmTarget(t *testing.T) {
	performed := stubPerformSeams(t)
	cases := []struct {
		tool, good, resolved string
	}{
		{"terminate_node", "node-a", ""},
		{"delete_dac", "dac-a", ""},
		{"delete_inference_service", "isvc-a", "ns-a/isvc-a"},
	}
	for _, c := range cases {
		args := with(mutationArgs[c.tool], "confirm", true)
		for _, bad := range []any{nil, "", "node-b", "dac-b"} {
			a := args
			if bad != nil {
				a = with(args, "confirm_target", bad)
			}
			res := callTool(t, c.tool, a)
			require.True(t, res.IsError, "%s with confirm_target %v", c.tool, bad)
			assert.Contains(t, resultText(t, res), "confirm_target", c.tool)
		}
		require.Empty(t, *performed, "%s acted without confirm_target", c.tool)

		res := callTool(t, c.tool, with(args, "confirm_target", c.good))
		require.False(t, res.IsError, "%s: %s", c.tool, resultText(t, res))
		if c.resolved != "" {
			res = callTool(t, c.tool, with(args, "confirm_target", c.resolved))
			require.False(t, res.IsError, "%s resolved target: %s", c.tool, resultText(t, res))
		}
		*performed = nil
	}
}

// With --mcp-read-only no mutating tool is registered at all, so no
// prompt can reach one; every read tool, get_drain_status included, is.
func TestReadOnlyServer_RegistersNoMutationTools(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	sess := newTestPair(ctx, t, stubLoader{}, func(c *config.Config) { c.MCPReadOnly = true })
	res, err := sess.ListTools(ctx, &sdk.ListToolsParams{})
	require.NoError(t, err)

	got := map[string]bool{}
	for _, tool := range res.Tools {
		got[tool.Name] = true
		assert.True(t, tool.Annotations != nil && tool.Annotations.ReadOnlyHint, "%s is not read-only", tool.Name)
	}
	for name := range toolActions {
		assert.False(t, got[name], "read-only server registers %s", name)
	}
	for name, hint := range wantHints {
		if hint == hintReadOnly {
			assert.True(t, got[name], "read-only server is missing %s", name)
		}
	}
}

// A mutating tool called on a read-only server is unknown, not refused.
func TestReadOnlyServer_RejectsMutationCalls(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	sess := newTestPair(ctx, t, stubLoader{}, func(c *config.Config) { c.MCPReadOnly = true })
	_, err := sess.CallTool(ctx, &sdk.CallToolParams{
		Name:      "terminate_node",
		Arguments: map[string]any{"node": "node-a", "confirm": true, "confirm_target": "node-a"},
	})
	require.ErrorContains(t, err, `unknown tool "terminate_node"`)
}
