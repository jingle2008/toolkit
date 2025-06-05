package rows

import (
	"testing"

	"github.com/jingle2008/toolkit/internal/app/domain"
	"github.com/jingle2008/toolkit/pkg/models"
	"go.uber.org/zap"
)

func TestGetScopedItems_KeyAndName(t *testing.T) {
	// Setup test data
	overrides := map[string][]models.LimitTenancyOverride{
		"tenantA": {
			{Name: "limit1", Regions: []string{"us-west"}, Values: []models.LimitRange{{Min: 1, Max: 10}}},
			{Name: "limit2", Regions: []string{"us-east"}, Values: []models.LimitRange{{Min: 2, Max: 20}}},
		},
		"tenantB": {
			{Name: "limit3", Regions: []string{"us-central"}, Values: []models.LimitRange{{Min: 3, Max: 30}}},
		},
	}
	scopeCategory := domain.Tenant
	ctxKey := &domain.AppContext{Category: scopeCategory, Name: "tenantA"}
	ctxName := &domain.AppContext{Category: domain.LimitTenancyOverride, Name: "limit3"}
	logger := zap.NewNop()

	// When ctx.Category == scopeCategory, key is set, so only tenantA is used
	rows := GetScopedItems(logger, overrides, scopeCategory, ctxKey, "")
	if len(rows) != 2 {
		t.Errorf("expected 2 rows for tenantA, got %d", len(rows))
	}
	if rows[0][1] != "limit1" || rows[1][1] != "limit2" {
		t.Errorf("unexpected rows for tenantA: %v", rows)
	}

	// When ctx.Category != scopeCategory, name is set, so only items with Name == ctx.Name are used
	rows = GetScopedItems(logger, overrides, scopeCategory, ctxName, "")
	if len(rows) != 1 {
		t.Errorf("expected 1 row for limit3, got %d", len(rows))
	}
	if rows[0][1] != "limit3" {
		t.Errorf("unexpected row for limit3: %v", rows[0])
	}

	// Test filter string (should match "us-west" region)
	rows = GetScopedItems(logger, overrides, scopeCategory, nil, "west")
	if len(rows) != 1 || rows[0][2] != "us-west" {
		t.Errorf("expected 1 row with region us-west, got %v", rows)
	}
}
