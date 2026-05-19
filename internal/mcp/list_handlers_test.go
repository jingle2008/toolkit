package mcp

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jingle2008/toolkit/pkg/models"
)

// Each list_* handler shares the same shape: load from the loader,
// filter, return jsonResult. The stubLoader returns empty data, so
// these tests don't validate content — they verify the handler path
// is reachable (no IsError) and the trailing helpers
// (listFlatResult / listGroupedResult) are exercised.

// callList drives one list_* tool call through the existing newTestPair
// scaffold, asserts the call succeeded, and returns the result. Keeps
// each test below to the call name and assertions specific to it.
func callList(t *testing.T, name string, args map[string]any) *sdk.CallToolResult {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)

	rec := &recorder{}
	sess := newTestPair(ctx, t, stubLoader{}, rec)

	res, err := sess.CallTool(ctx, &sdk.CallToolParams{Name: name, Arguments: args})
	require.NoError(t, err, "tools/call: %s", name)
	require.NotNil(t, res)
	assert.False(t, res.IsError, "%s should not error with stubLoader: %+v", name, res)
	return res
}

func TestList_Tenants(t *testing.T) {
	t.Parallel()
	callList(t, "list_tenants", nil)
}

func TestList_Environments(t *testing.T) {
	t.Parallel()
	callList(t, "list_environments", nil)
}

func TestList_ServiceTenancies(t *testing.T) {
	t.Parallel()
	callList(t, "list_service_tenancies", nil)
}

func TestList_GpuNodes(t *testing.T) {
	t.Parallel()
	callList(t, "list_gpu_nodes", nil)
}

// fakeGpuNodeLoader returns scripted data from LoadGpuNodes while
// delegating every other Loader method to the empty stubLoader. Used
// to exercise the typed wrapper path with real items.
type fakeGpuNodeLoader struct {
	stubLoader
	nodes map[string][]models.GpuNode
}

func (f *fakeGpuNodeLoader) LoadGpuNodes(context.Context, string, models.Environment) (map[string][]models.GpuNode, error) {
	return f.nodes, nil
}

// TestList_GpuNodes_TypedWrapperShape pins the wire shape of the
// grouped tool: each item is the flat union of GpuNode's JSON fields
// plus a top-level `pool` key. This is the regression bait for the
// "is it flattened?" review feedback that prompted the gpuNodeWithPool
// wrapper to replace map[string]any.
func TestList_GpuNodes_TypedWrapperShape(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)

	loader := &fakeGpuNodeLoader{
		nodes: map[string][]models.GpuNode{
			"pool-a": {{Name: "node-1", IsReady: true}},
		},
	}
	rec := &recorder{}
	sess := newTestPair(ctx, t, loader, rec)

	res, err := sess.CallTool(ctx, &sdk.CallToolParams{Name: "list_gpu_nodes"})
	require.NoError(t, err)
	require.False(t, res.IsError)

	// Round-trip StructuredContent through JSON so we can interrogate
	// it without depending on a typed Go decode — that's exactly the
	// view an MCP client gets on the wire.
	scBytes, err := json.Marshal(res.StructuredContent)
	require.NoError(t, err)
	var env struct {
		Items []map[string]any `json:"items"`
		Count int              `json:"count"`
	}
	require.NoError(t, json.Unmarshal(scBytes, &env))
	require.Equal(t, 1, env.Count)
	require.Len(t, env.Items, 1)
	item := env.Items[0]
	assert.Equal(t, "pool-a", item["pool"], "group key should be flattened to top-level pool field")
	assert.Equal(t, "node-1", item["name"], "GpuNode.Name should be flat at top level (embedded, not under nodes:)")
	assert.Equal(t, true, item["isReady"], "GpuNode.IsReady should be flat at top level")
}

func TestList_DACs(t *testing.T) {
	t.Parallel()
	callList(t, "list_dacs", nil)
}

func TestList_ModelArtifacts(t *testing.T) {
	t.Parallel()
	callList(t, "list_model_artifacts", nil)
}

func TestList_Definitions_AllKinds(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"limit", "console_property", "property"} {
		callList(t, "list_definitions", map[string]any{"kind": kind})
	}
}

func TestList_TenancyOverrides_AllKinds(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"limit", "console_property", "property"} {
		callList(t, "list_tenancy_overrides", map[string]any{"kind": kind})
	}
}

func TestList_RegionalOverrides_AllKinds(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"limit", "console_property", "property"} {
		callList(t, "list_regional_overrides", map[string]any{"kind": kind})
	}
}

func TestList_Definitions_UnknownKind(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)

	rec := &recorder{}
	sess := newTestPair(ctx, t, stubLoader{}, rec)

	res, err := sess.CallTool(ctx, &sdk.CallToolParams{
		Name:      "list_definitions",
		Arguments: map[string]any{"kind": "bogus"},
	})
	require.NoError(t, err)
	require.NotNil(t, res)
	assert.True(t, res.IsError, "unknown kind must surface as tool error")
}

func TestList_Aliases(t *testing.T) {
	t.Parallel()
	callList(t, "list_aliases", nil)
}
