package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"
	"time"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jingle2008/toolkit/internal/infra/loader"
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

// assertGroupedItem drives one list_* tool call against a loader
// scripted to return a single item, round-trips StructuredContent
// through JSON, and hands the lone item to the supplied closure.
// Shared by both flat and wrapper-shape tests so the boilerplate
// (round-trip, count/length assertions) stays in one place.
func assertGroupedItem(
	t *testing.T,
	toolName string,
	ld loader.Loader,
	check func(t *testing.T, item map[string]any),
) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	rec := &recorder{}
	sess := newTestPair(ctx, t, ld, rec)

	res, err := sess.CallTool(ctx, &sdk.CallToolParams{Name: toolName})
	require.NoError(t, err)
	require.False(t, res.IsError)

	// Round-trip StructuredContent through JSON so we interrogate
	// the same view an MCP client gets on the wire (no typed decode).
	scBytes, err := json.Marshal(res.StructuredContent)
	require.NoError(t, err)
	var env struct {
		Items []map[string]any `json:"items"`
		Count int              `json:"count"`
	}
	require.NoError(t, json.Unmarshal(scBytes, &env))
	require.Equal(t, 1, env.Count)
	require.Len(t, env.Items, 1)
	check(t, env.Items[0])
}

// assertGroupedWrapperShape is for tools that DO wrap the value with
// an injected group key (e.g. list_dacs adds `tenant`). Verifies the
// injected key is present alongside the embedded fields.
func assertGroupedWrapperShape(
	t *testing.T,
	toolName string,
	ld loader.Loader,
	expectGroupKey string,
	expectGroupValue string,
	extra func(t *testing.T, item map[string]any),
) {
	t.Helper()
	assertGroupedItem(t, toolName, ld, func(t *testing.T, item map[string]any) {
		assert.Equal(t, expectGroupValue, item[expectGroupKey],
			"group key %q should be flattened to top level", expectGroupKey)
		extra(t, item)
	})
}

// assertGroupedFlatShape is for tools that DON'T wrap (the group key
// is already a value field — poolName on GpuNode, model_name on
// ModelArtifact). The closure does its own assertions including
// checking the redundant key is absent.
func assertGroupedFlatShape(
	t *testing.T,
	toolName string,
	ld loader.Loader,
	check func(t *testing.T, item map[string]any),
) {
	t.Helper()
	assertGroupedItem(t, toolName, ld, check)
}

// TestList_GpuNodes_FlatShape pins the wire shape: each item is a
// GpuNode object with the originating pool already on it as
// `poolName`. No wrapper, no separate `pool` field — the loader
// always sets node.NodePool to the same value as the map key, so
// injection would just duplicate. Regression bait against
// accidentally re-wrapping in the future.
func TestList_GpuNodes_FlatShape(t *testing.T) {
	t.Parallel()
	loader := &fakeGpuNodeLoader{
		nodes: map[string][]models.GpuNode{
			"pool-a": {{Name: "node-1", NodePool: "pool-a", IsReady: true}},
		},
	}
	assertGroupedFlatShape(t, "list_gpu_nodes", loader,
		func(t *testing.T, item map[string]any) {
			assert.Equal(t, "pool-a", item["poolName"], "originating pool should come through as poolName")
			assert.Equal(t, "node-1", item["name"])
			assert.Equal(t, true, item["isReady"])
			_, hasPool := item["pool"]
			assert.False(t, hasPool, "redundant `pool` field should not be added (poolName carries it)")
		})
}

// fakeDACLoader returns scripted DACs from LoadDedicatedAIClusters; every
// other method delegates to stubLoader. Mirrors fakeGpuNodeLoader.
type fakeDACLoader struct {
	stubLoader
	dacs map[string][]models.DedicatedAICluster
}

func (f *fakeDACLoader) LoadDedicatedAIClusters(context.Context, string, models.Environment) (map[string][]models.DedicatedAICluster, error) {
	return f.dacs, nil
}

// fakeModelArtifactLoader returns scripted artifacts from LoadDataset
// (the only path artifact data flows through). All other methods come
// from stubLoader.
type fakeModelArtifactLoader struct {
	stubLoader
	artifacts map[string][]models.ModelArtifact
}

func (f *fakeModelArtifactLoader) LoadDataset(context.Context, string, models.Environment) (*models.Dataset, error) {
	return &models.Dataset{ModelArtifactMap: f.artifacts}, nil
}

// TestList_DACs_TypedWrapperShape — sibling of the GpuNode test. Same
// regression bait applied to the second grouped tool: any future
// refactor that re-mapifies the dacWithTenant wrapper would break
// the flat-top-level contract this test pins.
func TestList_DACs_TypedWrapperShape(t *testing.T) {
	t.Parallel()
	loader := &fakeDACLoader{
		dacs: map[string][]models.DedicatedAICluster{
			"acme": {{Name: "dac-1", Status: "READY"}},
		},
	}
	assertGroupedWrapperShape(t, "list_dacs", loader, "tenant", "acme",
		func(t *testing.T, item map[string]any) {
			assert.Equal(t, "dac-1", item["name"], "DAC.Name should be flat at top level")
			assert.Equal(t, "READY", item["status"])
		})
}

// TestList_ModelArtifacts_FlatShape pins the wire shape: each item
// is a ModelArtifact with the originating base model already on it
// as `model_name`. No `model` field is added — the loader writes
// ModelName from the same Terraform key used as the map key.
func TestList_ModelArtifacts_FlatShape(t *testing.T) {
	t.Parallel()
	loader := &fakeModelArtifactLoader{
		artifacts: map[string][]models.ModelArtifact{
			"cohere.command-r": {{Name: "artifact-1", ModelName: "cohere.command-r", TensorRTVersion: "9.2"}},
		},
	}
	assertGroupedFlatShape(t, "list_model_artifacts", loader,
		func(t *testing.T, item map[string]any) {
			assert.Equal(t, "cohere.command-r", item["model_name"], "originating base model should come through as model_name")
			assert.Equal(t, "artifact-1", item["name"])
			assert.Equal(t, "9.2", item["tensorrt_version"])
			_, hasModel := item["model"]
			assert.False(t, hasModel, "redundant `model` field should not be added (model_name carries it)")
		})
}

// TestList_GpuNodes_LimitCapsAcrossGroups pins the limit semantic
// for MCP grouped tools: cap is across the whole flattened result,
// not per group. Filter happens before limit (the only ordering
// that makes sense — see CHANGELOG).
func TestList_GpuNodes_LimitCapsAcrossGroups(t *testing.T) {
	t.Parallel()
	loader := &fakeGpuNodeLoader{
		nodes: map[string][]models.GpuNode{
			"pool-a": {{Name: "a1", NodePool: "pool-a"}, {Name: "a2", NodePool: "pool-a"}},
			"pool-b": {{Name: "b1", NodePool: "pool-b"}, {Name: "b2", NodePool: "pool-b"}},
		},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	rec := &recorder{}
	sess := newTestPair(ctx, t, loader, rec)

	res, err := sess.CallTool(ctx, &sdk.CallToolParams{
		Name:      "list_gpu_nodes",
		Arguments: map[string]any{"limit": 3},
	})
	require.NoError(t, err)
	require.False(t, res.IsError)

	scBytes, err := json.Marshal(res.StructuredContent)
	require.NoError(t, err)
	var env struct {
		Items []map[string]any `json:"items"`
		Count int              `json:"count"`
	}
	require.NoError(t, json.Unmarshal(scBytes, &env))
	assert.Equal(t, 3, env.Count, "limit=3 should yield 3 items across 4 flattened")
	require.Len(t, env.Items, 3)
	assert.Equal(t, "a1", env.Items[0]["name"])
	assert.Equal(t, "a2", env.Items[1]["name"])
	assert.Equal(t, "b1", env.Items[2]["name"], "limit should spill into next group's items, not skip the group")
}

// TestGroupedWrapper_NoDuplicateGroupKey is the sentinel guard
// against a future model gaining a `tenant` JSON-tagged field that
// would silently shadow (or duplicate) dacWithTenant's group-key
// field on the wire. (GpuNode/ModelArtifact dropped their wrappers
// — see TestList_*_FlatShape — because their group key is already
// a value field; only DAC still needs a wrapper.)
func TestGroupedWrapper_NoDuplicateGroupKey(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		marshal  func() ([]byte, error)
		groupKey string
	}{
		{
			"dacWithTenant",
			func() ([]byte, error) {
				return json.Marshal(dacWithTenant{Tenant: "t", DedicatedAICluster: models.DedicatedAICluster{Name: "d"}})
			},
			"tenant",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			raw, err := tc.marshal()
			require.NoError(t, err)
			// json.Marshal emits each key at most once; this catches the
			// case where the embedded model gains a clashing JSON tag
			// (encoding/json would silently drop one). Counts the
			// `"<key>":` occurrences across the whole document.
			needle := []byte(`"` + tc.groupKey + `":`)
			n := bytes.Count(raw, needle)
			assert.Equal(t, 1, n, "%s should appear exactly once in %s, got %d (raw=%s)", needle, tc.name, n, raw)
		})
	}
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
