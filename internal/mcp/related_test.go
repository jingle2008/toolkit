package mcp

import (
	"context"
	"encoding/json"
	"slices"
	"testing"
	"time"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jingle2008/toolkit/pkg/models"
)

// relatedLoader scripts workloads, endpoints and a tenant so list_related
// has something to narrow; every other method comes from stubLoader.
type relatedLoader struct{ stubLoader }

func (relatedLoader) LoadGPUWorkloadsByNode(context.Context, string, models.Environment) (map[string][]models.GPUWorkload, error) {
	return map[string][]models.GPUWorkload{
		"node-a": {{Name: "pod-1", Node: "node-a", Namespace: "amaaaaaa-dac1", TenantID: "acme"}, {Name: "pod-2", Node: "node-a", Namespace: "amaaaaaa-dac2"}},
		"node-b": {{Name: "pod-3", Node: "node-b", Namespace: "amaaaaaa-dac1", TenantID: "acme", Runtime: "vllm"}},
	}, nil
}

func (relatedLoader) LoadInferenceServices(context.Context, string, models.Environment) (map[string][]models.InferenceService, error) {
	return map[string][]models.InferenceService{
		"amaaaaaa-dac1": {{Name: "isvc-1", Namespace: "amaaaaaa-dac1", Type: models.InferenceServiceDedicated, Model: "llama", Runtime: "vllm"}},
		"amaaaaaa-dac2": {{Name: "isvc-2", Namespace: "amaaaaaa-dac2", Type: models.InferenceServiceDedicated, Model: "llama", Runtime: "vllm", Legacy: true}},
	}, nil
}

func (relatedLoader) LoadTenancyOverrideGroup(context.Context, string, models.Environment) (models.TenancyOverrideGroup, error) {
	return models.TenancyOverrideGroup{Tenants: []models.Tenant{{Name: "Acme", ID: "ocid1.tenancy.oc1..acme"}}}, nil
}

type relatedOut struct {
	Items    []map[string]any `json:"items"`
	Warnings []string         `json:"warnings"`
}

func callRelated(t *testing.T, args map[string]any) (*sdk.CallToolResult, relatedOut) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	sess := newTestPair(ctx, t, relatedLoader{})
	res, err := sess.CallTool(ctx, &sdk.CallToolParams{Name: "list_related", Arguments: args})
	require.NoError(t, err)
	var out relatedOut
	if !res.IsError {
		raw, err := json.Marshal(res.StructuredContent)
		require.NoError(t, err)
		require.NoError(t, json.Unmarshal(raw, &out))
	}
	return res, out
}

func itemNames(out relatedOut) []string {
	names := make([]string, len(out.Items))
	for i, it := range out.Items {
		names[i], _ = it["name"].(string)
	}
	slices.Sort(names)
	return names
}

func TestListRelated(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		args map[string]any
		want []string
	}{
		{"dac pods across nodes", map[string]any{"category": "gw", "parent_category": "dac", "parent_name": "amaaaaaa-dac1"}, []string{"pod-1", "pod-3"}},
		{"tenant pods by resolved owner", map[string]any{"category": "gpuworkload", "parent_category": "tenant", "parent_name": "Acme"}, []string{"pod-1", "pod-3"}},
		{"runtime pods", map[string]any{"category": "gw", "parent_category": "sr", "parent_name": "vllm"}, []string{"pod-3"}},
		{"runtime endpoints skip legacy", map[string]any{"category": "is", "parent_category": "servingruntime", "parent_name": "vllm"}, []string{"isvc-1"}},
		{"model endpoints", map[string]any{"category": "is", "parent_category": "bm", "parent_name": "llama"}, []string{"isvc-1"}},
		{"with filter", map[string]any{"category": "gw", "parent_category": "dac", "parent_name": "amaaaaaa-dac1", "filter": "node-b"}, []string{"pod-3"}},
		{"with limit", map[string]any{"category": "gw", "parent_category": "dac", "parent_name": "amaaaaaa-dac1", "limit": 1}, []string{"pod-1"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			res, out := callRelated(t, tc.args)
			require.False(t, res.IsError, "%+v", res)
			assert.Equal(t, tc.want, itemNames(out))
		})
	}
}

func TestListRelated_EmptyWarns(t *testing.T) {
	t.Parallel()
	res, out := callRelated(t, map[string]any{"category": "gw", "parent_category": "dac", "parent_name": "nope"})
	require.False(t, res.IsError)
	assert.Empty(t, out.Items)
	require.NotEmpty(t, out.Warnings)
	assert.Contains(t, out.Warnings[len(out.Warnings)-1], `no GPUWorkload rows related to DedicatedAICluster "nope"`)
}

func TestListRelated_Errors(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		args map[string]any
		want string
	}{
		{
			"unrelated parent",
			map[string]any{"category": "gw", "parent_category": "gpupool", "parent_name": "p"},
			"GPUPool is not related to GPUWorkload; valid parent_category values: Tenant, GPUNode, DedicatedAICluster, ServingRuntime",
		},
		{"unscoped category", map[string]any{"category": "tenant", "parent_category": "dac", "parent_name": "x"}, "Tenant rows are not related"},
		{"unknown category", map[string]any{"category": "nope", "parent_category": "dac", "parent_name": "x"}, "category"},
		// The schema marks parent_name required; an empty one reaches the handler.
		{"missing name", map[string]any{"category": "gw", "parent_category": "dac"}, "missing properties"},
		{"empty name", map[string]any{"category": "gw", "parent_category": "dac", "parent_name": ""}, "parent_name is required"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			res, _ := callRelated(t, tc.args)
			require.True(t, res.IsError, "%+v", res)
			raw, _ := json.Marshal(res.Content)
			assert.Contains(t, string(raw), tc.want)
		})
	}
}

// The description lists every relation, so agents can see valid pairs.
func TestRelatedToolDescription_ListsRelations(t *testing.T) {
	t.Parallel()
	d := relatedToolDescription()
	assert.Contains(t, d, "GPUWorkload ← Tenant, GPUNode, DedicatedAICluster, ServingRuntime")
	assert.Contains(t, d, "InferenceService ← Tenant, DedicatedAICluster, BaseModel, ImportedModel, ServingRuntime")
}
