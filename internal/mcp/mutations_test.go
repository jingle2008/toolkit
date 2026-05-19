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

	rec := &recorder{}
	clientSess := newTestPair(ctx, t, stubLoader{}, rec)

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

	// Notification should explain the refusal.
	msgs := waitForMsgs(t, rec)
	body, _ := msgs[0].Data.(string)
	if !strings.Contains(body, "refused") {
		t.Errorf("expected refusal notification, got: %q", body)
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

	rec := &recorder{}
	clientSess := newTestPair(ctx, t, stubLoader{}, rec)

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

	// Info notification on success.
	msgs := waitForMsgs(t, rec)
	body, _ := msgs[0].Data.(string)
	assert.Contains(t, body, "cordon node/node-a: OK")
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

	rec := &recorder{}
	clientSess := newTestPair(ctx, t, stubLoader{}, rec)

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

	var gotNode *models.GpuNode
	orig := mcpTerminateFn
	defer func() { mcpTerminateFn = orig }()
	mcpTerminateFn = func(_ context.Context, n *models.GpuNode, _ models.Environment, _ logging.Logger) error {
		gotNode = n
		return nil
	}

	rec := &recorder{}
	clientSess := newTestPair(ctx, t, stubLoader{}, rec)

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

	var gotNode *models.GpuNode
	orig := mcpSoftResetFn
	defer func() { mcpSoftResetFn = orig }()
	mcpSoftResetFn = func(_ context.Context, n *models.GpuNode, _ models.Environment, _ logging.Logger) error {
		gotNode = n
		return nil
	}

	rec := &recorder{}
	clientSess := newTestPair(ctx, t, stubLoader{}, rec)

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
	// Info notification on success.
	msgs := waitForMsgs(t, rec)
	body, _ := msgs[0].Data.(string)
	assert.Contains(t, body, "reboot node/node-a: OK")
}

func TestIntegration_ScaleGpuPoolTool_ConfirmTrueExecutes(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)

	// Stub the MCP-level resolver seam so we don't need a fake k8s
	// + OCI pipeline; the resolver is covered in internal/resolve.
	origResolve := mcpResolveGpuPoolFn
	defer func() { mcpResolveGpuPoolFn = origResolve }()
	mcpResolveGpuPoolFn = func(_ context.Context, _ *Server, _ models.Environment, name string) (*models.GpuPool, error) {
		return &models.GpuPool{Name: name, ID: "ocid1.instancepool.fake", Size: 12, ActualSize: 4}, nil
	}

	var gotPool *models.GpuPool
	origInc := mcpIncreasePoolSizeFn
	defer func() { mcpIncreasePoolSizeFn = origInc }()
	mcpIncreasePoolSizeFn = func(_ context.Context, p *models.GpuPool, _ models.Environment, _ logging.Logger) error {
		gotPool = p
		return nil
	}

	rec := &recorder{}
	clientSess := newTestPair(ctx, t, stubLoader{}, rec)

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
	msgs := waitForMsgs(t, rec)
	body, _ := msgs[0].Data.(string)
	assert.Contains(t, body, "scale gpu_pool/pool-a: OK")
}

func TestIntegration_ScaleGpuPoolTool_ResolverError(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)

	origResolve := mcpResolveGpuPoolFn
	defer func() { mcpResolveGpuPoolFn = origResolve }()
	mcpResolveGpuPoolFn = func(context.Context, *Server, models.Environment, string) (*models.GpuPool, error) {
		return nil, errors.New("gpu pool \"pool-x\" not found in repo")
	}

	rec := &recorder{}
	clientSess := newTestPair(ctx, t, stubLoader{}, rec)

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

	rec := &recorder{}
	clientSess := newTestPair(ctx, t, stubLoader{}, rec, func(c *config.Config) {
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

	rec := &recorder{}
	clientSess := newTestPair(ctx, t, stubLoader{}, rec) // flag NOT set

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
	origResolve := mcpResolveGpuNodeFn
	defer func() { mcpResolveGpuNodeFn = origResolve }()
	mcpResolveGpuNodeFn = func(_ context.Context, _ *Server, env models.Environment, name, ocid string) (*models.GpuNode, error) {
		gotResolverEnv = env
		return &models.GpuNode{Name: name, ID: ocid}, nil
	}
	origReset := mcpSoftResetFn
	defer func() { mcpSoftResetFn = origReset }()
	mcpSoftResetFn = func(_ context.Context, _ *models.GpuNode, env models.Environment, _ logging.Logger) error {
		gotActionEnv = env
		return nil
	}

	rec := &recorder{}
	clientSess := newTestPair(ctx, t, stubLoader{}, rec, func(c *config.Config) {
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
	mcpTerminateFn = func(_ context.Context, _ *models.GpuNode, env models.Environment, _ logging.Logger) error {
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
	origPoolResolve := mcpResolveGpuPoolFn
	defer func() { mcpResolveGpuPoolFn = origPoolResolve }()
	mcpResolveGpuPoolFn = func(_ context.Context, _ *Server, env models.Environment, name string) (*models.GpuPool, error) {
		gotResolverEnv = env
		return &models.GpuPool{Name: name, ID: "ocid1.pool", Size: 4}, nil
	}
	origInc := mcpIncreasePoolSizeFn
	defer func() { mcpIncreasePoolSizeFn = origInc }()
	mcpIncreasePoolSizeFn = func(_ context.Context, _ *models.GpuPool, env models.Environment, _ logging.Logger) error {
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

	rec := &recorder{}
	clientSess := newTestPair(ctx, t, stubLoader{}, rec)

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
	msgs := waitForMsgs(t, rec)
	body, _ := msgs[0].Data.(string)
	assert.Contains(t, body, "delete dac/dac-x: OK")
}

func TestIntegration_MutationTool_PerformErrorPropagates(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)

	orig := mcpDrainNodeFn
	defer func() { mcpDrainNodeFn = orig }()
	mcpDrainNodeFn = func(context.Context, string, string, string) error {
		return errors.New("pods stuck terminating")
	}

	rec := &recorder{}
	clientSess := newTestPair(ctx, t, stubLoader{}, rec)

	res, err := clientSess.CallTool(ctx, &sdk.CallToolParams{
		Name:      "drain_node",
		Arguments: map[string]any{"node": "node-a", "confirm": true},
	})
	require.NoError(t, err, "tool call should not error at transport level")
	require.NotNil(t, res)
	assert.True(t, res.IsError)

	// Error notification carries the underlying message.
	msgs := waitForMsgs(t, rec)
	var found bool
	for _, m := range msgs {
		body, _ := m.Data.(string)
		if strings.Contains(body, "pods stuck terminating") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected error notification to mention the underlying cause; got %+v", msgs)
	}
}

func TestIntegration_MutationTools_RegisteredInListTools(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)

	rec := &recorder{}
	clientSess := newTestPair(ctx, t, stubLoader{}, rec)

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
	} {
		assert.True(t, got[name], "tools/list missing mutation tool %q", name)
	}
}
