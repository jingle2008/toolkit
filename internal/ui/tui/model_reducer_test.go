package tui

import (
	"context"
	"testing"
	"time"

	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/internal/ui/tui/common"
	"github.com/jingle2008/toolkit/pkg/models"
)

func TestHandleBaseModelsLoaded(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	m.gen = 1
	items := []models.BaseModel{{Name: "bm1"}}

	m.handleBaseModelsLoaded(items, 1)
	if len(m.dataset.BaseModels) != 1 || m.dataset.BaseModels[0].Name != "bm1" {
		t.Fatalf("BaseModels not updated: %#v", m.dataset.BaseModels)
	}
}

func TestHandleBaseModelsLoaded_GenMismatch(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	m.gen = 2
	m.dataset.BaseModels = []models.BaseModel{{Name: "old"}}

	m.handleBaseModelsLoaded([]models.BaseModel{{Name: "new"}}, 1)
	if len(m.dataset.BaseModels) != 1 || m.dataset.BaseModels[0].Name != "old" {
		t.Fatalf("BaseModels updated on gen mismatch: %#v", m.dataset.BaseModels)
	}
}

func TestHandleGpuPoolsLoaded(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	m.gen = 1
	items := []models.GpuPool{{Name: "pool1"}}

	cmd := m.handleGpuPoolsLoaded(items, 1)
	if cmd == nil {
		t.Fatal("expected non-nil cmd from handleGpuPoolsLoaded")
	}
	if len(m.dataset.GpuPools) != 1 || m.dataset.GpuPools[0].Name != "pool1" {
		t.Fatalf("GpuPools not updated: %#v", m.dataset.GpuPools)
	}
}

func TestHandleGpuNodesLoaded(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	m.gen = 1
	items := map[string][]models.GpuNode{"pool": {{Name: "node1"}}}

	m.handleGpuNodesLoaded(items, 1)
	if got := m.dataset.GpuNodeMap["pool"]; len(got) != 1 || got[0].Name != "node1" {
		t.Fatalf("GpuNodeMap not updated: %#v", m.dataset.GpuNodeMap)
	}
}

func TestHandleDedicatedAIClustersLoaded(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	m.gen = 1
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
	m.gen = 1
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
	m.gen = 1

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

func TestApplyDataset_ResetsFilter(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	m.gen = 1
	m.curFilter = "old"
	m.category = domain.Tenant

	m.applyDataset(func(ds *models.Dataset) {
		ds.Tenants = []models.Tenant{{Name: "tenant1"}}
	}, domain.Tenant, 1)

	if m.curFilter != "" {
		t.Fatalf("expected curFilter reset, got %q", m.curFilter)
	}
}

func TestGetCompartmentID_FromDataset(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	m.dataset.GpuNodeMap = map[string][]models.GpuNode{
		"pool": {{CompartmentID: "ocid1.compartment"}},
	}

	got, err := m.getCompartmentID(context.Background())
	if err != nil {
		t.Fatalf("getCompartmentID error: %v", err)
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

func TestOpContext(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	ctx, cancel := m.opContext()
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
