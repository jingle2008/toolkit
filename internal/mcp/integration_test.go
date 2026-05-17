package mcp

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jingle2008/toolkit/internal/config"
	"github.com/jingle2008/toolkit/pkg/infra/logging"
	"github.com/jingle2008/toolkit/pkg/models"
)

// stubLoader implements just enough of the loader.Loader interface to
// satisfy NewServer. The integration test only exercises list_aliases
// (which doesn't reach the loader at all), so every method returns an
// empty result; the methods exist only because Server's constructor
// stores a loader.Loader.
type stubLoader struct{}

func (stubLoader) LoadDataset(context.Context, string, models.Environment) (*models.Dataset, error) {
	return &models.Dataset{}, nil
}

func (stubLoader) LoadBaseModels(context.Context, string, models.Environment) ([]models.BaseModel, error) {
	return nil, nil
}

func (stubLoader) LoadGpuPools(context.Context, string, models.Environment) ([]models.GpuPool, error) {
	return nil, nil
}

func (stubLoader) LoadGpuNodes(context.Context, string, models.Environment) (map[string][]models.GpuNode, error) {
	return nil, nil
}

func (stubLoader) LoadDedicatedAIClusters(context.Context, string, models.Environment) (map[string][]models.DedicatedAICluster, error) {
	return nil, nil
}

func (stubLoader) LoadTenancyOverrideGroup(context.Context, string, models.Environment) (models.TenancyOverrideGroup, error) {
	return models.TenancyOverrideGroup{}, nil
}

func (stubLoader) LoadLimitRegionalOverrides(context.Context, string, models.Environment) ([]models.LimitRegionalOverride, error) {
	return nil, nil
}

func (stubLoader) LoadConsolePropertyRegionalOverrides(context.Context, string, models.Environment) ([]models.ConsolePropertyRegionalOverride, error) {
	return nil, nil
}

func (stubLoader) LoadPropertyRegionalOverrides(context.Context, string, models.Environment) ([]models.PropertyRegionalOverride, error) {
	return nil, nil
}

// TestIntegration_ToolsListAndCall wires NewServer against a stub
// loader over an in-memory MCP transport pair, then round-trips:
//
//   - tools/list (asserts every registered tool name is reachable)
//   - tools/call list_aliases (asserts the listResult envelope shape
//     end-to-end including JSON-RPC framing)
//
// Catches regressions in registerTools (typos, missing schemas,
// mismatched handler signatures) that the manual stdio smoke test
// would catch but `go test` would not.
func TestIntegration_ToolsListAndCall(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)

	srv := NewServer(
		config.Config{
			RepoPath:  "/dev/null",
			EnvType:   "dev",
			EnvRegion: "us-ashburn-1",
			EnvRealm:  "oc1",
		},
		stubLoader{},
		logging.NewNoOpLogger(),
		"test",
	)

	clientT, serverT := sdk.NewInMemoryTransports()
	serverSess, err := srv.server.Connect(ctx, serverT, nil)
	require.NoError(t, err, "server.Connect")
	t.Cleanup(func() { _ = serverSess.Close() })

	client := sdk.NewClient(&sdk.Implementation{Name: "test-client", Version: "v0"}, nil)
	clientSess, err := client.Connect(ctx, clientT, nil)
	require.NoError(t, err, "client.Connect")
	t.Cleanup(func() { _ = clientSess.Close() })

	// tools/list — every tool registered in registerTools must appear.
	listRes, err := clientSess.ListTools(ctx, &sdk.ListToolsParams{})
	require.NoError(t, err, "tools/list")
	got := make(map[string]bool, len(listRes.Tools))
	for _, tool := range listRes.Tools {
		got[tool.Name] = true
	}
	want := []string{
		"list_tenants",
		"list_base_models",
		"list_gpu_pools",
		"list_gpu_nodes",
		"list_dacs",
		"list_environments",
		"list_service_tenancies",
		"list_model_artifacts",
		"list_definitions",
		"list_tenancy_overrides",
		"list_regional_overrides",
		"list_aliases",
	}
	for _, name := range want {
		assert.True(t, got[name], "tools/list missing %q (got %d tools total)", name, len(listRes.Tools))
	}
	assert.Len(t, listRes.Tools, len(want), "unexpected tool count — registerTools may have drifted")

	// tools/call list_aliases — exercises the JSON-RPC framing and
	// asserts the listResult envelope shape end-to-end. list_aliases
	// doesn't touch the loader, so the stub is irrelevant here.
	callRes, err := clientSess.CallTool(ctx, &sdk.CallToolParams{Name: "list_aliases"})
	require.NoError(t, err, "tools/call list_aliases")
	require.NotNil(t, callRes)
	require.Len(t, callRes.Content, 1, "expected exactly one content block")

	textBlock, ok := callRes.Content[0].(*sdk.TextContent)
	require.True(t, ok, "first content block should be *TextContent, got %T", callRes.Content[0])

	var envelope struct {
		Items []struct {
			Alias    string `json:"alias"`
			Category string `json:"category"`
		} `json:"items"`
		Count    int      `json:"count"`
		Warnings []string `json:"warnings,omitempty"`
	}
	require.NoError(t, json.Unmarshal([]byte(textBlock.Text), &envelope), "envelope is valid JSON")
	assert.Equal(t, envelope.Count, len(envelope.Items), "count must match items length")
	assert.NotEmpty(t, envelope.Items, "list_aliases should return at least one alias")
	for _, it := range envelope.Items {
		assert.NotEmpty(t, it.Alias)
		assert.NotEmpty(t, it.Category)
	}
}
