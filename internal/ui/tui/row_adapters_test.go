package tui

import (
	"testing"

	"github.com/jingle2008/toolkit/internal/domain"
	logging "github.com/jingle2008/toolkit/internal/infra/logging"
	"github.com/jingle2008/toolkit/pkg/models"
	"go.uber.org/zap"
)

func TestLimitTenancyOverrideRow_ToRow(t *testing.T) {
	t.Parallel()
	row := LimitTenancyOverrideRow(models.LimitTenancyOverride{
		Name:    "limit",
		Regions: []string{"us-west", "us-east"},
		Values:  []models.LimitRange{{Min: 1, Max: 10}},
	}).ToRow("scope")
	if row[0] != "scope" || row[1] != "limit" || row[2] != "us-west, us-east" || row[3] != "1" || row[4] != "10" {
		t.Errorf("unexpected row: %v", row)
	}
}

func TestGetTableRow_GpuNodeAndDedicatedAICluster(t *testing.T) {
	t.Parallel()
	logger := logging.NewNoOpLogger()
	tenant := "scope"

	node := models.GpuNode{
		NodePool:     "pool",
		Name:         "node1",
		InstanceType: "typeA",
		Allocatable:  8,
		Allocated:    3,
		IsHealthy:    true,
		IsReady:      false,
	}
	row := GetTableRow(logger, tenant, node)
	if row[0] != "pool" || row[1] != "node1" {
		t.Errorf("GpuNode dispatch failed: %v", row)
	}

	cluster := models.DedicatedAICluster{
		Name:      "dac1",
		Type:      "GPU",
		UnitShape: "shapeA",
		Profile:   "",
		Size:      4,
		Status:    "Active",
	}
	row = GetTableRow(logger, tenant, cluster)
	if row[0] != tenant || row[1] != "dac1" {
		t.Errorf("DedicatedAICluster dispatch failed: %v", row)
	}
}

func TestGetScopedItems(t *testing.T) {
	t.Parallel()
	// Use LimitTenancyOverride as the NamedFilterable type
	logger := logging.NewNoOpLogger()
	m := map[string][]models.LimitTenancyOverride{
		"scope1": {{
			Name:    "limitA",
			Regions: []string{"us-west"},
			Values:  []models.LimitRange{{Min: 1, Max: 2}},
		}},
	}
	ctx := &domain.ToolkitContext{Name: "scope1", Category: domain.Tenant}
	rows := GetScopedItems(logger, m, domain.Tenant, ctx, "")
	if len(rows) != 1 || rows[0][0] != "scope1" || rows[0][1] != "limitA" {
		t.Errorf("unexpected GetScopedItems result: %v", rows)
	}
}

func TestConsolePropertyTenancyOverrideRow_ToRow(t *testing.T) {
	t.Parallel()
	vals := []struct {
		Value string `json:"value"`
	}{{Value: "v"}}
	row := ConsolePropertyTenancyOverrideRow(models.ConsolePropertyTenancyOverride{
		TenantID: "tid",
		ConsolePropertyRegionalOverride: models.ConsolePropertyRegionalOverride{
			Name:    "cp",
			Regions: []string{"r1"},
			Values:  vals,
		},
	}).ToRow("scope")
	if row[0] != "scope" || row[1] != "cp" || row[2] != "r1" || row[3] != "v" {
		t.Errorf("unexpected row: %v", row)
	}
}

func TestPropertyTenancyOverrideRow_ToRow(t *testing.T) {
	t.Parallel()
	vals := []struct {
		Value string `json:"value"`
	}{{Value: "val"}}
	row := PropertyTenancyOverrideRow(models.PropertyTenancyOverride{
		Tag: "tag",
		PropertyRegionalOverride: models.PropertyRegionalOverride{
			Name:    "p",
			Regions: []string{"r2"},
			Values:  vals,
		},
	}).ToRow("scope")
	if row[0] != "scope" || row[1] != "p" || row[2] != "r2" || row[3] != "val" {
		t.Errorf("unexpected row: %v", row)
	}
}

func TestGpuNodeRow_ToRow(t *testing.T) {
	t.Parallel()
	row := GpuNodeRow(models.GpuNode{
		NodePool:     "pool",
		Name:         "node1",
		InstanceType: "typeA",
		Allocatable:  8,
		Allocated:    3,
		IsHealthy:    true,
		IsReady:      false,
	}).ToRow("")
	// The last column is GetStatus(), which may depend on IsHealthy/IsReady.
	if row[0] != "pool" || row[1] != "node1" || row[2] != "typeA" || row[3] != "8" || row[4] != "5" || row[5] != "true" || row[6] != "false" {
		t.Errorf("unexpected row: %v", row)
	}
}

func TestDedicatedAIClusterRow_ToRow(t *testing.T) { //nolint:cyclop
	t.Parallel()
	row := DedicatedAIClusterRow(models.DedicatedAICluster{
		Name:      "dac1",
		Type:      "GPU",
		UnitShape: "shapeA",
		Profile:   "",
		Size:      4,
		Status:    "Active",
	}).ToRow("scope")
	if row[0] != "scope" || row[1] != "dac1" || row[2] != "GPU" || row[3] != "shapeA" || row[4] != "4" || row[5] != "Active" {
		t.Errorf("unexpected row: %v", row)
	}
	// Test branch where UnitShape == "" and Profile is used
	row2 := DedicatedAIClusterRow(models.DedicatedAICluster{
		Name:      "dac2",
		Type:      "GPU",
		UnitShape: "",
		Profile:   "profileA",
		Size:      2,
		Status:    "Inactive",
	}).ToRow("scope2")
	if row2[0] != "scope2" || row2[1] != "dac2" || row2[2] != "GPU" || row2[3] != "profileA" || row2[4] != "2" || row2[5] != "Inactive" {
		t.Errorf("unexpected row2: %v", row2)
	}
}

func TestGetTableRow_UnexpectedType(t *testing.T) {
	t.Parallel()
	logger := logging.NewNoOpLogger()
	row := GetTableRow(logger, "tenant", 12345)
	if row != nil {
		t.Errorf("expected nil for unexpected type, got %v", row)
	}
}

func TestGetScopedItems_NilCtxAndNonMatchingCategory(t *testing.T) {
	t.Parallel()
	logger := logging.NewZapLogger(zap.NewNop().Sugar(), false)
	m := map[string][]models.LimitTenancyOverride{
		"scope1": {{
			Name:    "limitA",
			Regions: []string{"us-west"},
			Values:  []models.LimitRange{{Min: 1, Max: 2}},
		}},
	}
	// ctx == nil
	rows := GetScopedItems(logger, m, domain.Tenant, nil, "")
	if len(rows) != 1 || rows[0][0] != "scope1" {
		t.Errorf("unexpected GetScopedItems result for nil ctx: %v", rows)
	}
	// ctx.Category != scopeCategory
	ctx := &domain.ToolkitContext{Name: "scope1", Category: domain.LimitDefinition}
	rows2 := GetScopedItems(logger, m, domain.Tenant, ctx, "")
	if len(rows2) != 0 {
		t.Errorf("expected 0 rows for non-matching category, got: %v", rows2)
	}
}

func TestGetTableRow_Dispatches(t *testing.T) {
	t.Parallel()
	logger := logging.NewZapLogger(zap.NewNop().Sugar(), false)
	tenant := "scope"

	limit := models.LimitTenancyOverride{Name: "l", Regions: []string{"r"}, Values: []models.LimitRange{{Min: 1, Max: 2}}}
	row := GetTableRow(logger, tenant, limit)
	if row[0] != tenant || row[1] != "l" {
		t.Errorf("LimitTenancyOverride dispatch failed: %v", row)
	}

	vals := []struct {
		Value string `json:"value"`
	}{{Value: "v"}}
	cp := models.ConsolePropertyTenancyOverride{
		TenantID: "tid",
		ConsolePropertyRegionalOverride: models.ConsolePropertyRegionalOverride{
			Name:    "cp",
			Regions: []string{"r"},
			Values:  vals,
		},
	}
	row = GetTableRow(logger, tenant, cp)
	if row[0] != tenant || row[1] != "cp" {
		t.Errorf("ConsolePropertyTenancyOverride dispatch failed: %v", row)
	}

	vals2 := []struct {
		Value string `json:"value"`
	}{{Value: "val"}}
	prop := models.PropertyTenancyOverride{
		Tag: "tag",
		PropertyRegionalOverride: models.PropertyRegionalOverride{
			Name:    "p",
			Regions: []string{"r"},
			Values:  vals2,
		},
	}
	row = GetTableRow(logger, tenant, prop)
	if row[0] != tenant || row[1] != "p" {
		t.Errorf("PropertyTenancyOverride dispatch failed: %v", row)
	}
}
