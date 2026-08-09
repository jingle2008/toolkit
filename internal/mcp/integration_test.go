package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jingle2008/toolkit/internal/config"
	"github.com/jingle2008/toolkit/internal/infra/loader"
	"github.com/jingle2008/toolkit/internal/infra/terraform"
	"github.com/jingle2008/toolkit/pkg/infra/logging"
	"github.com/jingle2008/toolkit/pkg/models"
)

// stubLoader implements just enough of the loader.Composite interface to
// satisfy NewServer. The integration test only exercises list_aliases
// (which doesn't reach the loader at all), so every method returns an
// empty result; the methods exist only because Server's constructor
// stores a loader.Composite.
type stubLoader struct{}

func (stubLoader) LoadDataset(context.Context, string, models.Environment) (*models.Dataset, error) {
	return &models.Dataset{}, nil
}

func (stubLoader) LoadBaseModels(context.Context, string, models.Environment) ([]models.BaseModel, error) {
	return nil, nil
}

func (stubLoader) LoadImportedModels(context.Context, string, models.Environment) (map[string][]models.ImportedModel, error) {
	return nil, nil //nolint:nilnil // empty-map stub; tests rarely read this
}

func (stubLoader) LoadGPUPools(context.Context, string, models.Environment) ([]models.GPUPool, error) {
	return nil, nil
}

func (stubLoader) LoadGPUNodesByPool(context.Context, string, models.Environment) (map[string][]models.GPUNode, error) {
	return nil, nil //nolint:nilnil // empty-map test stub; integration tests never read this
}

func (stubLoader) LoadGPUWorkloadsByNode(context.Context, string, models.Environment) (map[string][]models.GPUWorkload, error) {
	return nil, nil //nolint:nilnil // empty-map test stub; integration tests never read this
}

func (stubLoader) LoadDedicatedAIClusters(context.Context, string, models.Environment) (map[string][]models.DedicatedAICluster, error) {
	return nil, nil //nolint:nilnil // empty-map test stub; integration tests never read this
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

// errBaseModelsLoader makes LoadBaseModels return a fixed error so we
// can exercise a tool handler's fatal-error path. Everything else
// inherits stubLoader's empty defaults.
type errBaseModelsLoader struct {
	stubLoader
	err error
}

func (l errBaseModelsLoader) LoadBaseModels(context.Context, string, models.Environment) ([]models.BaseModel, error) {
	return nil, l.err
}

// partialGPUPoolsLoader makes LoadGPUPools return a *terraform.PartialLoadError
// so we can exercise the partial-success path in handleListGPUPools
// (tool call still succeeds; the envelope carries a warning).
type partialGPUPoolsLoader struct {
	stubLoader
	err *terraform.PartialLoadError
}

func (l partialGPUPoolsLoader) LoadGPUPools(context.Context, string, models.Environment) ([]models.GPUPool, error) {
	return nil, l.err
}

// fixedGPUPoolsLoader returns scripted pools so EnrichGPUPools's
// fast-path (len(pools)==0) doesn't short-circuit the new
// enrichment branch in handleListGPUPools. Everything else inherits
// from stubLoader.
type fixedGPUPoolsLoader struct {
	stubLoader
	pools []models.GPUPool
}

func (l fixedGPUPoolsLoader) LoadGPUPools(context.Context, string, models.Environment) ([]models.GPUPool, error) {
	return l.pools, nil
}

func newTestPair(ctx context.Context, t *testing.T, ld loader.Composite, opts ...func(*config.Config)) *sdk.ClientSession {
	t.Helper()
	cfg := config.Config{
		RepoPath:  "/dev/null",
		EnvType:   "dev",
		EnvRegion: "us-ashburn-1",
		EnvRealm:  "oc1",
	}
	for _, o := range opts {
		o(&cfg)
	}
	srv := NewServer(
		cfg,
		ld,
		logging.NewNoOpLogger(),
		"test",
	)

	clientT, serverT := sdk.NewInMemoryTransports()
	serverSess, err := srv.server.Connect(ctx, serverT, nil)
	require.NoError(t, err, "server.Connect")
	t.Cleanup(func() { _ = serverSess.Close() })

	client := sdk.NewClient(
		&sdk.Implementation{Name: "test-client", Version: "v0"},
		nil,
	)
	clientSess, err := client.Connect(ctx, clientT, nil)
	require.NoError(t, err, "client.Connect")
	t.Cleanup(func() { _ = clientSess.Close() })

	return clientSess
}

// resultText joins the text blocks of a tool result.
//
// It is how a client reads both a failure — the SDK renders a handler's
// returned error as TextContent alongside IsError — and the auto-emitted
// text form of a structured result. Since notify() was removed (SEP-2577
// deprecated the MCP logging channel it rode on), this and
// StructuredContent are the only places a message can surface, which
// makes them the right thing to assert against.
func resultText(t *testing.T, res *sdk.CallToolResult) string {
	t.Helper()
	var sb strings.Builder
	for _, c := range res.Content {
		if tc, ok := c.(*sdk.TextContent); ok {
			sb.WriteString(tc.Text)
			sb.WriteString("\n")
		}
	}
	return sb.String()
}

// structured round-trips a result's StructuredContent through JSON so
// tests can interrogate it as a plain map regardless of the concrete
// envelope type.
func structured(t *testing.T, res *sdk.CallToolResult) map[string]any {
	t.Helper()
	require.NotNil(t, res.StructuredContent, "StructuredContent should be populated")
	b, err := json.Marshal(res.StructuredContent)
	require.NoError(t, err)
	var m map[string]any
	require.NoError(t, json.Unmarshal(b, &m))
	return m
}

func TestIntegration_SurfacesHandlerError(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)

	ld := errBaseModelsLoader{err: errors.New("kube unreachable")}
	clientSess := newTestPair(ctx, t, ld)

	res, err := clientSess.CallTool(ctx, &sdk.CallToolParams{Name: "list_base_models"})
	// The SDK surfaces tool handler errors via CallToolResult.IsError,
	// not a Go error from CallTool itself.
	require.NoError(t, err, "tools/call transport error")
	require.NotNil(t, res)
	assert.True(t, res.IsError, "expected IsError=true on tool failure")

	// failTool's label and the underlying cause must both reach the
	// client. This is the whole error channel now that notify() is gone.
	body := resultText(t, res)
	assert.Contains(t, body, "load base models")
	assert.Contains(t, body, "kube unreachable")
}

func TestIntegration_WarnsOnPartialLoad(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)

	partial := &terraform.PartialLoadError{
		Source: "GPUPools",
		Errs:   []error{errors.New("oke nodepools dir missing")},
	}
	ld := partialGPUPoolsLoader{err: partial}
	clientSess := newTestPair(ctx, t, ld)

	res, err := clientSess.CallTool(ctx, &sdk.CallToolParams{Name: "list_gpu_pools"})
	require.NoError(t, err, "tools/call transport error")
	require.NotNil(t, res)
	assert.False(t, res.IsError, "partial-load should not fail the tool call")

	// The per-source error reaches the client through the envelope's
	// warnings field.
	warnings := structured(t, res)["warnings"]
	require.NotNil(t, warnings, "partial load must populate warnings")
	assert.Contains(t, fmt.Sprint(warnings), "oke nodepools dir missing")
}

// TestIntegration_WarnsOnGPUPoolEnrichmentFailure pins the enrichment
// branch in handleListGPUPools (TUI parity step). With a non-empty pool
// slice and a deliberately bad kubeconfig path, resolve.EnrichGPUPools
// must fail at the CompartmentID step, surface the warning in the
// listResult.warnings envelope field, and still return the
// Terraform-derived pool (no IsError). Regression bait: a missed append
// to warnings or a re-introduction of fail-on-enrichment will both break
// this test.
func TestIntegration_WarnsOnGPUPoolEnrichmentFailure(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)

	ld := fixedGPUPoolsLoader{pools: []models.GPUPool{
		{Name: "p1", Shape: "BM.GPU", Status: "...", Size: 8},
	}}
	clientSess := newTestPair(ctx, t, ld, func(c *config.Config) {
		// ExplicitPath kubeconfig that can't be loaded → CompartmentID
		// fails at the clientcmd step, mirroring an offline / no-kube
		// host. We deliberately don't swap the resolve seams because
		// we want the real failure path (clientcmd parse) covered too.
		c.KubeConfig = "/dev/null/no-such-kubeconfig"
	})

	res, err := clientSess.CallTool(ctx, &sdk.CallToolParams{Name: "list_gpu_pools"})
	require.NoError(t, err, "tools/call transport error")
	require.NotNil(t, res)
	assert.False(t, res.IsError, "enrichment failure must not fail the tool call")

	// Decode the listResult envelope from StructuredContent.
	scBytes, err := json.Marshal(res.StructuredContent)
	require.NoError(t, err)
	var env struct {
		Items    []map[string]any `json:"items"`
		Count    int              `json:"count"`
		Warnings []string         `json:"warnings"`
	}
	require.NoError(t, json.Unmarshal(scBytes, &env))

	// Terraform-derived data must still be returned.
	assert.Equal(t, 1, env.Count, "Terraform pool should pass through despite enrichment failure")
	require.Len(t, env.Items, 1)
	assert.Equal(t, "p1", env.Items[0]["name"])
	assert.Equal(t, "...", env.Items[0]["status"], "placeholder status must survive enrichment failure")

	// The envelope is the client's channel for the warning.
	require.NotEmpty(t, env.Warnings, "warnings envelope must include enrichment-incomplete entry")
	foundWarn := false
	for _, w := range env.Warnings {
		if strings.Contains(w, "enrichment incomplete") {
			foundWarn = true
			break
		}
	}
	assert.True(t, foundWarn, "warnings should include 'enrichment incomplete' entry: %v", env.Warnings)
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
		// Read-only list_* tools.
		"list_tenants",
		"list_base_models",
		"list_imported_models",
		"list_gpu_pools",
		"list_gpu_nodes",
		"list_gpu_workloads",
		"list_dacs",
		"list_environments",
		"list_service_tenancies",
		"list_model_artifacts",
		"list_definitions",
		"list_tenancy_overrides",
		"list_regional_overrides",
		"list_aliases",
		// Mutation tools (all gated on confirm=true; see mutations.go).
		"cordon_node",
		"uncordon_node",
		"drain_node",
		"reboot_node",
		"terminate_node",
		"scale_gpu_pool",
		"delete_dac",
		"set_tenant",
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

	// Modern MCP clients prefer StructuredContent. Verify the SDK
	// populated it with the same envelope object — this is the
	// regression bait for the refactor that lifted list handlers from
	// `struct{}` Out to typed `listResult[T]` Out.
	require.NotNil(t, callRes.StructuredContent, "StructuredContent should be populated")
	scBytes, err := json.Marshal(callRes.StructuredContent)
	require.NoError(t, err)
	var scEnvelope struct {
		Items []struct {
			Alias    string `json:"alias"`
			Category string `json:"category"`
		} `json:"items"`
		Count int `json:"count"`
	}
	require.NoError(t, json.Unmarshal(scBytes, &scEnvelope), "structuredContent decodes as listResult envelope")
	assert.Equal(t, envelope.Count, scEnvelope.Count, "structuredContent count matches text envelope")
	assert.Equal(t, len(envelope.Items), len(scEnvelope.Items), "structuredContent items match text envelope")
}
