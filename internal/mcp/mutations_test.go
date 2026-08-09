//nolint:paralleltest // global seam vars (mcpSetCordonFn et al.) make these tests inherently sequential
package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jingle2008/toolkit/internal/config"
	"github.com/jingle2008/toolkit/pkg/infra/logging"
	"github.com/jingle2008/toolkit/pkg/models"
)

func TestIntegration_MutationTool_RequiresConfirm(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)

	called := false
	orig := mcpSetCordonFn
	defer func() { mcpSetCordonFn = orig }()
	mcpSetCordonFn = func(context.Context, string, string, string, bool) (bool, error) {
		called = true
		return true, nil
	}

	clientSess := newTestPair(ctx, t, stubLoader{})

	// confirm omitted (default false) → refused
	res, err := clientSess.CallTool(ctx, &sdk.CallToolParams{
		Name:      "cordon_node",
		Arguments: map[string]any{"node": "node-a"},
	})
	require.NoError(t, err)
	require.NotNil(t, res)
	assert.True(t, res.IsError, "expected IsError when confirm omitted")
	if called {
		t.Fatal("must not call k8s when confirm=false")
	}

	// The error text must explain how to proceed.
	body := resultText(t, res)
	if !strings.Contains(body, "confirm=true") {
		t.Errorf("expected refusal to mention confirm=true, got: %q", body)
	}
	if !strings.Contains(body, "node/node-a") {
		t.Errorf("expected refusal to name the target, got: %q", body)
	}
}

// assertMutationOK checks the mutationResult envelope a successful
// mutation returns. It replaces the old success notification, which
// carried the same action/kind/target triple out-of-band until SEP-2577
// deprecated that channel.
func assertMutationOK(t *testing.T, res *sdk.CallToolResult, action, kind, target string) {
	t.Helper()
	sc := structured(t, res)
	assert.Equal(t, "OK", sc["status"])
	assert.Equal(t, action, sc["action"])
	assert.Equal(t, kind, sc["kind"])
	assert.Equal(t, target, sc["target"])
}

// stubAllMutationSeams points every mutation action/resolver seam at a stub
// that records invocation, so a refusal test can prove no action ran. Returns
// a pointer to the shared "called" flag and a restore func.
func stubAllMutationSeams() (*bool, func()) {
	called := false
	mark := func() { called = true }

	oCordon, oDrain := mcpSetCordonFn, mcpDrainNodeFn
	oResolveNode, oReset, oTerm := mcpResolveGPUNodeFn, mcpSoftResetFn, mcpTerminateFn
	oResolvePool, oInc, oDelDAC := mcpResolveGPUPoolFn, mcpIncreasePoolSizeFn, mcpDeleteDACFn
	oUpsert := mcpUpsertTenantFn

	mcpSetCordonFn = func(context.Context, string, string, string, bool) (bool, error) { mark(); return true, nil }
	mcpDrainNodeFn = func(context.Context, string, string, string) error { mark(); return nil }
	mcpResolveGPUNodeFn = func(_ context.Context, _ *Server, _ models.Environment, name, ocid string) (*models.GPUNode, error) {
		mark()
		return &models.GPUNode{Name: name, ID: ocid}, nil
	}
	mcpSoftResetFn = func(context.Context, *models.GPUNode, models.Environment, logging.Logger) error { mark(); return nil }
	mcpTerminateFn = func(context.Context, *models.GPUNode, models.Environment, logging.Logger) error { mark(); return nil }
	mcpResolveGPUPoolFn = func(_ context.Context, _ *Server, _ models.Environment, name string) (*models.GPUPool, error) {
		mark()
		return &models.GPUPool{Name: name, ID: "ocid1.pool", Size: 1}, nil
	}
	mcpIncreasePoolSizeFn = func(context.Context, *models.GPUPool, models.Environment, logging.Logger) error { mark(); return nil }
	mcpDeleteDACFn = func(context.Context, *models.DedicatedAICluster, models.Environment, logging.Logger) error {
		mark()
		return nil
	}
	mcpUpsertTenantFn = func(*Server, models.TenantMetadata) error { mark(); return nil }

	return &called, func() {
		mcpSetCordonFn, mcpDrainNodeFn = oCordon, oDrain
		mcpResolveGPUNodeFn, mcpSoftResetFn, mcpTerminateFn = oResolveNode, oReset, oTerm
		mcpResolveGPUPoolFn, mcpIncreasePoolSizeFn, mcpDeleteDACFn = oResolvePool, oInc, oDelDAC
		mcpUpsertTenantFn = oUpsert
	}
}

// Every mutation tool — not just cordon_node — must refuse without acting when
// confirm is omitted. Guards the server-wide safety contract against a new
// mutation tool being added that forgets the confirm gate.
func TestIntegration_AllMutationTools_RefuseWithoutConfirm(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	t.Cleanup(cancel)

	calledPtr, restore := stubAllMutationSeams()
	t.Cleanup(restore)

	tools := []struct {
		name string
		args map[string]any
	}{
		{"cordon_node", map[string]any{"node": "node-a"}},
		{"uncordon_node", map[string]any{"node": "node-a"}},
		{"drain_node", map[string]any{"node": "node-a"}},
		{"reboot_node", map[string]any{"node": "node-a"}},
		{"terminate_node", map[string]any{"node": "node-a"}},
		{"scale_gpu_pool", map[string]any{"name": "pool-a"}},
		{"delete_dac", map[string]any{"name": "dac-a"}},
		{"set_tenant", map[string]any{"ocid": "ocid1.tenancy.oc1..aaaa", "name": "Acme"}},
	}

	for _, tc := range tools {
		t.Run(tc.name, func(t *testing.T) {
			*calledPtr = false
			clientSess := newTestPair(ctx, t, stubLoader{})

			res, err := clientSess.CallTool(ctx, &sdk.CallToolParams{
				Name:      tc.name,
				Arguments: tc.args, // confirm omitted → must refuse
			})
			require.NoError(t, err)
			require.NotNil(t, res)
			assert.True(t, res.IsError, "%s must refuse when confirm omitted", tc.name)
			assert.False(t, *calledPtr, "%s must not invoke its action when confirm omitted", tc.name)

			assert.Contains(t, resultText(t, res), "confirm=true",
				"%s should explain how to proceed", tc.name)
		})
	}
}

func TestIntegration_MutationTool_ConfirmTrueExecutes(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)

	var (
		gotNode string
		gotWant bool
	)
	orig := mcpSetCordonFn
	defer func() { mcpSetCordonFn = orig }()
	mcpSetCordonFn = func(_ context.Context, _, _, node string, want bool) (bool, error) {
		gotNode, gotWant = node, want
		return true, nil
	}

	clientSess := newTestPair(ctx, t, stubLoader{})

	res, err := clientSess.CallTool(ctx, &sdk.CallToolParams{
		Name:      "cordon_node",
		Arguments: map[string]any{"node": "node-a", "confirm": true},
	})
	require.NoError(t, err)
	require.NotNil(t, res)
	assert.False(t, res.IsError, "expected success with confirm=true")
	assert.Equal(t, "node-a", gotNode)
	assert.True(t, gotWant, "cordon_node should pass want=true")

	// Response body is the mutationResult envelope directly — no
	// items wrapper (mutations got their own typed shape in the
	// MCP result refactor).
	text := res.Content[0].(*sdk.TextContent).Text
	var env struct {
		Status string `json:"status"`
		Action string `json:"action"`
		Kind   string `json:"kind"`
		Target string `json:"target"`
	}
	require.NoError(t, json.Unmarshal([]byte(text), &env))
	assert.Equal(t, "OK", env.Status)
	assert.Equal(t, "cordon", env.Action)
	assert.Equal(t, "node", env.Kind)
	assert.Equal(t, "node-a", env.Target)

	// StructuredContent must carry the same mutationResult shape (no
	// items/count wrapper). Regression bait for the MCP refactor: if
	// a future change accidentally routes mutations back through
	// jsonResult, TextContent above would still pass — only this
	// assertion would catch the wrong envelope shipping in
	// StructuredContent.
	require.NotNil(t, res.StructuredContent)
	scBytes, err := json.Marshal(res.StructuredContent)
	require.NoError(t, err)
	var sc map[string]any
	require.NoError(t, json.Unmarshal(scBytes, &sc))
	assert.Equal(t, "OK", sc["status"])
	assert.Equal(t, "cordon", sc["action"])
	assert.Equal(t, "node", sc["kind"])
	assert.Equal(t, "node-a", sc["target"])
	_, hasItems := sc["items"]
	assert.False(t, hasItems, "mutation response should not carry a list-envelope items key")
	_, hasCount := sc["count"]
	assert.False(t, hasCount, "mutation response should not carry a list-envelope count key")
}

func TestIntegration_UncordonTool_PassesWantFalse(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)

	var gotWant bool
	orig := mcpSetCordonFn
	defer func() { mcpSetCordonFn = orig }()
	mcpSetCordonFn = func(_ context.Context, _, _, _ string, want bool) (bool, error) {
		gotWant = want
		return true, nil
	}

	clientSess := newTestPair(ctx, t, stubLoader{})

	_, err := clientSess.CallTool(ctx, &sdk.CallToolParams{
		Name:      "uncordon_node",
		Arguments: map[string]any{"node": "node-a", "confirm": true},
	})
	require.NoError(t, err)
	if gotWant {
		t.Error("uncordon_node must call SetCordon with want=false")
	}
}

func TestIntegration_TerminateTool_OcidBypass(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)

	var gotNode *models.GPUNode
	orig := mcpTerminateFn
	defer func() { mcpTerminateFn = orig }()
	mcpTerminateFn = func(_ context.Context, n *models.GPUNode, _ models.Environment, _ logging.Logger) error {
		gotNode = n
		return nil
	}

	clientSess := newTestPair(ctx, t, stubLoader{})

	res, err := clientSess.CallTool(ctx, &sdk.CallToolParams{
		Name: "terminate_node",
		Arguments: map[string]any{
			"node":    "node-a",
			"ocid":    "ocid1.instance.fake",
			"confirm": true,
		},
	})
	require.NoError(t, err)
	assert.False(t, res.IsError)
	if gotNode == nil || gotNode.ID != "ocid1.instance.fake" {
		t.Errorf("expected synthesized node, got: %+v", gotNode)
	}
}

func TestIntegration_RebootTool_ConfirmTrueExecutes(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)

	var gotNode *models.GPUNode
	orig := mcpSoftResetFn
	defer func() { mcpSoftResetFn = orig }()
	mcpSoftResetFn = func(_ context.Context, n *models.GPUNode, _ models.Environment, _ logging.Logger) error {
		gotNode = n
		return nil
	}

	clientSess := newTestPair(ctx, t, stubLoader{})

	res, err := clientSess.CallTool(ctx, &sdk.CallToolParams{
		Name: "reboot_node",
		Arguments: map[string]any{
			"node":    "node-a",
			"ocid":    "ocid1.instance.fake",
			"confirm": true,
		},
	})
	require.NoError(t, err)
	require.NotNil(t, res)
	assert.False(t, res.IsError)
	if gotNode == nil || gotNode.ID != "ocid1.instance.fake" {
		t.Errorf("expected synthesized node from --ocid, got: %+v", gotNode)
	}
	assertMutationOK(t, res, "reboot", "node", "node-a")
}

func TestIntegration_ScaleGPUPoolTool_ConfirmTrueExecutes(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)

	// Stub the MCP-level resolver seam so we don't need a fake k8s
	// + OCI pipeline; the resolver is covered in internal/resolve.
	origResolve := mcpResolveGPUPoolFn
	defer func() { mcpResolveGPUPoolFn = origResolve }()
	mcpResolveGPUPoolFn = func(_ context.Context, _ *Server, _ models.Environment, name string) (*models.GPUPool, error) {
		return &models.GPUPool{Name: name, ID: "ocid1.instancepool.fake", Size: 12, ActualSize: 4}, nil
	}

	var gotPool *models.GPUPool
	origInc := mcpIncreasePoolSizeFn
	defer func() { mcpIncreasePoolSizeFn = origInc }()
	mcpIncreasePoolSizeFn = func(_ context.Context, p *models.GPUPool, _ models.Environment, _ logging.Logger) error {
		gotPool = p
		return nil
	}

	clientSess := newTestPair(ctx, t, stubLoader{})

	res, err := clientSess.CallTool(ctx, &sdk.CallToolParams{
		Name: "scale_gpu_pool",
		Arguments: map[string]any{
			"name":    "pool-a",
			"confirm": true,
		},
	})
	require.NoError(t, err)
	require.NotNil(t, res)
	assert.False(t, res.IsError)
	if gotPool == nil || gotPool.ID != "ocid1.instancepool.fake" {
		t.Errorf("expected resolver-supplied pool, got: %+v", gotPool)
	}
	assertMutationOK(t, res, "scale", "gpu_pool", "pool-a")
}

func TestIntegration_ScaleGPUPoolTool_ResolverError(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)

	origResolve := mcpResolveGPUPoolFn
	defer func() { mcpResolveGPUPoolFn = origResolve }()
	mcpResolveGPUPoolFn = func(context.Context, *Server, models.Environment, string) (*models.GPUPool, error) {
		return nil, errors.New("gpu pool \"pool-x\" not found in repo")
	}

	clientSess := newTestPair(ctx, t, stubLoader{})

	res, err := clientSess.CallTool(ctx, &sdk.CallToolParams{
		Name: "scale_gpu_pool",
		Arguments: map[string]any{
			"name":    "pool-x",
			"confirm": true,
		},
	})
	require.NoError(t, err)
	require.NotNil(t, res)
	assert.True(t, res.IsError, "expected IsError when resolver fails")
}

func TestIntegration_MutationTool_HonorsEnvOverride_WhenAllowed(t *testing.T) {
	// With MutationEnvOverrideAllowed=true, per-call env_* fields flow
	// into the env the handler hands to the action — same semantics as
	// list_* tools.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)

	var gotEnv models.Environment
	orig := mcpDeleteDACFn
	defer func() { mcpDeleteDACFn = orig }()
	mcpDeleteDACFn = func(_ context.Context, _ *models.DedicatedAICluster, env models.Environment, _ logging.Logger) error {
		gotEnv = env
		return nil
	}

	clientSess := newTestPair(ctx, t, stubLoader{}, func(c *config.Config) {
		c.MutationEnvOverrideAllowed = true
	})

	_, err := clientSess.CallTool(ctx, &sdk.CallToolParams{
		Name: "delete_dac",
		Arguments: map[string]any{
			"name":       "dac-x",
			"confirm":    true,
			"env_realm":  "oc2",
			"env_region": "us-phoenix-1",
		},
	})
	require.NoError(t, err)
	assert.Equal(t, "oc2", gotEnv.Realm, "env_realm override should reach the action")
	assert.Equal(t, "us-phoenix-1", gotEnv.Region, "env_region override should reach the action")
	// env_type wasn't overridden, so the startup default ("dev") wins.
	assert.Equal(t, "dev", gotEnv.Type, "unset override field falls back to startup env")
}

func TestIntegration_MutationTool_IgnoresEnvOverride_WhenDisallowed(t *testing.T) {
	// Default (MutationEnvOverrideAllowed=false): the agent's env_*
	// fields are silently dropped. The action sees the startup env.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)

	var gotEnv models.Environment
	orig := mcpDeleteDACFn
	defer func() { mcpDeleteDACFn = orig }()
	mcpDeleteDACFn = func(_ context.Context, _ *models.DedicatedAICluster, env models.Environment, _ logging.Logger) error {
		gotEnv = env
		return nil
	}

	clientSess := newTestPair(ctx, t, stubLoader{}) // flag NOT set

	_, err := clientSess.CallTool(ctx, &sdk.CallToolParams{
		Name: "delete_dac",
		Arguments: map[string]any{
			"name":       "dac-x",
			"confirm":    true,
			"env_realm":  "oc2",          // requested
			"env_region": "us-phoenix-1", // requested
		},
	})
	require.NoError(t, err)
	assert.Equal(t, "oc1", gotEnv.Realm, "env_realm override must be ignored")
	assert.Equal(t, "us-ashburn-1", gotEnv.Region, "env_region override must be ignored")
}

// TestIntegration_MutationTool_PropagatesEnvOverride asserts the
// override reaches every action's input across reboot/terminate/scale
// — not just delete_dac. These three thread env through additional
// hops (resolver, OCI client construction) where a future refactor
// could accidentally drop it.
func TestIntegration_MutationTool_PropagatesEnvOverride(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)

	// reboot — env reaches the action AND the resolver.
	var gotResolverEnv, gotActionEnv models.Environment
	origResolve := mcpResolveGPUNodeFn
	defer func() { mcpResolveGPUNodeFn = origResolve }()
	mcpResolveGPUNodeFn = func(_ context.Context, _ *Server, env models.Environment, name, ocid string) (*models.GPUNode, error) {
		gotResolverEnv = env
		return &models.GPUNode{Name: name, ID: ocid}, nil
	}
	origReset := mcpSoftResetFn
	defer func() { mcpSoftResetFn = origReset }()
	mcpSoftResetFn = func(_ context.Context, _ *models.GPUNode, env models.Environment, _ logging.Logger) error {
		gotActionEnv = env
		return nil
	}

	clientSess := newTestPair(ctx, t, stubLoader{}, func(c *config.Config) {
		c.MutationEnvOverrideAllowed = true
	})

	_, err := clientSess.CallTool(ctx, &sdk.CallToolParams{
		Name: "reboot_node",
		Arguments: map[string]any{
			"node":      "node-a",
			"ocid":      "ocid1.instance.fake",
			"confirm":   true,
			"env_realm": "oc2",
		},
	})
	require.NoError(t, err)
	assert.Equal(t, "oc2", gotResolverEnv.Realm, "reboot resolver must see overridden realm")
	assert.Equal(t, "oc2", gotActionEnv.Realm, "reboot action must see overridden realm")

	// terminate — same shape, different action seam.
	gotActionEnv = models.Environment{}
	origTerm := mcpTerminateFn
	defer func() { mcpTerminateFn = origTerm }()
	mcpTerminateFn = func(_ context.Context, _ *models.GPUNode, env models.Environment, _ logging.Logger) error {
		gotActionEnv = env
		return nil
	}
	_, err = clientSess.CallTool(ctx, &sdk.CallToolParams{
		Name: "terminate_node",
		Arguments: map[string]any{
			"node":      "node-a",
			"ocid":      "ocid1.instance.fake",
			"confirm":   true,
			"env_realm": "oc2",
		},
	})
	require.NoError(t, err)
	assert.Equal(t, "oc2", gotActionEnv.Realm, "terminate action must see overridden realm")

	// scale — env reaches pool resolver AND IncreasePoolSize.
	gotResolverEnv = models.Environment{}
	gotActionEnv = models.Environment{}
	origPoolResolve := mcpResolveGPUPoolFn
	defer func() { mcpResolveGPUPoolFn = origPoolResolve }()
	mcpResolveGPUPoolFn = func(_ context.Context, _ *Server, env models.Environment, name string) (*models.GPUPool, error) {
		gotResolverEnv = env
		return &models.GPUPool{Name: name, ID: "ocid1.pool", Size: 4}, nil
	}
	origInc := mcpIncreasePoolSizeFn
	defer func() { mcpIncreasePoolSizeFn = origInc }()
	mcpIncreasePoolSizeFn = func(_ context.Context, _ *models.GPUPool, env models.Environment, _ logging.Logger) error {
		gotActionEnv = env
		return nil
	}
	_, err = clientSess.CallTool(ctx, &sdk.CallToolParams{
		Name: "scale_gpu_pool",
		Arguments: map[string]any{
			"name":      "pool-a",
			"confirm":   true,
			"env_realm": "oc2",
		},
	})
	require.NoError(t, err)
	assert.Equal(t, "oc2", gotResolverEnv.Realm, "scale pool resolver must see overridden realm")
	assert.Equal(t, "oc2", gotActionEnv.Realm, "scale action must see overridden realm")
}

func TestIntegration_DeleteDACTool_ConfirmTrueExecutes(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)

	var gotDAC *models.DedicatedAICluster
	orig := mcpDeleteDACFn
	defer func() { mcpDeleteDACFn = orig }()
	mcpDeleteDACFn = func(_ context.Context, d *models.DedicatedAICluster, _ models.Environment, _ logging.Logger) error {
		gotDAC = d
		return nil
	}

	clientSess := newTestPair(ctx, t, stubLoader{})

	res, err := clientSess.CallTool(ctx, &sdk.CallToolParams{
		Name: "delete_dac",
		Arguments: map[string]any{
			"name":    "dac-x",
			"confirm": true,
		},
	})
	require.NoError(t, err)
	require.NotNil(t, res)
	assert.False(t, res.IsError)
	if gotDAC == nil || gotDAC.Name != "dac-x" {
		t.Errorf("expected DAC with Name=dac-x, got: %+v", gotDAC)
	}
	assertMutationOK(t, res, "delete", "dac", "dac-x")
}

func TestIntegration_MutationTool_PerformErrorPropagates(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)

	orig := mcpDrainNodeFn
	defer func() { mcpDrainNodeFn = orig }()
	mcpDrainNodeFn = func(context.Context, string, string, string) error {
		return errors.New("pods stuck terminating")
	}

	clientSess := newTestPair(ctx, t, stubLoader{})

	res, err := clientSess.CallTool(ctx, &sdk.CallToolParams{
		Name:      "drain_node",
		Arguments: map[string]any{"node": "node-a", "confirm": true},
	})
	require.NoError(t, err, "tool call should not error at transport level")
	require.NotNil(t, res)
	assert.True(t, res.IsError)

	// The error text carries the underlying message.
	body := resultText(t, res)
	if !strings.Contains(body, "pods stuck terminating") {
		t.Errorf("expected error to mention the underlying cause; got %q", body)
	}
}

func TestIntegration_MutationTools_RegisteredInListTools(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)

	clientSess := newTestPair(ctx, t, stubLoader{})

	listRes, err := clientSess.ListTools(ctx, &sdk.ListToolsParams{})
	require.NoError(t, err)
	got := make(map[string]bool, len(listRes.Tools))
	for _, tool := range listRes.Tools {
		got[tool.Name] = true
	}
	for _, name := range []string{
		"cordon_node", "uncordon_node", "drain_node",
		"reboot_node", "terminate_node",
		"scale_gpu_pool", "delete_dac",
		"set_tenant",
	} {
		assert.True(t, got[name], "tools/list missing mutation tool %q", name)
	}
}

func TestIntegration_SetTenantTool_ConfirmTrueExecutes(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)

	var got models.TenantMetadata
	orig := mcpUpsertTenantFn
	defer func() { mcpUpsertTenantFn = orig }()
	mcpUpsertTenantFn = func(_ *Server, e models.TenantMetadata) error {
		got = e
		return nil
	}

	clientSess := newTestPair(ctx, t, stubLoader{})

	res, err := clientSess.CallTool(ctx, &sdk.CallToolParams{
		Name: "set_tenant",
		Arguments: map[string]any{
			"ocid":    "ocid1.tenancy.oc1..aaaa",
			"name":    "Acme",
			"confirm": true,
		},
	})
	require.NoError(t, err)
	require.NotNil(t, res)
	assert.False(t, res.IsError)
	if got.ID != "ocid1.tenancy.oc1..aaaa" || got.Name == nil || *got.Name != "Acme" {
		t.Errorf("unexpected entry: %+v", got)
	}
	if got.IsInternal == nil || *got.IsInternal != true {
		t.Errorf("IsInternal should default true, got %v", got.IsInternal)
	}
	assertMutationOK(t, res, "set", "tenant", "ocid1.tenancy.oc1..aaaa")
}

func TestIntegration_SetTenantTool_RejectsBadOCID(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)

	orig := mcpUpsertTenantFn
	defer func() { mcpUpsertTenantFn = orig }()
	mcpUpsertTenantFn = func(*Server, models.TenantMetadata) error {
		t.Fatal("must not write for bad OCID")
		return nil
	}

	clientSess := newTestPair(ctx, t, stubLoader{})

	res, err := clientSess.CallTool(ctx, &sdk.CallToolParams{
		Name:      "set_tenant",
		Arguments: map[string]any{"ocid": "nope", "name": "Acme", "confirm": true},
	})
	require.NoError(t, err)
	require.NotNil(t, res)
	assert.True(t, res.IsError, "bad OCID must error")
}

func TestIntegration_SetTenantTool_RejectsBadOCID_WithoutConfirm(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)

	orig := mcpUpsertTenantFn
	defer func() { mcpUpsertTenantFn = orig }()
	mcpUpsertTenantFn = func(*Server, models.TenantMetadata) error {
		t.Fatal("must not write for bad OCID even without confirm")
		return nil
	}

	clientSess := newTestPair(ctx, t, stubLoader{})

	res, err := clientSess.CallTool(ctx, &sdk.CallToolParams{
		Name:      "set_tenant",
		Arguments: map[string]any{"ocid": "nope", "name": "Acme"},
	})
	require.NoError(t, err)
	require.NotNil(t, res)
	assert.True(t, res.IsError, "bad OCID must error even without confirm")
}

func TestIntegration_SetTenantTool_RequiresName(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)

	orig := mcpUpsertTenantFn
	defer func() { mcpUpsertTenantFn = orig }()
	mcpUpsertTenantFn = func(*Server, models.TenantMetadata) error {
		t.Fatal("must not write when name missing")
		return nil
	}

	clientSess := newTestPair(ctx, t, stubLoader{})

	res, err := clientSess.CallTool(ctx, &sdk.CallToolParams{
		Name:      "set_tenant",
		Arguments: map[string]any{"ocid": "ocid1.tenancy.oc1..aaaa", "confirm": true},
	})
	require.NoError(t, err)
	require.NotNil(t, res)
	assert.True(t, res.IsError, "missing name must error")
}
