package tui

import (
	"context"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/table"
	"github.com/stretchr/testify/require"

	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/internal/ui/tui/common"
	"github.com/jingle2008/toolkit/pkg/models"
)

func TestHandleBaseModelsLoaded(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	m.gens.msg = 1
	items := []models.BaseModel{{Name: "bm1"}}

	m.handleBaseModelsLoaded(items, 1)
	if len(m.dataset.BaseModels) != 1 || m.dataset.BaseModels[0].Name != "bm1" {
		t.Fatalf("BaseModels not updated: %#v", m.dataset.BaseModels)
	}
}

func TestHandleImportedModelsLoaded(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	m.gens.msg = 1
	items := map[string][]models.ImportedModel{
		"ocid1.tenancy.x": {{BaseModel: models.BaseModel{Name: "im1"}, Namespace: "team-x", TenantID: "ocid1.tenancy.x"}},
	}

	m.handleImportedModelsLoaded(items, 1)
	// SetImportedModelMap re-keys to Tenant.Name when matched; the
	// test model has no realm tenants configured, so the raw key
	// passes through.
	got := m.dataset.ImportedModelMap["ocid1.tenancy.x"]
	if len(got) != 1 || got[0].Name != "im1" {
		t.Fatalf("ImportedModelMap not updated: %#v", m.dataset.ImportedModelMap)
	}
}

func TestHandleImportedModelsLoaded_GenMismatch(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	m.gens.msg = 2
	m.dataset.ImportedModelMap = map[string][]models.ImportedModel{
		"ocid1.tenancy.x": {{BaseModel: models.BaseModel{Name: "old"}, Namespace: "team-x", TenantID: "ocid1.tenancy.x"}},
	}

	m.handleImportedModelsLoaded(map[string][]models.ImportedModel{
		"ocid1.tenancy.y": {{BaseModel: models.BaseModel{Name: "new"}, Namespace: "team-y", TenantID: "ocid1.tenancy.y"}},
	}, 1)
	got := m.dataset.ImportedModelMap["ocid1.tenancy.x"]
	if len(got) != 1 || got[0].Name != "old" {
		t.Fatalf("ImportedModelMap updated on gen mismatch: %#v", m.dataset.ImportedModelMap)
	}
}

// A background catalog load (e.g. resolving a DAC's metrics capability while
// the DAC list is on screen) must cache its data without rebuilding the
// current table, which would reset the cursor and clear the active filter.
func TestApplyDataset_BackgroundLoadPreservesCurrentView(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	m.category = domain.DedicatedAICluster
	m.filter = "keep-me"

	m.applyDataset(func(ds *models.Dataset) {
		ds.BaseModels = []models.BaseModel{{Name: "bm1"}}
	}, domain.BaseModel, 1)

	if m.filter != "keep-me" {
		t.Fatalf("background load cleared the active filter: %q", m.filter)
	}
	if len(m.dataset.BaseModels) != 1 {
		t.Fatalf("background load did not cache the data: %#v", m.dataset.BaseModels)
	}
}

// A load for the category currently on screen refreshes the rows but PRESERVES
// the active filter — only category navigation clears it (see
// TestUpdateCategoryCore_NavigationClearsFilter).
func TestApplyDataset_CurrentCategoryLoadPreservesFilter(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	m.category = domain.BaseModel
	m.filter = "keep"

	m.applyDataset(func(ds *models.Dataset) {
		ds.BaseModels = []models.BaseModel{{Name: "bm1"}}
	}, domain.BaseModel, 1)

	if m.filter != "keep" {
		t.Fatalf("current-category load cleared the filter: %q", m.filter)
	}
	if len(m.dataset.BaseModels) != 1 {
		t.Fatalf("current-category load did not apply data: %#v", m.dataset.BaseModels)
	}
}

func TestHandleBaseModelsLoaded_GenMismatch(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	m.gens.msg = 2
	m.dataset.BaseModels = []models.BaseModel{{Name: "old"}}

	m.handleBaseModelsLoaded([]models.BaseModel{{Name: "new"}}, 1)
	if len(m.dataset.BaseModels) != 1 || m.dataset.BaseModels[0].Name != "old" {
		t.Fatalf("BaseModels updated on gen mismatch: %#v", m.dataset.BaseModels)
	}
}

func TestHandleGPUPoolsLoaded(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	m.gens.msg = 1
	items := []models.GPUPool{{Name: "pool1"}}

	cmd := m.handleGPUPoolsLoaded(items, 1)
	if cmd == nil {
		t.Fatal("expected non-nil cmd from handleGPUPoolsLoaded")
	}
	if len(m.dataset.GPUPools) != 1 || m.dataset.GPUPools[0].Name != "pool1" {
		t.Fatalf("GPUPools not updated: %#v", m.dataset.GPUPools)
	}
}

func TestHandleGPUNodesLoaded(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	m.gens.msg = 1
	items := map[string][]models.GPUNode{"pool": {{Name: "node1"}}}

	m.handleGPUNodesLoaded(items, 1)
	if got := m.dataset.GPUNodeMap["pool"]; len(got) != 1 || got[0].Name != "node1" {
		t.Fatalf("GPUNodeMap not updated: %#v", m.dataset.GPUNodeMap)
	}
}

func TestHandleGPUWorkloadsLoaded(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	m.dataset = &models.Dataset{}
	items := map[string][]models.GPUWorkload{"node-a": {{Name: "p1", Node: "node-a"}}}
	m.handleGPUWorkloadsLoaded(items, m.gens.msg)
	if got := m.dataset.GPUWorkloadMap["node-a"]; len(got) != 1 || got[0].Name != "p1" {
		t.Fatalf("GPUWorkloadMap not applied: %+v", m.dataset.GPUWorkloadMap)
	}
}

func TestHandleDedicatedAIClustersLoaded(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	m.gens.msg = 1
	items := map[string][]models.DedicatedAICluster{
		"id1": {{Name: "cluster1"}},
	}

	m.handleDedicatedAIClustersLoaded(items, 1)
	if got := m.dataset.DedicatedAIClusterMap["tenant1"]; len(got) != 1 || got[0].Name != "cluster1" {
		t.Fatalf("DedicatedAIClusterMap not updated: %#v", m.dataset.DedicatedAIClusterMap)
	}
}

func TestHandleTenancyOverridesLoaded(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	m.gens.msg = 1
	group := models.TenancyOverrideGroup{
		Tenants: []models.Tenant{{Name: "tenant-x"}},
		LimitTenancyOverrideMap: map[string][]models.LimitTenancyOverride{
			"t1": {{LimitRegionalOverride: models.LimitRegionalOverride{Name: "l1"}}},
		},
		ConsolePropertyTenancyOverrideMap: map[string][]models.ConsolePropertyTenancyOverride{
			"t1": {{ConsolePropertyRegionalOverride: models.ConsolePropertyRegionalOverride{Name: "c1"}}},
		},
		PropertyTenancyOverrideMap: map[string][]models.PropertyTenancyOverride{
			"t1": {{PropertyRegionalOverride: models.PropertyRegionalOverride{Name: "p1"}}},
		},
	}

	m.handleTenancyOverridesLoaded(group, 1)
	if len(m.dataset.Tenants) != 1 || m.dataset.Tenants[0].Name != "tenant-x" {
		t.Fatalf("Tenants not updated: %#v", m.dataset.Tenants)
	}
	if len(m.dataset.LimitTenancyOverrideMap["t1"]) != 1 {
		t.Fatalf("LimitTenancyOverrideMap not updated: %#v", m.dataset.LimitTenancyOverrideMap)
	}
	if len(m.dataset.ConsolePropertyTenancyOverrideMap["t1"]) != 1 {
		t.Fatalf("ConsolePropertyTenancyOverrideMap not updated: %#v", m.dataset.ConsolePropertyTenancyOverrideMap)
	}
	if len(m.dataset.PropertyTenancyOverrideMap["t1"]) != 1 {
		t.Fatalf("PropertyTenancyOverrideMap not updated: %#v", m.dataset.PropertyTenancyOverrideMap)
	}
}

func TestHandleRegionalOverridesLoaded(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	m.gens.msg = 1

	limitOverrides := []models.LimitRegionalOverride{{Name: "l1"}}
	consoleOverrides := []models.ConsolePropertyRegionalOverride{{Name: "c1"}}
	propertyOverrides := []models.PropertyRegionalOverride{{Name: "p1"}}

	m.handleLimitRegionalOverridesLoaded(limitOverrides, 1)
	m.handleConsolePropertyRegionalOverridesLoaded(consoleOverrides, 1)
	m.handlePropertyRegionalOverridesLoaded(propertyOverrides, 1)

	if len(m.dataset.LimitRegionalOverrides) != 1 {
		t.Fatalf("LimitRegionalOverrides not updated: %#v", m.dataset.LimitRegionalOverrides)
	}
	if len(m.dataset.ConsolePropertyRegionalOverrides) != 1 {
		t.Fatalf("ConsolePropertyRegionalOverrides not updated: %#v", m.dataset.ConsolePropertyRegionalOverrides)
	}
	if len(m.dataset.PropertyRegionalOverrides) != 1 {
		t.Fatalf("PropertyRegionalOverrides not updated: %#v", m.dataset.PropertyRegionalOverrides)
	}
}

// A same-category data load must preserve the active filter; the filter is
// cleared only on category navigation (updateCategoryCore).
func TestApplyDataset_PreservesFilter(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	m.gens.msg = 1
	m.filter = "old"
	m.category = domain.Tenant

	m.applyDataset(func(ds *models.Dataset) {
		ds.Tenants = []models.Tenant{{Name: "tenant1"}}
	}, domain.Tenant, 1)

	if m.filter != "old" {
		t.Fatalf("same-category load cleared the filter: got %q, want %q", m.filter, "old")
	}
}

func TestGetCompartmentID_FromDataset(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	m.dataset.GPUNodeMap = map[string][]models.GPUNode{
		"pool": {{CompartmentID: "ocid1.compartment"}},
	}

	got, err := m.lookupCompartmentID(context.Background())
	if err != nil {
		t.Fatalf("lookupCompartmentID error: %v", err)
	}
	if got != "ocid1.compartment" {
		t.Fatalf("got compartment %q", got)
	}
}

func TestSortTableByColumn_Toggles(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	m.sortColumn = common.NameCol
	m.sortAsc = true

	m.sortTableByColumn(common.NameCol)
	if m.sortAsc {
		t.Fatal("expected sortAsc to toggle to false")
	}
	m.sortTableByColumn(common.TypeCol)
	if m.sortColumn != common.TypeCol || !m.sortAsc {
		t.Fatalf("expected sortColumn=%q sortAsc=true, got %q %v", common.TypeCol, m.sortColumn, m.sortAsc)
	}
}

// A same-category reload keeps the cursor on the row the user had selected,
// matched by its Name cell, even when its index shifts.
func TestApplyRows_PreservesSelectedRowAcrossReload(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	m.category = domain.BaseModel
	m.dataset = &models.Dataset{BaseModels: []models.BaseModel{
		{Name: "bm1"}, {Name: "bm2"}, {Name: "bm3"},
	}}
	m.refreshDisplay()   // populate columns + rows
	m.table.SetCursor(1) // select bm2

	m.refreshDisplay() // simulate an in-place reload of the same category

	got := m.selectedRawRow()
	if len(got) == 0 || got[0] != "bm2" {
		t.Fatalf("cursor not preserved on reload: %v", got)
	}
}

// When the previously-selected row is gone after a reload, the cursor clamps to
// a valid index instead of pointing past the end.
func TestApplyRows_ClampsWhenSelectedRowDisappears(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	m.category = domain.BaseModel
	m.dataset = &models.Dataset{BaseModels: []models.BaseModel{
		{Name: "bm1"}, {Name: "bm2"}, {Name: "bm3"},
	}}
	m.refreshDisplay()
	m.table.SetCursor(1) // select bm2

	m.dataset.BaseModels = []models.BaseModel{{Name: "bm1"}, {Name: "bm3"}}
	m.refreshDisplay() // bm2 no longer present

	c := m.table.Cursor()
	if c < 0 || c >= len(m.table.Rows()) {
		t.Fatalf("cursor out of range after reload: %d (rows=%d)", c, len(m.table.Rows()))
	}
}

// For scoped categories (e.g. ImportedModel), the Name cell alone is not a
// unique row key — itemKeyFrom keys these on ScopedItemKey{Scope: row[1],
// Name: row[0]}. A reload must re-home the cursor onto the same item by its
// full key, not jump to the first row that merely shares the Name.
func TestApplyRows_PreservesSelectedRow_DuplicateNamesScopedCategory(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	m.category = domain.ImportedModel

	// Two rows share the Name "modelA" but live under different scopes
	// (row[1]): the user selects the second one.
	rows := []table.Row{
		{"modelA", "tenant1"},
		{"modelA", "tenant2"},
	}
	m.applyRows(rows, tableStats{}, true)
	m.table.SetCursor(1) // select (modelA, tenant2)

	// Reload with the same rows. Matching on row[0] alone would re-home onto
	// index 0 (the first "modelA" = tenant1); matching on the item key keeps
	// the cursor on (modelA, tenant2).
	m.applyRows([]table.Row{
		{"modelA", "tenant1"},
		{"modelA", "tenant2"},
	}, tableStats{}, true)

	got := m.selectedRawRow()
	if len(got) < 2 || got[1] != "tenant2" {
		t.Fatalf("reload re-homed onto the wrong same-named row: got %v, want scope tenant2", got)
	}
}

// scopedTruncatedModel returns a model set up as ImportedModel with two
// narrow, middle-truncated key columns (Name + Tenant) — the layout that
// makes the displayed rows differ from their un-truncated identity.
func scopedTruncatedModel(t *testing.T) *Model {
	t.Helper()
	m := newTestModel(t)
	m.category = domain.ImportedModel
	m.headers = []header{
		{text: "Name", truncateMiddle: true},
		{text: "Tenant", truncateMiddle: true},
	}
	table.WithColumns([]table.Column{
		{Title: "Name", Width: 7},
		{Title: "Tenant", Width: 7},
	})(m.table)
	return m
}

// The reload identity match must run against m.rawRows (un-truncated), not the
// `rows` slice that applyMiddleTruncation has already shortened. With the key
// cells truncated, comparing the displayed cells against the un-truncated
// prevKey would miss and drop the cursor to the top. Same offset → fast path.
func TestApplyRows_FastPath_TruncatedKeyColumns(t *testing.T) {
	t.Parallel()
	m := scopedTruncatedModel(t)

	const longName = "imported-model-very-long-name"
	m.applyRows([]table.Row{
		{longName, "tenant-aaaaaaaaaaaaaaaa"},
		{longName, "tenant-bbbbbbbbbbbbbbbb"},
	}, tableStats{}, true)
	m.table.SetCursor(1) // select (longName, tenant-b…)

	// Reload, same order: the item is still at offset 1 (fast path).
	m.applyRows([]table.Row{
		{longName, "tenant-aaaaaaaaaaaaaaaa"},
		{longName, "tenant-bbbbbbbbbbbbbbbb"},
	}, tableStats{}, true)

	got := m.selectedRawRow()
	if m.table.Cursor() != 1 || len(got) < 2 || got[1] != "tenant-bbbbbbbbbbbbbbbb" {
		t.Fatalf("truncated-key reload lost the selection: cursor=%d got=%v", m.table.Cursor(), got)
	}
}

// When the selected item moves to a new offset, the fast path misses and the
// scan (also over m.rawRows) must still find it by identity — even with the
// key cells truncated.
func TestApplyRows_FallbackScan_AfterReorder_TruncatedKeyColumns(t *testing.T) {
	t.Parallel()
	m := scopedTruncatedModel(t)

	const longName = "imported-model-very-long-name"
	m.applyRows([]table.Row{
		{longName, "tenant-aaaaaaaaaaaaaaaa"},
		{longName, "tenant-bbbbbbbbbbbbbbbb"},
	}, tableStats{}, true)
	m.table.SetCursor(1) // select (longName, tenant-b…)

	// Reload with the rows swapped: tenant-b is now at offset 0, so the fast
	// path at offset 1 misses and the scan must relocate it.
	m.applyRows([]table.Row{
		{longName, "tenant-bbbbbbbbbbbbbbbb"},
		{longName, "tenant-aaaaaaaaaaaaaaaa"},
	}, tableStats{}, true)

	got := m.selectedRawRow()
	if m.table.Cursor() != 0 || len(got) < 2 || got[1] != "tenant-bbbbbbbbbbbbbbbb" {
		t.Fatalf("reorder reload did not follow the item: cursor=%d got=%v", m.table.Cursor(), got)
	}
}

func TestOpContext(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	ctx, cancel := m.opCtx()
	t.Cleanup(cancel)

	deadline, ok := ctx.Deadline()
	if !ok {
		t.Fatal("expected deadline")
	}
	remaining := time.Until(deadline)
	if remaining <= 0 {
		t.Fatal("expected deadline in the future")
	}
}

// handleDataMsg owns only the foundational dataset load and the refresh
// signal; per-category data flows through the typed *LoadedMsg handlers.
// Feeding it per-category data must NOT mutate the dataset.
func TestHandleDataMsg_IgnoresPerCategoryPayload(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	m.dataset = &models.Dataset{}

	m.handleDataMsg(dataMsg{Data: []models.GPUPool{{Name: "p1"}}})
	require.Empty(t, m.dataset.GPUPools, "per-category payload must not be applied by handleDataMsg")

	ds := &models.Dataset{}
	m.handleDataMsg(dataMsg{Data: ds})
	require.Same(t, ds, m.dataset, "foundational *models.Dataset payload must still be applied")
}
