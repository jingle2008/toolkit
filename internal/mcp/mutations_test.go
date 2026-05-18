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
	clientSess := newTestPair(t, ctx, stubLoader{}, rec)

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
	msgs := waitForMsgs(t, rec, 1)
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
	clientSess := newTestPair(t, ctx, stubLoader{}, rec)

	res, err := clientSess.CallTool(ctx, &sdk.CallToolParams{
		Name:      "cordon_node",
		Arguments: map[string]any{"node": "node-a", "confirm": true},
	})
	require.NoError(t, err)
	require.NotNil(t, res)
	assert.False(t, res.IsError, "expected success with confirm=true")
	assert.Equal(t, "node-a", gotNode)
	assert.True(t, gotWant, "cordon_node should pass want=true")

	// Response body is the {status, action, kind, target} envelope.
	text := res.Content[0].(*sdk.TextContent).Text
	var env struct {
		Items struct {
			Status string `json:"status"`
			Action string `json:"action"`
			Target string `json:"target"`
		} `json:"items"`
	}
	require.NoError(t, json.Unmarshal([]byte(text), &env))
	assert.Equal(t, "OK", env.Items.Status)
	assert.Equal(t, "cordon", env.Items.Action)
	assert.Equal(t, "node-a", env.Items.Target)

	// Info notification on success.
	msgs := waitForMsgs(t, rec, 1)
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
	clientSess := newTestPair(t, ctx, stubLoader{}, rec)

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
	clientSess := newTestPair(t, ctx, stubLoader{}, rec)

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

func TestIntegration_MutationTool_PerformErrorPropagates(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)

	orig := mcpDrainNodeFn
	defer func() { mcpDrainNodeFn = orig }()
	mcpDrainNodeFn = func(context.Context, string, string, string) error {
		return errors.New("pods stuck terminating")
	}

	rec := &recorder{}
	clientSess := newTestPair(t, ctx, stubLoader{}, rec)

	res, err := clientSess.CallTool(ctx, &sdk.CallToolParams{
		Name:      "drain_node",
		Arguments: map[string]any{"node": "node-a", "confirm": true},
	})
	require.NoError(t, err, "tool call should not error at transport level")
	require.NotNil(t, res)
	assert.True(t, res.IsError)

	// Error notification carries the underlying message.
	msgs := waitForMsgs(t, rec, 1)
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
	clientSess := newTestPair(t, ctx, stubLoader{}, rec)

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
