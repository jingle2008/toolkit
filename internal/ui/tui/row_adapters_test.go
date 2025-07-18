package tui

import (
	"testing"

	"github.com/jingle2008/toolkit/internal/domain"
	logging "github.com/jingle2008/toolkit/pkg/infra/logging"
	"github.com/jingle2008/toolkit/pkg/models"
	"go.uber.org/zap"
)

func TestLimitTenancyOverrideRow_ToRow(t *testing.T) {
	t.Parallel()
	row := LimitTenancyOverrideRow(models.LimitTenancyOverride{
		LimitRegionalOverride: models.LimitRegionalOverride{
			Name:    "limit",
			Regions: []string{"us-west", "us-east"},
			Values:  []models.LimitRange{{Min: 1, Max: 10}},
		},
	}).ToRow("scope")
	if row[0] != "limit" || row[1] != "scope" || row[2] != "us-west, us-east" || row[3] != "1" || row[4] != "10" {
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
	if row[0] != "node1" || row[1] != "pool" {
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
	if row[0] != "dac1" || row[1] != tenant || row[2] != "" || row[3] != "" {
		t.Errorf("DedicatedAICluster dispatch failed: %v", row)
	}
}

func TestGetScopedItems(t *testing.T) {
	t.Parallel()
	// Use LimitTenancyOverride as the NamedFilterable type
	logger := logging.NewNoOpLogger()
	m := map[string][]models.LimitTenancyOverride{
		"scope1": {{
			LimitRegionalOverride: models.LimitRegionalOverride{
				Name:    "limitA",
				Regions: []string{"us-west"},
				Values:  []models.LimitRange{{Min: 1, Max: 2}},
			},
		}},
	}
	ctx := &domain.ToolkitContext{Name: "scope1", Category: domain.Tenant}
	rows := GetScopedItems(logger, m, domain.Tenant, ctx, "", nil)
	if len(rows) != 1 || rows[0][0] != "limitA" || rows[0][1] != "scope1" {
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
	if row[0] != "cp" || row[1] != "scope" || row[2] != "r1" || row[3] != "v" {
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
	if row[0] != "p" || row[1] != "scope" || row[2] != "r2" || row[3] != "val" {
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
	if row[0] != "node1" || row[1] != "pool" || row[2] != "typeA" || row[3] != "8" || row[4] != "5" || row[5] != "true" || row[6] != "false" {
		t.Errorf("unexpected row: %v", row)
	}
}

//nolint:cyclop
func TestDedicatedAIClusterRow_ToRow(t *testing.T) {
	t.Parallel()
	t.Run("UnitShape branch", func(t *testing.T) {
		t.Parallel()
		row := DedicatedAIClusterRow(models.DedicatedAICluster{
			Name:      "dac1",
			Type:      "GPU",
			UnitShape: "shapeA",
			Profile:   "",
			Size:      4,
			Status:    "Active",
		}).ToRow("scope")
		if row[0] != "dac1" || row[1] != "scope" || row[2] != "" || row[3] != "" || row[4] != "GPU" || row[5] != "shapeA" || row[6] != "4" || row[7] != "" || row[8] != "Active" {
			t.Errorf("unexpected row: %v", row)
		}
	})
	t.Run("Profile branch", func(t *testing.T) {
		t.Parallel()
		row2 := DedicatedAIClusterRow(models.DedicatedAICluster{
			Name:      "dac2",
			Type:      "GPU",
			UnitShape: "",
			Profile:   "profileA",
			Size:      2,
			Status:    "Inactive",
		}).ToRow("scope2")
		if row2[0] != "dac2" || row2[1] != "scope2" || row2[2] != "" || row2[3] != "" || row2[4] != "GPU" || row2[5] != "profileA" || row2[6] != "2" || row2[7] != "" || row2[8] != "Inactive" {
			t.Errorf("unexpected row2: %v", row2)
		}
	})
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
			LimitRegionalOverride: models.LimitRegionalOverride{
				Name:    "limitA",
				Regions: []string{"us-west"},
				Values:  []models.LimitRange{{Min: 1, Max: 2}},
			},
		}},
	}
	// ctx == nil
	rows := GetScopedItems(logger, m, domain.Tenant, nil, "", nil)
	if len(rows) != 1 || rows[0][0] != "limitA" {
		t.Errorf("unexpected GetScopedItems result for nil ctx: %v", rows)
	}
	// ctx.Category != scopeCategory
	ctx := &domain.ToolkitContext{Name: "scope1", Category: domain.LimitDefinition}
	rows2 := GetScopedItems(logger, m, domain.Tenant, ctx, "", nil)
	if len(rows2) != 0 {
		t.Errorf("expected 0 rows for non-matching category, got: %v", rows2)
	}
}

func TestGetTableRow_Dispatches(t *testing.T) {
	t.Parallel()
	logger := logging.NewZapLogger(zap.NewNop().Sugar(), false)
	tenant := "scope"

	limit := models.LimitTenancyOverride{
		LimitRegionalOverride: models.LimitRegionalOverride{
			Name:    "l",
			Regions: []string{"r"},
			Values:  []models.LimitRange{{Min: 1, Max: 2}},
		},
	}
	row := GetTableRow(logger, tenant, limit)
	if row[0] != "l" || row[1] != tenant {
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
	if row[0] != "cp" || row[1] != tenant {
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
	if row[0] != "p" || row[1] != tenant {
		t.Errorf("PropertyTenancyOverride dispatch failed: %v", row)
	}
}
