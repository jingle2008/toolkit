package tui

import (
	"testing"

	"github.com/charmbracelet/bubbles/table"
	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/pkg/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_getHeaders_returns_expected_headers(t *testing.T) {
	t.Parallel()
	headers := getHeaders(domain.Tenant)
	assert.NotNil(t, headers)
	assert.Equal(t, "Name", headers[0].text)
	assert.InEpsilon(t, 0.20, headers[0].ratio, 0.0001)
}

func Test_getBaseModels_returns_rows(t *testing.T) {
	t.Parallel()
	baseModels := []models.BaseModel{
		{
			InternalName:   "bm1",
			Name:           "BM1",
			Version:        "v1",
			Type:           "typeA",
			MaxTokens:      1024,
			Capabilities:   []string{"cap1", "cap2"},
			IsExperimental: true,
			IsInternal:     true,
			LifeCyclePhase: "DEPRECATED",
		},
	}
	rows := filterRows(baseModels, "", false, baseModelToRow)
	assert.Len(t, rows, 1)
	assert.Equal(t, table.Row{
		"BM1", "", "v1", "", "", "1024", "EXP/INT/RTD", "",
	}, rows[0])
}

func Test_getModelArtifacts_returns_rows(t *testing.T) {
	t.Parallel()
	rows := getTableRows(&models.Dataset{
		ModelArtifactMap: map[string][]models.ModelArtifact{
			"M1": {
				{
					ModelName:       "M1",
					Name:            "artifactA",
					TensorRTVersion: "8.0",
					GpuCount:        2,
					GpuShape:        "A100",
				},
			},
		},
	}, domain.ModelArtifact, nil, "", "", true, false)
	assert.Len(t, rows, 1)
	assert.Equal(t, table.Row{"artifactA", "M1", "2x A100", "8.0"}, rows[0])
}

func Test_getItemKey_and_getItemKeyString(t *testing.T) {
	t.Parallel()
	row := table.Row{"DAC1", "TenantX", "GPU", "A100", "4", "Active"}
	key := getItemKey(domain.DedicatedAICluster, row)
	assert.Equal(t, models.ScopedItemKey{Scope: "TenantX", Name: "DAC1"}, key)
	keyStr := getItemKeyString(domain.DedicatedAICluster, key)
	assert.Equal(t, "TenantX/DAC1", keyStr)
}

func Test_findItem_returns_expected(t *testing.T) {
	t.Parallel()
	dataset := &models.Dataset{
		Tenants: []models.Tenant{
			{Name: "TenantA", IDs: []string{"idA"}},
		},
	}
	key := "TenantA"
	item := findItem(dataset, domain.Tenant, key)
	tenant, ok := item.(*models.Tenant)
	assert.True(t, ok)
	assert.NotNil(t, tenant)
	assert.Equal(t, "TenantA", tenant.Name)
}

func Test_getTableRows_empty_dataset(t *testing.T) {
	t.Parallel()
	// Should not panic or return rows for nil dataset
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("expected panic for nil dataset")
		}
	}()
	_ = getTableRows(nil, domain.Tenant, nil, "", "", true, false)
}

func TestGetItemKeyAndString(t *testing.T) {
	t.Parallel()
	// Table-driven: category, row, expected string
	tests := []struct {
		category domain.Category
		row      table.Row
		keyStr   string
	}{
		{domain.Tenant, table.Row{"tenant1"}, "tenant1"},
		{domain.LimitDefinition, table.Row{"limdef"}, "limdef"},
		{domain.ConsolePropertyDefinition, table.Row{"cpdef"}, "cpdef"},
		{domain.PropertyDefinition, table.Row{"pdef"}, "pdef"},
		{domain.LimitTenancyOverride, table.Row{"limdef", "tenant1"}, "tenant1/limdef"},
		{domain.ConsolePropertyTenancyOverride, table.Row{"cpdef", "tenant1"}, "tenant1/cpdef"},
		{domain.PropertyTenancyOverride, table.Row{"pdef", "tenant1"}, "tenant1/pdef"},
		{domain.ConsolePropertyRegionalOverride, table.Row{"cpdef"}, "cpdef"},
		{domain.PropertyRegionalOverride, table.Row{"pdef"}, "pdef"},
		{domain.BaseModel, table.Row{"BM1", "", "v1", "", "C,C*2", "1024", "EXP/INT/RTD", ""}, "BM1"},
		{domain.ModelArtifact, table.Row{"artifact", "gpu", "model"}, "artifact"},
		{domain.Environment, table.Row{"env"}, "env"},
		{domain.ServiceTenancy, table.Row{"svc"}, "svc"},
		{domain.GpuPool, table.Row{"pool"}, "pool"},
		{domain.GpuNode, table.Row{"node", "pool"}, "pool/node"},
		{domain.DedicatedAICluster, table.Row{"dac", "tenant1"}, "tenant1/dac"},
	}
	for _, tt := range tests {
		key := getItemKey(tt.category, tt.row)
		str := getItemKeyString(tt.category, key)
		require.Equal(t, tt.keyStr, str, "category %v", tt.category)
	}
}

func TestGetHeadersAndTableRows(t *testing.T) {
	t.Parallel()
	// Cover all categories for getHeaders and getTableRows
	categories := []domain.Category{
		domain.Tenant, domain.LimitDefinition, domain.ConsolePropertyDefinition, domain.PropertyDefinition,
		domain.LimitTenancyOverride, domain.ConsolePropertyTenancyOverride, domain.PropertyTenancyOverride,
		domain.ConsolePropertyRegionalOverride, domain.PropertyRegionalOverride, domain.BaseModel, domain.ModelArtifact,
		domain.Environment, domain.ServiceTenancy, domain.GpuPool, domain.GpuNode, domain.DedicatedAICluster,
	}
	ds := &models.Dataset{}
	for _, cat := range categories {
		headers := getHeaders(cat)
		_ = getTableRows(ds, cat, nil, "", "", true, false)
		// Extra assertions for coverage
		if len(headers) > 0 {
			require.NotEmpty(t, headers[0].text)
			require.Greater(t, headers[0].ratio, 0.0)
		}
	}
}

func buildFullTestDataset() *models.Dataset {
	return &models.Dataset{
		Tenants:          []models.Tenant{{Name: "tenant1"}},
		Environments:     []models.Environment{{Type: "type1", Region: "region1", Realm: "realm1"}},
		GpuPools:         []models.GpuPool{{Name: "pool1"}},
		GpuNodeMap:       map[string][]models.GpuNode{"pool1": {{NodePool: "pool1", Name: "node1"}}},
		ServiceTenancies: []models.ServiceTenancy{{Name: "svc1"}},
		BaseModels:       []models.BaseModel{{InternalName: "v1", Name: "bm1", Version: "v1", Type: "typeA"}},
		ModelArtifactMap: map[string][]models.ModelArtifact{
			"artifact1": {{ModelName: "bm1", Name: "artifact1"}},
		},
		LimitDefinitionGroup: models.LimitDefinitionGroup{
			Values: []models.LimitDefinition{{Name: "limdef"}},
		},
		ConsolePropertyDefinitionGroup: models.ConsolePropertyDefinitionGroup{
			Values: []models.ConsolePropertyDefinition{{Name: "cpdef"}},
		},
		PropertyDefinitionGroup: models.PropertyDefinitionGroup{
			Values: []models.PropertyDefinition{{Name: "pdef"}},
		},
		LimitTenancyOverrideMap: map[string][]models.LimitTenancyOverride{
			"tenant1": {{
				LimitRegionalOverride: models.LimitRegionalOverride{
					Name:    "limdef",
					Regions: []string{"us"},
					Values:  []models.LimitRange{{Min: 1, Max: 2}},
				},
			}},
		},
		ConsolePropertyTenancyOverrideMap: map[string][]models.ConsolePropertyTenancyOverride{
			"tenant1": {{
				TenantID: "tenant1",
				ConsolePropertyRegionalOverride: models.ConsolePropertyRegionalOverride{
					Name:    "cpdef",
					Regions: []string{"us"},
					Values: []struct {
						Value string `json:"value"`
					}{{Value: "val"}},
				},
			}},
		},
		PropertyTenancyOverrideMap: map[string][]models.PropertyTenancyOverride{
			"tenant1": {{
				Tag: "tenant1",
				PropertyRegionalOverride: models.PropertyRegionalOverride{
					Name:    "pdef",
					Regions: []string{"us"},
					Values: []struct {
						Value string `json:"value"`
					}{{Value: "val"}},
				},
			}},
		},
		ConsolePropertyRegionalOverrides: []models.ConsolePropertyRegionalOverride{
			{Name: "cpdef", Regions: []string{"us"}, Values: []struct {
				Value string `json:"value"`
			}{{Value: "val"}}},
		},
		PropertyRegionalOverrides: []models.PropertyRegionalOverride{
			{Name: "pdef", Regions: []string{"us"}, Values: []struct {
				Value string `json:"value"`
			}{{Value: "val"}}},
		},
		DedicatedAIClusterMap: map[string][]models.DedicatedAICluster{
			"tenant1": {{Name: "dac1", Type: "t", UnitShape: "shape", Size: 1, Status: "active"}},
		},
	}
}

func TestAllCategories_HeadersAndRows(t *testing.T) {
	t.Parallel()
	ds := buildFullTestDataset()
	// Iterate from Tenant to DedicatedAICluster (skip CategoryUnknown)
	for cat := domain.Tenant; cat <= domain.DedicatedAICluster; cat++ {
		headers := getHeaders(cat)
		if len(headers) > 0 {
			sum := 0.0
			for _, h := range headers {
				require.NotEmpty(t, h.text)
				require.Greater(t, h.ratio, 0.0)
				sum += h.ratio
			}
			require.InDelta(t, 1.0, sum, 0.1, "header ratios should sum to ~1")
		}
		// getTableRows should not panic
		_ = getTableRows(ds, cat, nil, "", "", true, false)
	}
}

// --- Added: Comprehensive findItem test for all categories ---

func TestFindItem_AllCategories(t *testing.T) {
	t.Parallel()
	ds := buildFullTestDataset()

	// Table-driven: category, key, want
	tests := []struct {
		category domain.Category
		key      any
		want     any
	}{
		{domain.Tenant, "tenant1", &ds.Tenants[0]},
		{domain.LimitDefinition, "limdef", &ds.LimitDefinitionGroup.Values[0]},
		{domain.ConsolePropertyDefinition, "cpdef", &ds.ConsolePropertyDefinitionGroup.Values[0]},
		{domain.PropertyDefinition, "pdef", &ds.PropertyDefinitionGroup.Values[0]},
		{domain.LimitTenancyOverride, models.ScopedItemKey{Scope: "tenant1", Name: "limdef"}, &ds.LimitTenancyOverrideMap["tenant1"][0]},
		{domain.ConsolePropertyTenancyOverride, models.ScopedItemKey{Scope: "tenant1", Name: "cpdef"}, &ds.ConsolePropertyTenancyOverrideMap["tenant1"][0]},
		{domain.PropertyTenancyOverride, models.ScopedItemKey{Scope: "tenant1", Name: "pdef"}, &ds.PropertyTenancyOverrideMap["tenant1"][0]},
		{domain.ConsolePropertyRegionalOverride, "cpdef", &ds.ConsolePropertyRegionalOverrides[0]},
		{domain.PropertyRegionalOverride, "pdef", &ds.PropertyRegionalOverrides[0]},
		{domain.BaseModel, "bm1", &ds.BaseModels[0]},
		{domain.ModelArtifact, "artifact1", &ds.ModelArtifactMap["artifact1"][0]},
		{domain.Environment, "type1-UNKNOWN", &ds.Environments[0]},
		{domain.ServiceTenancy, "svc1", &ds.ServiceTenancies[0]},
		{domain.GpuPool, "pool1", &ds.GpuPools[0]},
		{domain.GpuNode, models.ScopedItemKey{Scope: "pool1", Name: "node1"}, &ds.GpuNodeMap["pool1"][0]},
		{domain.DedicatedAICluster, models.ScopedItemKey{Scope: "tenant1", Name: "dac1"}, &ds.DedicatedAIClusterMap["tenant1"][0]},
	}

	for _, tt := range tests {
		got := findItem(ds, tt.category, tt.key)
		require.Equal(t, tt.want, got, "category %v", tt.category)
	}
}

func TestGetItemKey_EmptyRow(t *testing.T) {
	t.Parallel()
	var empty table.Row
	key := getItemKey(domain.Tenant, empty)
	require.Nil(t, key)
}

func TestGetItemKey_NilRow(t *testing.T) {
	t.Parallel()
	var nilRow table.Row
	key := getItemKey(domain.Tenant, nilRow)
	require.Nil(t, key)
}

func TestGetHeaders_KnownCategory(t *testing.T) {
	t.Parallel()
	headers := getHeaders(domain.Tenant)
	assert.NotNil(t, headers)
	assert.NotEmpty(t, headers)
}

func TestGetHeaders_UnknownCategory(t *testing.T) {
	t.Parallel()
	headers := getHeaders(domain.Category(9999))
	assert.Nil(t, headers)
}

func TestGetItemKeyString_Simple(t *testing.T) {
	t.Parallel()
	key := getItemKeyString(domain.Tenant, "foo")
	assert.Equal(t, "foo", key)
}

func TestGetItemKeyString_Scoped(t *testing.T) {
	t.Parallel()
	k := models.ScopedItemKey{Scope: "scope", Name: "name"}
	key := getItemKeyString(domain.LimitTenancyOverride, k)
	assert.Equal(t, "scope/name", key)
}

func TestFilterRows(t *testing.T) {
	t.Parallel()
	items := []models.Environment{
		{Type: "foo", Region: "us-phx-1"},
		{Type: "bar", Region: "us-ashburn-1"},
	}
	rows := filterRows(items, "foo", false, func(e models.Environment) table.Row {
		return table.Row{e.Type, e.Region}
	})
	assert.Len(t, rows, 1)
	assert.Equal(t, "foo", rows[0][0])
}

func TestGetTableRows_UnknownCategory(t *testing.T) {
	t.Parallel()
	rows := getTableRows(&models.Dataset{}, domain.Category(9999), nil, "", "", true, false)
	assert.Nil(t, rows)
}

func TestGetItemKey_AndFindItem(t *testing.T) {
	t.Parallel()
	row := table.Row{"foo"}
	key := getItemKey(domain.Tenant, row)
	assert.Equal(t, "foo", key)
	ds := &models.Dataset{Tenants: []models.Tenant{{Name: "foo"}}}
	item := findItem(ds, domain.Tenant, key)
	assert.NotNil(t, item)
}

func TestGetBaseModels_SortsAndFilters(t *testing.T) {
	t.Parallel()
	m := []models.BaseModel{
		{InternalName: "a", Name: "A"},
		{InternalName: "b", Name: "B"},
	}
	rows := filterRows(m, "a", false, baseModelToRow)
	assert.Len(t, rows, 1)
	assert.Contains(t, rows[0][0], "A")
}

func TestGetTableRows_AliasCategory(t *testing.T) {
	t.Parallel()
	dataset := &models.Dataset{}
	rows := getTableRows(dataset, domain.Alias, nil, "", "", true, false)
	assert.Equal(t, len(domain.Categories), len(rows), "should return one row per category")

	// Find GpuNode row
	found := false
	for _, row := range rows {
		if len(row) > 0 && row[0] == "GpuNode" {
			found = true
			break
		}
	}
	assert.True(t, found, "GpuNode row should be present")

	// Filtering
	filtered := getTableRows(dataset, domain.Alias, nil, "tenant", "", true, false)
	assert.Len(t, filtered, 1, "filter 'tenant' should return exactly one row")
	assert.Equal(t, "Tenant", filtered[0][0])
}
