package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/internal/infra/loader"
	"github.com/jingle2008/toolkit/internal/relations"
	"github.com/jingle2008/toolkit/pkg/models"
)

// faultyTools maps each faulty-capable category to its list tool.
var faultyTools = map[domain.Category]string{
	domain.BaseModel:          "list_base_models",
	domain.Tenant:             "list_tenants",
	domain.GPUPool:            "list_gpu_pools",
	domain.GPUNode:            "list_gpu_nodes",
	domain.GPUWorkload:        "list_gpu_workloads",
	domain.ServingRuntime:     "list_serving_runtimes",
	domain.DedicatedAICluster: "list_dacs",
	domain.InferenceService:   "list_inference_services",
}

// `faulty` is in the schema of exactly the faulty categories' tools.
func TestListTools_FaultyInputOnFaultyCategories(t *testing.T) {
	t.Parallel()
	require.Len(t, faultyTools, len(relations.FaultyCategories()))
	want := map[string]bool{}
	for _, c := range relations.FaultyCategories() {
		name, ok := faultyTools[c]
		require.True(t, ok, "no list tool mapped for %s", c)
		want[name] = true
	}
	for _, tool := range listedTools(t) {
		raw, err := json.Marshal(tool.InputSchema)
		require.NoError(t, err)
		var schema struct {
			Properties map[string]any `json:"properties"`
		}
		require.NoError(t, json.Unmarshal(raw, &schema))
		_, has := schema.Properties["faulty"]
		assert.Equal(t, want[tool.Name], has, "%s faulty input", tool.Name)
	}
}

func stubFaultyDataset(t *testing.T, ds *models.Dataset, warnings []string, err error) {
	t.Helper()
	orig := mcpFaultyDatasetFn
	t.Cleanup(func() { mcpFaultyDatasetFn = orig })
	mcpFaultyDatasetFn = func(context.Context, loader.Composite, string, string, models.Environment, domain.Category) (*models.Dataset, []string, error) {
		return ds, warnings, err
	}
}

// orphanDataset has one ready endpoint whose DAC exists and one whose
// DAC is gone.
func orphanDataset() *models.Dataset {
	return &models.Dataset{
		DedicatedAIClusterMap: map[string][]models.DedicatedAICluster{"acme": {{Name: "amaaaaaa-dac"}}},
		InferenceServiceMap: map[string][]models.InferenceService{
			"amaaaaaa-dac":  {{Name: "isvc-ok", Namespace: "amaaaaaa-dac", Type: models.InferenceServiceDedicated, Ready: "True"}},
			"amaaaaaa-gone": {{Name: "isvc-orphan", Namespace: "amaaaaaa-gone", Type: models.InferenceServiceDedicated, Ready: "True"}},
		},
	}
}

func callStructured(t *testing.T, name string, args map[string]any) (*sdk.CallToolResult, map[string]any) {
	t.Helper()
	res := callTool(t, name, args)
	if res.IsError {
		return res, nil
	}
	return res, structured(t, res)
}

// faulty=true returns the rows Ctrl+Z shows: here the orphan, whose own
// state is fine, and the load's warnings.
//
//nolint:paralleltest // replaces mcpFaultyDatasetFn
func TestListInferenceServices_Faulty(t *testing.T) {
	stubFaultyDataset(t, orphanDataset(), []string{"some GPU pool sources failed to load"}, nil)
	_, sc := callStructured(t, "list_inference_services", map[string]any{"faulty": true})
	items, ok := sc["items"].([]any)
	require.True(t, ok)
	require.Len(t, items, 1)
	assert.Equal(t, "isvc-orphan", items[0].(map[string]any)["name"]) //nolint:forcetypeassert // JSON object
	assert.Equal(t, []any{"some GPU pool sources failed to load"}, sc["warnings"])
}

//nolint:paralleltest // replaces mcpFaultyDatasetFn
func TestListGPUPools_FaultyLoadErrorIsToolError(t *testing.T) {
	stubFaultyDataset(t, nil, nil, errors.New("live GPU pool sizes unavailable"))
	res, _ := callStructured(t, "list_gpu_pools", map[string]any{"faulty": true})
	require.True(t, res.IsError)
	assert.Contains(t, resultText(t, res), "live GPU pool sizes unavailable")
}

//nolint:paralleltest // replaces mcpIntegrityDatasetFn
func TestCheckIntegrity(t *testing.T) {
	orig := mcpIntegrityDatasetFn
	t.Cleanup(func() { mcpIntegrityDatasetFn = orig })
	mcpIntegrityDatasetFn = func(context.Context, loader.Composite, string, string, models.Environment) (*models.Dataset, string, error) {
		return orphanDataset(), "", nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	res, err := newTestPair(ctx, t, stubLoader{}).CallTool(ctx, &sdk.CallToolParams{Name: "check_integrity"})
	require.NoError(t, err)
	require.False(t, res.IsError, resultText(t, res))
	byName := map[string]map[string]any{}
	for _, it := range structured(t, res)["items"].([]any) { //nolint:forcetypeassert // listResult items
		m := it.(map[string]any)       //nolint:forcetypeassert // JSON object
		byName[m["name"].(string)] = m //nolint:forcetypeassert // name is a string
	}

	isvc := byName["InferenceService → DedicatedAICluster"]
	require.NotNil(t, isvc)
	assert.Equal(t, "FAIL", isvc["status"])
	assert.Equal(t, "InferenceService", isvc["child"])
	assert.Equal(t, []any{map[string]any{"row": "amaaaaaa-gone/isvc-orphan", "missing": "amaaaaaa-gone"}}, isvc["dangling"])

	tenancy := byName["tenancy resolution"]
	require.NotNil(t, tenancy)
	assert.NotContains(t, tenancy, "child", "the tenancy rule spans categories")
	assert.Equal(t, "Tenant", tenancy["parent"])
}
