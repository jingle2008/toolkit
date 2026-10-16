package tui

import (
	"slices"
	"testing"

	"github.com/charmbracelet/bubbles/table"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/pkg/models"
)

// TestRowSources_ParentKeysMatchRelations keeps the TUI's per-parent row
// matching in step with the domain relation registry: every relation into
// a category has a parent key in its row source (or drilling in would
// show the unfiltered list), and every parent key belongs to a relation
// (or it is dead code that a lingering scope could still trigger on the
// unguarded export path).
func TestRowSources_ParentKeysMatchRelations(t *testing.T) {
	t.Parallel()
	want := make(map[domain.Category][]domain.Category)
	for _, r := range domain.Relations() {
		want[r.Child] = append(want[r.Child], r.Parent)
	}
	for _, cat := range domain.Categories {
		exp := want[cat]
		slices.Sort(exp)
		assert.Equalf(t, exp, rowSources[cat].parents, "%s: row-source parent keys differ from relations", cat)
	}
}

func rowNames(rows []table.Row) []string {
	names := make([]string, len(rows))
	for i, r := range rows {
		names[i] = r[0]
	}
	slices.Sort(names)
	return names
}

// A DAC scopes GPU workloads by namespace, across every node they run on.
func TestComputeTableRows_DACScopesGPUWorkloads(t *testing.T) {
	t.Parallel()
	ds := &models.Dataset{GPUWorkloadMap: map[string][]models.GPUWorkload{
		"node-a": {
			{Name: "pod-1", Node: "node-a", Namespace: "dac-1"},
			{Name: "pod-2", Node: "node-a", Namespace: "dac-2"},
		},
		"node-b": {
			{Name: "pod-3", Node: "node-b", Namespace: "dac-1"},
			{Name: "pod-4", Node: "node-b", Namespace: "llama-3-3-70b-instruct"},
		},
	}}
	scope := &domain.Scope{Category: domain.DedicatedAICluster, Name: "dac-1"}

	rows, _ := computeTableRows(ds, domain.GPUWorkload, scope, "", "", true, false)
	assert.Equal(t, []string{"pod-1", "pod-3"}, rowNames(rows))

	// The free-text filter still applies inside the scope, and matches
	// the node (group key) as it does unscoped.
	rows, _ = computeTableRows(ds, domain.GPUWorkload, scope, "node-b", "", true, false)
	assert.Equal(t, []string{"pod-3"}, rowNames(rows))

	// Export narrows the same way.
	exp := rowSources[domain.GPUWorkload].rows(rowCtx{dataset: ds, scope: scope, export: true})
	assert.Equal(t, []string{"pod-1", "pod-3"}, rowNames(exp))

	// The node scope is unchanged: one node's workloads, whatever the DAC.
	rows, _ = computeTableRows(ds, domain.GPUWorkload, &domain.Scope{Category: domain.GPUNode, Name: "node-a"}, "", "", true, false)
	assert.Equal(t, []string{"pod-1", "pod-2"}, rowNames(rows))
}

// A BaseModel scopes the endpoints serving it from the shared catalog.
// Imported and legacy endpoints never match, even when their model name
// collides with a catalog name.
func TestComputeTableRows_BaseModelScopesInferenceServices(t *testing.T) {
	t.Parallel()
	ds := &models.Dataset{InferenceServiceMap: map[string][]models.InferenceService{
		"dac-1":             {{Name: "isvc-1", Namespace: "dac-1", Model: "llama"}},
		"dac-2":             {{Name: "isvc-2", Namespace: "dac-2", Model: "cohere"}},
		"llama":             {{Name: "isvc-3", Namespace: "llama", Model: "llama"}},
		"dac-3":             {{Name: "isvc-4", Namespace: "dac-3", Model: "llama", ImportedModel: true}},
		"content-moderator": {{Name: "isvc-5", Namespace: "content-moderator", Model: "llama", Legacy: true}},
	}}
	scope := &domain.Scope{Category: domain.BaseModel, Name: "llama"}

	rows, _ := computeTableRows(ds, domain.InferenceService, scope, "", "", true, false)
	assert.Equal(t, []string{"isvc-1", "isvc-3"}, rowNames(rows))

	// The DAC scope is unchanged: the namespace's endpoints only.
	rows, _ = computeTableRows(ds, domain.InferenceService, &domain.Scope{Category: domain.DedicatedAICluster, Name: "dac-1"}, "", "", true, false)
	assert.Equal(t, []string{"isvc-1"}, rowNames(rows))
}

// Enter on a BaseModel now drills into the endpoints serving it (y still
// opens details), and o returns to the model through the breadcrumb.
func TestEnterOnBaseModel_DrillsIntoInferenceServices(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	m.category = domain.BaseModel
	m.rawRows = []table.Row{{"llama"}}
	m.table.SetRows(m.rawRows)
	m.table.SetCursor(0)

	_ = m.enterContext()
	require.Equal(t, domain.InferenceService, m.category)
	require.Equal(t, &domain.Scope{Category: domain.BaseModel, Name: "llama"}, m.scope)

	_ = m.jumpToParent()
	assert.Equal(t, domain.BaseModel, m.category)
}

// A reference is not a structural parent: without a breadcrumb, o on a
// workload still goes to its node and on an endpoint to its DAC.
func TestParentScope_IgnoresReferences(t *testing.T) {
	t.Parallel()
	got, ok := parentScope(domain.GPUWorkload, table.Row{"pod-1", "node-a"})
	require.True(t, ok)
	assert.Equal(t, domain.Scope{Category: domain.GPUNode, Name: "node-a"}, got)

	got, ok = parentScope(domain.InferenceService, table.Row{"isvc-1", "dac-1"})
	require.True(t, ok)
	assert.Equal(t, domain.Scope{Category: domain.DedicatedAICluster, Name: "dac-1"}, got)
}

// An ImportedModel scopes the endpoints serving it by model name — both
// a namespaced BaseModel ref (ImportedModel set) and a tenant-labelled
// ClusterBaseModel ref (ImportedModel unset). Legacy endpoints never match.
func TestComputeTableRows_ImportedModelScopesInferenceServices(t *testing.T) {
	t.Parallel()
	const model = "amaaaaaa-imported"
	ds := &models.Dataset{InferenceServiceMap: map[string][]models.InferenceService{
		"dac-1": {{Name: "isvc-1", Namespace: "dac-1", Model: model, ImportedModel: true}},
		"dac-2": {{Name: "isvc-2", Namespace: "dac-2", Model: model}}, // tenant ClusterBaseModel
		"dac-3": {{Name: "isvc-3", Namespace: "dac-3", Model: "llama"}},
		"dac-4": {{Name: "isvc-4", Namespace: "dac-4", Model: model, Legacy: true}},
	}}
	scope := &domain.Scope{Category: domain.ImportedModel, Name: model}

	rows, _ := computeTableRows(ds, domain.InferenceService, scope, "", "", true, false)
	assert.Equal(t, []string{"isvc-1", "isvc-2"}, rowNames(rows))
}

// Enter on an ImportedModel drills into the endpoints serving it, while o
// without a breadcrumb still goes to its tenant.
func TestEnterOnImportedModel_DrillsIntoInferenceServices(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	m.category = domain.ImportedModel
	m.rawRows = []table.Row{{"amaaaaaa-imported", "tenant-1"}}
	m.table.SetRows(m.rawRows)
	m.table.SetCursor(0)

	_ = m.enterContext()
	require.Equal(t, domain.InferenceService, m.category)
	require.Equal(t, &domain.Scope{Category: domain.ImportedModel, Name: "amaaaaaa-imported"}, m.scope)

	got, ok := parentScope(domain.ImportedModel, table.Row{"amaaaaaa-imported", "tenant-1"})
	require.True(t, ok)
	assert.Equal(t, domain.Scope{Category: domain.Tenant, Name: "tenant-1"}, got)
}

// A tenant scopes pods and endpoints by their resolved owner; rows whose
// tenancy didn't resolve are under no tenant.
func TestComputeTableRows_TenantScopesWorkloadsAndEndpoints(t *testing.T) {
	t.Parallel()
	tenant := &models.Tenant{Name: "Tenant X"}
	other := &models.Tenant{Name: "Tenant Y"}
	ds := &models.Dataset{
		GPUWorkloadMap: map[string][]models.GPUWorkload{
			"node-a": {{Name: "pod-1", Owner: tenant}, {Name: "pod-2", Owner: other}},
			"node-b": {{Name: "pod-3", Owner: tenant}, {Name: "pod-4", TenantID: "Tenant X"}},
		},
		InferenceServiceMap: map[string][]models.InferenceService{
			"dac-1": {{Name: "isvc-1", Owner: tenant}},
			"dac-2": {{Name: "isvc-2", Owner: other}},
			"llama": {{Name: "isvc-3"}}, // on-demand: no tenant
		},
	}
	scope := &domain.Scope{Category: domain.Tenant, Name: "Tenant X"}

	rows, _ := computeTableRows(ds, domain.GPUWorkload, scope, "", "", true, false)
	assert.Equal(t, []string{"pod-1", "pod-3"}, rowNames(rows), "pod-4's raw tenancy-id must not match a tenant name")

	rows, _ = computeTableRows(ds, domain.InferenceService, scope, "", "", true, false)
	assert.Equal(t, []string{"isvc-1"}, rowNames(rows))
}

// A runtime scopes the endpoints naming it and the pods running it. A
// legacy endpoint never matches, even when its legacy runtime shares the
// name, and a pod matches by the runtime it actually runs — which covers
// endpoints whose runtime OME selected at deploy time.
func TestComputeTableRows_ServingRuntimeScopesEndpointsAndPods(t *testing.T) {
	t.Parallel()
	const rt = "gais-contentmoderate"
	ds := &models.Dataset{
		InferenceServiceMap: map[string][]models.InferenceService{
			"dac-1":             {{Name: "isvc-1", Runtime: rt}},
			"dac-2":             {{Name: "isvc-2"}}, // runtime selected at deploy time
			"dac-3":             {{Name: "isvc-3", Runtime: "vllm"}},
			"content-moderator": {{Name: "isvc-4", Runtime: rt, Legacy: true}},
		},
		GPUWorkloadMap: map[string][]models.GPUWorkload{
			"node-a": {{Name: "pod-1", Namespace: "dac-1", Runtime: rt}, {Name: "pod-2", Namespace: "dac-2", Runtime: rt}},
			"node-b": {{Name: "pod-3", Namespace: "dac-3", Runtime: "vllm"}},
		},
	}
	scope := &domain.Scope{Category: domain.ServingRuntime, Name: rt}

	rows, _ := computeTableRows(ds, domain.InferenceService, scope, "", "", true, false)
	assert.Equal(t, []string{"isvc-1"}, rowNames(rows))

	rows, _ = computeTableRows(ds, domain.GPUWorkload, scope, "", "", true, false)
	assert.Equal(t, []string{"pod-1", "pod-2"}, rowNames(rows))
}

// Enter on a ServingRuntime drills into the endpoints naming it; o
// returns through the breadcrumb (a runtime has no structural parent).
func TestEnterOnServingRuntime_DrillsIntoInferenceServices(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	m.category = domain.ServingRuntime
	m.rawRows = []table.Row{{"vllm"}}
	m.table.SetRows(m.rawRows)
	m.table.SetCursor(0)

	_ = m.enterContext()
	require.Equal(t, domain.InferenceService, m.category)
	require.Equal(t, &domain.Scope{Category: domain.ServingRuntime, Name: "vllm"}, m.scope)

	_ = m.jumpToParent()
	assert.Equal(t, domain.ServingRuntime, m.category)
}

// Enter on a Tenant still opens its limit overrides: the reference
// children are listed after the Contains ones.
func TestEnterOnTenant_StillOpensLimitOverrides(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	m.category = domain.Tenant
	m.rawRows = []table.Row{{"Tenant X"}}
	m.table.SetRows(m.rawRows)
	m.table.SetCursor(0)

	_ = m.enterContext()
	assert.Equal(t, domain.LimitTenancyOverride, m.category)
}
