package mcp

import (
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

// assertGroupedFlatShape is for tools whose group key is already a
// value field on the model (poolName on GpuNode, tenantId on DAC,
// model_name on ModelArtifact — none use a wrapper). The closure
// does its own assertions including checking the redundant key is
// absent.
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

// TestList_DACs_FlatShape pins the wire shape: each item is a DAC
// object with the originating tenant already on it as `tenantId`.
// The loader keys by dac.TenantID
// (internal/infra/k8s/dac.go:157), so the map key always equals
// that flat field — no wrapper, no separate `tenant` field. Regression
// bait against accidentally re-wrapping in the future.
func TestList_DACs_FlatShape(t *testing.T) {
	t.Parallel()
	loader := &fakeDACLoader{
		dacs: map[string][]models.DedicatedAICluster{
			"acme": {{Name: "dac-1", Status: "READY", TenantID: "acme"}},
		},
	}
	assertGroupedFlatShape(t, "list_dacs", loader,
		func(t *testing.T, item map[string]any) {
			assert.Equal(t, "acme", item["tenantId"], "originating tenant should come through as tenantId")
			assert.Equal(t, "dac-1", item["name"])
			assert.Equal(t, "READY", item["status"])
			_, hasTenant := item["tenant"]
			assert.False(t, hasTenant, "redundant `tenant` field should not be added (tenantId carries it)")
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

// TestList_GpuNodes_Limit_ZeroAndOverflow pins the same kubectl-style
// no-cap semantics the CLI side asserts in TestWriteSlice_Limit:
// limit=0 is unlimited; limit > len is a no-op. Doing this against a
// real CallTool ensures the contract holds at the MCP wire layer
// even if TruncateSlice is later reimplemented per-handler.
func TestList_GpuNodes_Limit_ZeroAndOverflow(t *testing.T) {
	t.Parallel()
	loader := &fakeGpuNodeLoader{
		nodes: map[string][]models.GpuNode{
			"pool-a": {{Name: "a1", NodePool: "pool-a"}, {Name: "a2", NodePool: "pool-a"}},
			"pool-b": {{Name: "b1", NodePool: "pool-b"}, {Name: "b2", NodePool: "pool-b"}},
		},
	}
	count := func(t *testing.T, args map[string]any) int {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		t.Cleanup(cancel)
		rec := &recorder{}
		sess := newTestPair(ctx, t, loader, rec)

		res, err := sess.CallTool(ctx, &sdk.CallToolParams{Name: "list_gpu_nodes", Arguments: args})
		require.NoError(t, err)
		require.False(t, res.IsError)
		scBytes, err := json.Marshal(res.StructuredContent)
		require.NoError(t, err)
		var env struct {
			Count int `json:"count"`
		}
		require.NoError(t, json.Unmarshal(scBytes, &env))
		return env.Count
	}
	assert.Equal(t, 4, count(t, map[string]any{"limit": 0}), "limit=0 should be unlimited (all 4 items)")
	assert.Equal(t, 4, count(t, map[string]any{"limit": 99}), "limit > len should be a no-op")
	assert.Equal(t, 4, count(t, nil), "omitting limit should default to unlimited")
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

// fakeImportedModelLoader serves scripted ImportedModels through
// LoadImportedModels; everything else inherits stubLoader's empty
// defaults.
type fakeImportedModelLoader struct {
	stubLoader
	items []models.ImportedModel
}

func (f *fakeImportedModelLoader) LoadImportedModels(context.Context, string, models.Environment) ([]models.ImportedModel, error) {
	return f.items, nil
}

// TestList_ImportedModels_FlatShape pins the wire shape: BaseModel
// fields are flattened at the top level alongside `namespace`,
// `tenantId`, and `source`. Regression bait against an accidental
// nesting refactor (e.g. wrapping BaseModel under a `model` key).
func TestList_ImportedModels_FlatShape(t *testing.T) {
	t.Parallel()
	loader := &fakeImportedModelLoader{
		items: []models.ImportedModel{
			{
				BaseModel: models.BaseModel{
					Name: "import-a", DisplayName: "Import A", Vendor: "acme", Version: "v1", Status: "Ready",
					StorageURI: "oci://n/tenancy/b/bucket/o/path",
				},
				Namespace: "team-x",
			},
		},
	}
	assertGroupedFlatShape(t, "list_imported_models", loader,
		func(t *testing.T, item map[string]any) {
			// Embedded BaseModel fields surface at the top level.
			assert.Equal(t, "import-a", item["name"])
			assert.Equal(t, "Import A", item["displayName"])
			assert.Equal(t, "acme", item["vendor"])
			assert.Equal(t, "oci://n/tenancy/b/bucket/o/path", item["storageUri"], "storageUri must come through from BaseModel")
			// ImportedModel-specific identity fields sit alongside.
			assert.Equal(t, "team-x", item["namespace"])
			// Source field was removed — derivable from namespace.
			_, hasSource := item["source"]
			assert.False(t, hasSource, "source field was dropped; consumers derive from namespace (empty = cluster-scoped)")
			// Cluster-scoped indicator absent when source is namespaced.
			_, hasTenantID := item["tenantId"]
			assert.False(t, hasTenantID, "tenantId should be omitempty when not set")
			// Nothing wrapped under `model`.
			_, hasModelKey := item["model"]
			assert.False(t, hasModelKey, "BaseModel fields must be flat at the top level, not nested under `model`")
		})
}

func TestList_Aliases(t *testing.T) {
	t.Parallel()
	callList(t, "list_aliases", nil)
}

// TestList_InputSchemas_ExposeLimit pins that every list_* tool that
// accepts limit publishes it in the tool's published InputSchema. The
// kindInput tools embed listInput; this guards against a future
// regression where the embedded Limit field stops surfacing through
// the SDK's reflection-based schema generator. The included
// description from the `jsonschema:` tag (see server.go listInput) is
// what an LLM agent actually reads to decide whether to pass a value.
func TestList_InputSchemas_ExposeLimit(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	rec := &recorder{}
	sess := newTestPair(ctx, t, stubLoader{}, rec)

	listRes, err := sess.ListTools(ctx, &sdk.ListToolsParams{})
	require.NoError(t, err)

	// list_tenants uses listInput directly; list_definitions /
	// list_tenancy_overrides / list_regional_overrides use kindInput
	// (which embeds listInput). All four must expose limit.
	wantLimit := map[string]bool{
		"list_tenants":            false,
		"list_definitions":        false,
		"list_tenancy_overrides":  false,
		"list_regional_overrides": false,
	}
	for _, tool := range listRes.Tools {
		if _, want := wantLimit[tool.Name]; !want {
			continue
		}
		// InputSchema is map[string]any on the wire; remarshal to walk
		// the standard JSON-schema shape instead of typing on whatever
		// the SDK happens to surface client-side.
		raw, err := json.Marshal(tool.InputSchema)
		require.NoError(t, err, "%s schema marshal", tool.Name)
		var schema struct {
			Properties map[string]struct {
				Type        string `json:"type"`
				Description string `json:"description"`
			} `json:"properties"`
		}
		require.NoError(t, json.Unmarshal(raw, &schema), "%s schema decode", tool.Name)
		limit, ok := schema.Properties["limit"]
		require.True(t, ok, "%s should publish `limit` in InputSchema, got: %s", tool.Name, raw)
		assert.Equal(t, "integer", limit.Type, "%s.limit should be integer-typed", tool.Name)
		assert.NotEmpty(t, limit.Description, "%s.limit should carry the jsonschema description (so agents see it)", tool.Name)
		wantLimit[tool.Name] = true
	}
	for name, found := range wantLimit {
		assert.True(t, found, "expected %s in ListTools response", name)
	}
}
