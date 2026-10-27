package mcp

import (
	"context"
	"testing"
	"time"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jingle2008/toolkit/pkg/models"
)

// agedLoader serves one row of each category whose items carry an age,
// created three days ago, with no Age of its own.
type agedLoader struct {
	stubLoader
	created time.Time
}

func (l agedLoader) LoadGPUNodesByPool(context.Context, string, models.Environment) (map[string][]models.GPUNode, error) {
	return map[string][]models.GPUNode{"pool": {{Name: "node-1", NodePool: "pool", CreatedAt: l.created}}}, nil
}

func (l agedLoader) LoadGPUWorkloadsByNode(context.Context, string, models.Environment) (map[string][]models.GPUWorkload, error) {
	return map[string][]models.GPUWorkload{"node-1": {{Name: "pod-1", Node: "node-1", CreatedAt: l.created}}}, nil
}

func (l agedLoader) LoadDedicatedAIClusters(context.Context, string, models.Environment) (map[string][]models.DedicatedAICluster, error) {
	return map[string][]models.DedicatedAICluster{"t": {{Name: "dac-1", TenantID: "t", CreatedAt: l.created}}}, nil
}

func (l agedLoader) LoadInferenceServices(context.Context, string, models.Environment) (map[string][]models.InferenceService, error) {
	return map[string][]models.InferenceService{"ns": {{Name: "svc-1", Namespace: "ns", CreatedAt: l.created}}}, nil
}

// Each list tool's items carry age, derived from createdAt as they are
// written, and pass the SDK's output-schema check: real rows, so the
// validation actually runs.
func TestListTools_AgeDerivedFromCreatedAt(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	created := time.Now().Add(-3*24*time.Hour - time.Hour).UTC().Truncate(time.Second)
	sess := newTestPair(ctx, t, agedLoader{created: created})

	for _, tool := range []string{"list_gpu_nodes", "list_gpu_workloads", "list_dacs", "list_inference_services"} {
		res, err := sess.CallTool(ctx, &sdk.CallToolParams{Name: tool, Arguments: map[string]any{}})
		require.NoError(t, err, tool)
		require.False(t, res.IsError, "%s: %s", tool, resultText(t, res))
		items, ok := structured(t, res)["items"].([]any)
		require.Truef(t, ok, "%s items", tool)
		require.Lenf(t, items, 1, tool)
		item, ok := items[0].(map[string]any)
		require.True(t, ok)
		assert.Equalf(t, "3d", item["age"], tool)
		assert.Equalf(t, created.Format(time.RFC3339), item["createdAt"], tool)
	}
}
