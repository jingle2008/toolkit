package mcp

import (
	"context"
	"testing"
	"time"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/pkg/models"
)

// itemLoader is stubLoader with one DAC and an endpoint name two
// namespaces share.
type itemLoader struct{ stubLoader }

func (itemLoader) LoadDedicatedAIClusters(context.Context, string, models.Environment) (map[string][]models.DedicatedAICluster, error) {
	return map[string][]models.DedicatedAICluster{"t1": {{Name: "dac-a", TenantID: "t1"}}}, nil
}

func (itemLoader) LoadInferenceServices(context.Context, string, models.Environment) (map[string][]models.InferenceService, error) {
	return map[string][]models.InferenceService{
		"ns-a": {{Name: "svc", Namespace: "ns-a"}, {Name: "other", Namespace: "ns-a"}},
		"ns-b": {{Name: "svc", Namespace: "ns-b"}},
	}, nil
}

func getItem(t *testing.T, args map[string]any) *sdk.CallToolResult {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	res, err := newTestPair(ctx, t, itemLoader{}).CallTool(ctx, &sdk.CallToolParams{Name: "get_item", Arguments: args})
	require.NoError(t, err)
	return res
}

func TestGetItem_ExactName(t *testing.T) {
	t.Parallel()
	res := getItem(t, map[string]any{"category": "dac", "name": "dac-a"})
	require.False(t, res.IsError, resultText(t, res))
	sc := structured(t, res)
	assert.InDelta(t, 1, sc["count"], 0)
	assert.Equal(t, "dac-a", sc["items"].([]any)[0].(map[string]any)["name"]) //nolint:forcetypeassert // JSON

	// A name two namespaces share: both rows, told apart by namespace.
	sc = structured(t, getItem(t, map[string]any{"category": "inferenceservice", "name": "svc"}))
	assert.InDelta(t, 2, sc["count"], 0)

	res = getItem(t, map[string]any{"category": "dac", "name": "dac"})
	require.True(t, res.IsError, "a near-miss is not a match")
	assert.Contains(t, resultText(t, res), `no DedicatedAICluster named "dac"`)
}

// Every category but Alias can be got by name, through its list tool:
// listAll's switch must not fall through for any of them.
func TestGetItem_EveryCategory(t *testing.T) {
	t.Parallel()
	for _, c := range domain.Categories {
		if c == domain.Alias {
			continue
		}
		res := getItem(t, map[string]any{"category": c.String(), "name": "no-such-row"})
		require.True(t, res.IsError)
		assert.Contains(t, resultText(t, res), "no "+c.String()+" named", c.String())
	}
	res := getItem(t, map[string]any{"category": "alias", "name": "t"})
	assert.Contains(t, resultText(t, res), "no rows to get by name")
}
