package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
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

// partialGpuPoolsLoader makes LoadGpuPools return a *terraform.PartialLoadError
// so we can exercise the partial-success path in handleListGpuPools
// (tool call still succeeds; a warning notification is emitted).
type partialGpuPoolsLoader struct {
	stubLoader
	err *terraform.PartialLoadError
}

func (l partialGpuPoolsLoader) LoadGpuPools(context.Context, string, models.Environment) ([]models.GpuPool, error) {
	return nil, l.err
}

// recorder collects notifications/message frames the server emits.
type recorder struct {
	mu   sync.Mutex
	msgs []*sdk.LoggingMessageParams
}

func (r *recorder) record(_ context.Context, req *sdk.LoggingMessageRequest) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.msgs = append(r.msgs, req.Params)
}

func (r *recorder) snapshot() []*sdk.LoggingMessageParams {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]*sdk.LoggingMessageParams, len(r.msgs))
	copy(out, r.msgs)
	return out
}

func newTestPair(ctx context.Context, t *testing.T, ld loader.Loader, rec *recorder, opts ...func(*config.Config)) *sdk.ClientSession {
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
		&sdk.ClientOptions{LoggingMessageHandler: rec.record},
	)
	clientSess, err := client.Connect(ctx, clientT, nil)
	require.NoError(t, err, "client.Connect")
	t.Cleanup(func() { _ = clientSess.Close() })

	// Server gates Log on this — without a level set, notifications drop silently.
	require.NoError(t, clientSess.SetLoggingLevel(ctx, &sdk.SetLoggingLevelParams{Level: "debug"}))
	return clientSess
}

// waitForMsgs polls the recorder until at least one message has
// arrived or the deadline expires. Notification delivery on the
// in-memory transport is asynchronous to the tool response, so we
// can't synchronously assert right after CallTool returns.
func waitForMsgs(t *testing.T, rec *recorder) []*sdk.LoggingMessageParams {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		got := rec.snapshot()
		if len(got) >= 1 {
			return got
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("expected ≥1 notifications, got %d", len(rec.snapshot()))
	return nil
}

func TestIntegration_NotifiesOnHandlerError(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)

	rec := &recorder{}
	ld := errBaseModelsLoader{err: errors.New("kube unreachable")}
	clientSess := newTestPair(ctx, t, ld, rec)

	res, err := clientSess.CallTool(ctx, &sdk.CallToolParams{Name: "list_base_models"})
	// The SDK surfaces tool handler errors via CallToolResult.IsError,
	// not a Go error from CallTool itself.
	require.NoError(t, err, "tools/call transport error")
	require.NotNil(t, res)
	assert.True(t, res.IsError, "expected IsError=true on tool failure")

	msgs := waitForMsgs(t, rec)
	require.NotEmpty(t, msgs)
	got := msgs[0]
	assert.Equal(t, sdk.LoggingLevel("error"), got.Level, "expected error-level notification")
	assert.Equal(t, "toolkit", got.Logger)
	body, ok := got.Data.(string)
	require.True(t, ok, "Data should be a string, got %T", got.Data)
	assert.Contains(t, body, "load base models")
	assert.Contains(t, body, "kube unreachable")
}

func TestIntegration_NotifiesOnPartialLoad(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)

	rec := &recorder{}
	partial := &terraform.PartialLoadError{
		Source: "GpuPools",
		Errs:   []error{errors.New("oke nodepools dir missing")},
	}
	ld := partialGpuPoolsLoader{err: partial}
	clientSess := newTestPair(ctx, t, ld, rec)

	res, err := clientSess.CallTool(ctx, &sdk.CallToolParams{Name: "list_gpu_pools"})
	require.NoError(t, err, "tools/call transport error")
	require.NotNil(t, res)
	assert.False(t, res.IsError, "partial-load should not fail the tool call")

	msgs := waitForMsgs(t, rec)
	got := msgs[0]
	assert.Equal(t, sdk.LoggingLevel("warning"), got.Level)
	body, ok := got.Data.(string)
	require.True(t, ok, "Data should be a string, got %T", got.Data)
	assert.Contains(t, body, "load gpu pools")
	assert.True(t, strings.Contains(body, "oke nodepools dir missing"),
		"warning body should include the per-source error: %q", body)
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
		// Mutation tools (all gated on confirm=true; see mutations.go).
		"cordon_node",
		"uncordon_node",
		"drain_node",
		"reboot_node",
		"terminate_node",
		"scale_gpu_pool",
		"delete_dac",
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
