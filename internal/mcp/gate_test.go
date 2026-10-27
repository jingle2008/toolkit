package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jingle2008/toolkit/internal/config"
	"github.com/jingle2008/toolkit/internal/infra/k8s"
	"github.com/jingle2008/toolkit/internal/infra/oci"
	"github.com/jingle2008/toolkit/internal/mutation"
	"github.com/jingle2008/toolkit/internal/mutation/ops"
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

// stubPerformSeams makes every lookup in d succeed and records which
// actions were performed, so a test can tell a lookup from an action.
func stubPerformSeams(d *ops.Deps) *[]string {
	performed := &[]string{}
	did := func(name string) { *performed = append(*performed, name) }
	stubAllMutationSeams(d)
	d.SetCordon = func(context.Context, string, string, string, bool) (bool, error) { did("cordon"); return true, nil }
	d.StartDrain = func(context.Context, string, string, string) (k8s.DrainStatus, error) {
		did("drain")
		return k8s.DrainStatus{}, nil
	}
	d.ResolveGPUNode = func(_ context.Context, _ models.Environment, name, _ string) (*models.GPUNode, error) {
		return &models.GPUNode{Name: name, ID: "ocid1.instance.resolved"}, nil
	}
	d.SoftReset = func(context.Context, *models.GPUNode, models.Environment, *slog.Logger) error {
		did("reset")
		return nil
	}
	d.Terminate = func(context.Context, *models.GPUNode, models.Environment, *slog.Logger) error {
		did("terminate")
		return nil
	}
	d.ResolveGPUPool = func(_ context.Context, _ models.Environment, name string) (*models.GPUPool, error) {
		return &models.GPUPool{Name: name, ID: "ocid1.pool", Size: 4, ActualSize: 2}, nil
	}
	d.IncreasePoolSize = func(context.Context, *models.GPUPool, int, models.Environment, *slog.Logger) (oci.PoolScale, error) {
		did("scale")
		return oci.PoolScale{}, nil
	}
	d.DeleteDAC = func(context.Context, *models.DedicatedAICluster, models.Environment, *slog.Logger) error {
		did("delete-dac")
		return nil
	}
	d.UpsertTenant = func(context.Context, models.TenantMetadata) error { did("set-tenant"); return nil }
	d.ResolveISVC = func(context.Context, string, string, string, string) (string, error) { return "ns-a", nil }
	d.RestartISVC = func(context.Context, string, string, string, string) error { did("restart-isvc"); return nil }
	d.PlanOrphanISVC = func(context.Context, string, string, string, string) (k8s.OrphanDeleteOutcome, error) {
		return k8s.OrphanFinalized, nil
	}
	d.DeleteOrphanISVC = func(context.Context, string, string, string, string) (k8s.OrphanDeleteOutcome, error) {
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

// callToolWith calls tool name on a server whose infra calls are d.
func callToolWith(t *testing.T, d *ops.Deps, name string, args map[string]any) *sdk.CallToolResult {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	res, err := newTestPair(ctx, t, stubLoader{}, withDeps(d)).CallTool(ctx, &sdk.CallToolParams{Name: name, Arguments: args})
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
		out[kv[i].(string)] = kv[i+1]
	}
	return out
}

// A dry run looks the target up but never acts, with or without confirm.
func TestMutationTools_DryRunNeverActs(t *testing.T) {
	t.Parallel()
	d := &ops.Deps{}
	performed := stubPerformSeams(d)
	for name, args := range mutationArgs {
		for _, confirm := range []bool{false, true} {
			res := callToolWith(t, d, name, with(args, "dry_run", true, "confirm", confirm))
			require.False(t, res.IsError, "%s: %s", name, resultText(t, res))
			assert.Equal(t, "DRY_RUN", structured(t, res)["status"], name)
		}
	}
	assert.Empty(t, *performed)
}

// A dry run reports what the lookup found.
func TestMutationTools_DryRunReportsResolved(t *testing.T) {
	t.Parallel()
	d := &ops.Deps{}
	stubPerformSeams(d)

	sc := structured(t, callToolWith(t, d, "terminate_node", with(mutationArgs["terminate_node"], "dry_run", true)))
	assert.Equal(t, map[string]any{"instance_id": "ocid1.instance.resolved"}, sc["resolved"])

	sc = structured(t, callToolWith(t, d, "delete_inference_service", with(mutationArgs["delete_inference_service"], "dry_run", true)))
	assert.Equal(t, "force-delete", sc["action"], "the planned action")
	assert.Equal(t, "ns-a/isvc-a", sc["target"], "the resolved target")
	assert.Equal(t, map[string]any{"namespace": "ns-a"}, sc["resolved"])

	sc = structured(t, callToolWith(t, d, "scale_gpu_pool", with(mutationArgs["scale_gpu_pool"], "dry_run", true)))
	assert.Equal(t, map[string]any{
		"pool_id": "ocid1.pool", "size": float64(4), "actual_size": float64(2), "add": float64(2), "target_size": float64(4),
	}, sc["resolved"])
	assert.Equal(t, "Grows the instance pool from 2 to 4 instances.", sc["note"])
}

// confirm=true doesn't run an irreversible tool: confirm_target must name
// the target, as requested or as resolved.
func TestIrreversibleTools_RequireConfirmTarget(t *testing.T) {
	t.Parallel()
	d := &ops.Deps{}
	performed := stubPerformSeams(d)
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
			res := callToolWith(t, d, c.tool, a)
			require.True(t, res.IsError, "%s with confirm_target %v", c.tool, bad)
			assert.Contains(t, resultText(t, res), "confirm_target", c.tool)
		}
		require.Empty(t, *performed, "%s acted without confirm_target", c.tool)

		res := callToolWith(t, d, c.tool, with(args, "confirm_target", c.good))
		require.False(t, res.IsError, "%s: %s", c.tool, resultText(t, res))
		if c.resolved != "" {
			res = callToolWith(t, d, c.tool, with(args, "confirm_target", c.resolved))
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
	sess := newTestPair(ctx, t, stubLoader{}, withConfig(func(c *config.Config) { c.MCPReadOnly = true }))
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
	sess := newTestPair(ctx, t, stubLoader{}, withConfig(func(c *config.Config) { c.MCPReadOnly = true }))
	_, err := sess.CallTool(ctx, &sdk.CallToolParams{
		Name:      "terminate_node",
		Arguments: map[string]any{"node": "node-a", "confirm": true, "confirm_target": "node-a"},
	})
	require.ErrorContains(t, err, `unknown tool "terminate_node"`)
}

// Every tool's dry run looks its target up as a real call would, so a
// target that doesn't exist fails the dry run instead of reporting
// DRY_RUN — an agent can't dry-run a typo and then confirm it.
func TestMutationTools_DryRunFailsOnMissingTarget(t *testing.T) {
	t.Parallel()
	d := &ops.Deps{}
	performed := stubPerformSeams(d)
	missing := errors.New("ghost not found")
	d.NodeCordoned = func(context.Context, string, string, string) (bool, error) { return false, missing }
	d.DrainStatus = func(context.Context, string, string, string) (k8s.DrainStatus, error) {
		return k8s.DrainStatus{}, missing
	}
	d.GetDAC = func(context.Context, *models.DedicatedAICluster, models.Environment) (oci.DACState, error) {
		return oci.DACState{}, missing
	}
	d.CheckTenant = func(models.TenantMetadata) (bool, error) { return false, missing }
	d.ResolveGPUNode = func(context.Context, models.Environment, string, string) (*models.GPUNode, error) {
		return nil, missing
	}
	d.ResolveGPUPool = func(context.Context, models.Environment, string) (*models.GPUPool, error) {
		return nil, missing
	}
	d.ResolveISVC = func(context.Context, string, string, string, string) (string, error) { return "", missing }

	for name, args := range mutationArgs {
		res := callToolWith(t, d, name, with(args, "dry_run", true))
		require.True(t, res.IsError, name)
		assert.Contains(t, resultText(t, res), "ghost not found", name)
	}
	assert.Empty(t, *performed)
}

// A blank required name is refused by name, before any lookup, rather
// than passed to the cluster or OCI.
func TestTools_BlankTargetRefused(t *testing.T) {
	t.Parallel()
	d := &ops.Deps{}
	performed := stubPerformSeams(d)
	for name, args := range mutationArgs {
		if name == "set_tenant" {
			continue // its name and OCID have their own checks
		}
		blank := with(args)
		for k := range blank {
			blank[k] = "  "
		}
		res := callToolWith(t, d, name, with(blank, "dry_run", true))
		require.True(t, res.IsError, name)
		assert.Contains(t, resultText(t, res), "target is required", name)
	}
	assert.Empty(t, *performed)

	res := callToolWith(t, d, "get_drain_status", map[string]any{"node": ""})
	require.True(t, res.IsError)
	assert.Contains(t, resultText(t, res), "node is required")
	res = callToolWith(t, d, "metrics_url", map[string]any{"category": "dac", "name": " "})
	require.True(t, res.IsError)
	assert.Contains(t, resultText(t, res), "name is required")
}
