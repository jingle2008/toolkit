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

// assertGroupedWrapperShape drives one grouped list_* tool call,
// round-trips StructuredContent through JSON, and runs the supplied
// extra-assertions closure against the lone item. Used by all three
// grouped-tool shape tests; factored to avoid `dupl` complaints
// while keeping each call site readable.
func assertGroupedWrapperShape(
	t *testing.T,
	toolName string,
	ld loader.Loader,
	expectGroupKey string,
	expectGroupValue string,
	extra func(t *testing.T, item map[string]any),
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
	item := env.Items[0]
	assert.Equal(t, expectGroupValue, item[expectGroupKey],
		"group key %q should be flattened to top level", expectGroupKey)
	extra(t, item)
}

// TestList_GpuNodes_TypedWrapperShape pins the wire shape of the
// grouped tool: each item is the flat union of GpuNode's JSON fields
// plus a top-level `pool` key. This is the regression bait for the
// "is it flattened?" review feedback that prompted the gpuNodeWithPool
// wrapper to replace map[string]any.
func TestList_GpuNodes_TypedWrapperShape(t *testing.T) {
	t.Parallel()
	loader := &fakeGpuNodeLoader{
		nodes: map[string][]models.GpuNode{
			"pool-a": {{Name: "node-1", IsReady: true}},
		},
	}
	assertGroupedWrapperShape(t, "list_gpu_nodes", loader, "pool", "pool-a",
		func(t *testing.T, item map[string]any) {
			assert.Equal(t, "node-1", item["name"], "GpuNode.Name should be flat at top level")
			assert.Equal(t, true, item["isReady"], "GpuNode.IsReady should be flat at top level")
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

// TestList_ModelArtifacts_TypedWrapperShape — same pattern for the
// third grouped tool. Completes the regression-bait set so any of
// the three wrappers regressing fails a test.
func TestList_ModelArtifacts_TypedWrapperShape(t *testing.T) {
	t.Parallel()
	loader := &fakeModelArtifactLoader{
		artifacts: map[string][]models.ModelArtifact{
			"cohere.command-r": {{Name: "artifact-1", TensorRTVersion: "9.2"}},
		},
	}
	assertGroupedWrapperShape(t, "list_model_artifacts", loader, "model", "cohere.command-r",
		func(t *testing.T, item map[string]any) {
			assert.Equal(t, "artifact-1", item["name"], "ModelArtifact.Name should be flat at top level")
			assert.Equal(t, "9.2", item["tensorrt_version"])
		})
}

// TestGroupedWrapper_NoDuplicateGroupKey is the sentinel guard against
// a future model gaining a `pool` / `tenant` / `model` JSON-tagged
// field that would silently shadow (or duplicate) the wrapper's
// group-key field on the wire. encoding/json picks the outer field
// in Go, but the spec around duplicate JSON keys is undefined; the
// test fails loudly the day a model adds such a field so we
// re-architect rather than ship ambiguous output.
func TestGroupedWrapper_NoDuplicateGroupKey(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		marshal  func() ([]byte, error)
		groupKey string
	}{
		{
			"gpuNodeWithPool",
			func() ([]byte, error) {
				return json.Marshal(gpuNodeWithPool{Pool: "p", GpuNode: models.GpuNode{Name: "n"}})
			},
			"pool",
		},
		{
			"dacWithTenant",
			func() ([]byte, error) {
				return json.Marshal(dacWithTenant{Tenant: "t", DedicatedAICluster: models.DedicatedAICluster{Name: "d"}})
			},
			"tenant",
		},
		{
			"modelArtifactWithModel",
			func() ([]byte, error) {
				return json.Marshal(modelArtifactWithModel{Model: "m", ModelArtifact: models.ModelArtifact{Name: "a"}})
			},
			"model",
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
