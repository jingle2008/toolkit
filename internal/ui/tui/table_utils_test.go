package tui

import (
	"testing"

	"github.com/charmbracelet/bubbles/table"
	"github.com/jingle2008/toolkit/internal/domain"
	logging "github.com/jingle2008/toolkit/pkg/infra/logging"
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

func Test_getTableRow_DedicatedAICluster(t *testing.T) {
	t.Parallel()
	cluster := models.DedicatedAICluster{
		Name:      "DAC1",
		Type:      "GPU",
		Size:      4,
		Status:    "Active",
		UnitShape: "A100",
	}
	row := GetTableRow(nil, "TenantX", cluster)
	assert.Equal(t, table.Row{"DAC1", "TenantX", "", "", "GPU", "A100", "4", "", "Active"}, row)
}
func Test_getLimitRegionalOverrides(t *testing.T) {
	t.Parallel()
	overrides := []models.LimitRegionalOverride{
		{
			Name:    "LimitA",
			Regions: []string{"us-phoenix-1", "eu-frankfurt-1"},
			Values:  []models.LimitRange{{Min: 10, Max: 100}},
		},
		{
			Name:    "LimitB",
			Regions: []string{"us-ashburn-1"},
			Values:  []models.LimitRange{{Min: 5, Max: 50}},
		},
	}
	rows := getLimitRegionalOverrides(overrides, "")
	require.Len(t, rows, 2)
	require.Equal(t, table.Row{"LimitA", "us-phoenix-1, eu-frankfurt-1", "10", "100"}, rows[0])
	require.Equal(t, table.Row{"LimitB", "us-ashburn-1", "5", "50"}, rows[1])
}

func Test_rowGenerationFunctions_tableDriven(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		rowsFunc func() []table.Row
		want     []table.Row
	}{
		{
			name: "getLimitDefinitions",
			rowsFunc: func() []table.Row {
				ldg := models.LimitDefinitionGroup{
					Values: []models.LimitDefinition{
						{
							Name:        "LimitA",
							Description: "descA",
							Scope:       "scopeA",
							DefaultMin:  "1",
							DefaultMax:  "10",
						},
						{
							Name:        "LimitB",
							Description: "descB",
							Scope:       "scopeB",
							DefaultMin:  "2",
							DefaultMax:  "20",
						},
					},
				}
				return getLimitDefinitions(ldg, "")
			},
			want: []table.Row{
				{"LimitA", "descA", "scopeA", "1", "10"},
				{"LimitB", "descB", "scopeB", "2", "20"},
			},
		},
		{
			name: "getPropertyDefinitions",
			rowsFunc: func() []table.Row {
				defs := []mockDefinition{
					{name: "PropA", desc: "descA", value: "valA"},
					{name: "PropB", desc: "descB", value: "valB"},
				}
				return getPropertyDefinitions(defs, "")
			},
			want: []table.Row{
				{"PropA", "descA", "valA"},
				{"PropB", "descB", "valB"},
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			rows := tc.rowsFunc()
			assert.Len(t, rows, len(tc.want))
			for i, wantRow := range tc.want {
				assert.Equal(t, wantRow, rows[i])
			}
		})
	}
}

func Test_getGpuPools_returns_rows(t *testing.T) {
	t.Parallel()
	pools := []models.GpuPool{
		{
			Name:         "poolA",
			Shape:        "A100.2",
			Size:         2,
			IsOkeManaged: true,
			CapacityType: "on-demand",
		},
		{
			Name:         "poolB",
			Shape:        "A100.4",
			Size:         4,
			IsOkeManaged: false,
			CapacityType: "reserved",
		},
	}
	rows := getGpuPools(pools, "")
	assert.Len(t, rows, 2)
	assert.Equal(t, table.Row{"poolA", "A100.2", "2", "4", "true", "on-demand"}, rows[0])
	assert.Equal(t, table.Row{"poolB", "A100.4", "4", "16", "false", "reserved"}, rows[1])
}

func Test_getBaseModels_returns_rows(t *testing.T) {
	t.Parallel()
	baseModels := map[string]*models.BaseModel{
		"bm1": {
			InternalName: "bm1",
			Name:         "BM1",
			Version:      "v1",
			Type:         "typeA",
			Category:     "catA",
			MaxTokens:    1024,
			Capabilities: map[string]*models.Capability{
				"cap1": {Capability: "cap1", Replicas: 0},
				"cap2": {Capability: "cap2", Replicas: 2},
			},
			IsExperimental:      true,
			IsInternal:          true,
			IsLongTermSupported: true,
			LifeCyclePhase:      "DEPRECATED",
		},
	}
	rows := getBaseModels(baseModels, "")
	assert.Len(t, rows, 1)
	assert.Equal(t, table.Row{
		"BM1", "bm1", "v1", "", "C/C*2", "1024", "EXP/INT/LTS/RTD",
	}, rows[0])
}

func Test_getModelArtifacts_returns_rows(t *testing.T) {
	t.Parallel()
	rows := getTableRows(nil, &models.Dataset{
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

func Test_getTableRows_and_scoped_items(t *testing.T) {
	t.Parallel()
	// Test getTableRows for LimitTenancyOverride (uses getScopedItems)
	dataset := &models.Dataset{
		LimitTenancyOverrideMap: map[string][]models.LimitTenancyOverride{
			"TenantA": {
				{
					LimitRegionalOverride: models.LimitRegionalOverride{
						Name:    "LimitA",
						Regions: []string{"us-phoenix-1"},
						Values:  []models.LimitRange{{Min: 1, Max: 10}},
					},
				},
			},
		},
	}
	rows := getTableRows(nil, dataset, domain.LimitTenancyOverride, &domain.ToolkitContext{Name: "TenantA", Category: domain.Tenant}, "", "", true, false)
	assert.Len(t, rows, 1)
	assert.Equal(t, table.Row{"LimitA", "TenantA", "us-phoenix-1", "1", "10"}, rows[0])

	// Test getTableRows for ConsolePropertyRegionalOverride (uses getRegionalOverrides)
	overrides := []mockOverride{
		{name: "PropA", regions: []string{"us-phoenix-1"}, value: "valA"},
	}
	rows2 := getRegionalOverrides(overrides, "")
	assert.Len(t, rows2, 1)
	assert.Equal(t, table.Row{"PropA", "us-phoenix-1", "valA"}, rows2[0])
}

// mockOverride implements models.DefinitionOverride for getRegionalOverrides test
type mockOverride struct {
	name    string
	regions []string
	value   string
}

func (m mockOverride) GetName() string               { return m.name }
func (m mockOverride) GetRegions() []string          { return m.regions }
func (m mockOverride) GetValue() string              { return m.value }
func (m mockOverride) GetFilterableFields() []string { return []string{m.name, m.value} }

func Test_getTableRow_other_types(t *testing.T) {
	t.Parallel()
	// GpuNode
	node := models.GpuNode{
		NodePool:     "poolA",
		Name:         "node1",
		InstanceType: "A100.8",
		Allocatable:  8,
		Allocated:    2,
		IsHealthy:    true,
		IsReady:      true,
	}
	row := GetTableRow(nil, "TenantX", node)
	assert.Equal(t, table.Row{"node1", "poolA", "A100.8", "8", "6", "true", "true", "", "OK"}, row)

	// LimitTenancyOverride
	lto := models.LimitTenancyOverride{
		LimitRegionalOverride: models.LimitRegionalOverride{
			Name:    "LimitA",
			Regions: []string{"us-phoenix-1"},
			Values:  []models.LimitRange{{Min: 1, Max: 10}},
		},
	}
	row2 := GetTableRow(nil, "TenantA", lto)
	assert.Equal(t, table.Row{"LimitA", "TenantA", "us-phoenix-1", "1", "10"}, row2)

	// PropertyRegionalOverride edge: empty regions and values
	pro := models.PropertyRegionalOverride{
		Name:    "PropX",
		Regions: []string{},
		Values: []struct {
			Value string "json:\"value\""
		}{{Value: "valX"}},
	}
	row3 := GetTableRow(nil, "TenantA", pro)
	assert.Nil(t, row3)
}

func Test_getTableRows_empty_dataset(t *testing.T) {
	t.Parallel()
	// Should not panic or return rows for nil dataset
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("expected panic for nil dataset")
		}
	}()
	_ = getTableRows(nil, nil, domain.Tenant, nil, "", "", true, false)
}

// mockDefinition implements models.Definition for testing getPropertyDefinitions
type mockDefinition struct {
	name  string
	desc  string
	value string
}

func (m mockDefinition) GetName() string               { return m.name }
func (m mockDefinition) GetDescription() string        { return m.desc }
func (m mockDefinition) GetValue() string              { return m.value }
func (m mockDefinition) GetFilterableFields() []string { return []string{m.name, m.desc, m.value} }

// --- Extra merged tests below ---

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
		{domain.BaseModel, table.Row{"BM1", "bm1", "v1", "", "C,C*2", "1024", "EXP/INT/LTS/RTD"}, "bm1"},
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
		_ = getTableRows(nil, ds, cat, nil, "", "", true, false)
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
		BaseModelMap: map[string]*models.BaseModel{
			"bm1": {InternalName: "v1", Name: "bm1", Version: "v1", Type: "typeA"},
		},
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
		_ = getTableRows(nil, ds, cat, nil, "", "", true, false)
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
		{domain.BaseModel, "v1", ds.BaseModelMap["bm1"]},
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

func TestGetTableRow(t *testing.T) {
	t.Parallel()
	// Each supported type should yield a non-nil row
	// Use the actual type for Values field from the model
	ltov := models.LimitTenancyOverride{}
	require.NotNil(t, GetTableRow(nil, "tenant", models.LimitTenancyOverride{
		LimitRegionalOverride: models.LimitRegionalOverride{
			Name:    "lim",
			Regions: []string{"us"},
			Values: append(ltov.Values[:0], struct {
				Min int "json:\"min\""
				Max int "json:\"max\""
			}{Min: 1, Max: 2}),
		},
	}))
	// Use the actual type for Values field from the model
	cprov := models.ConsolePropertyRegionalOverride{}
	require.NotNil(t, GetTableRow(nil, "tenant", models.ConsolePropertyTenancyOverride{
		TenantID: "tenant1",
		ConsolePropertyRegionalOverride: models.ConsolePropertyRegionalOverride{
			Name:    "cp",
			Regions: []string{"us"},
			Values: append(cprov.Values[:0], struct {
				Value string "json:\"value\""
			}{Value: "val"}),
		},
	}))
	// Use the actual type for Values field from the model
	prov := models.PropertyRegionalOverride{}
	require.NotNil(t, GetTableRow(nil, "tenant", models.PropertyTenancyOverride{
		Tag: "tenant1",
		PropertyRegionalOverride: models.PropertyRegionalOverride{
			Name:    "p",
			Regions: []string{"us"},
			Values: append(prov.Values[:0], struct {
				Value string "json:\"value\""
			}{Value: "val"}),
		},
	}))
	require.NotNil(t, GetTableRow(nil, "pool", models.GpuNode{
		NodePool: "pool", Name: "node", InstanceType: "type", Allocatable: 10, Allocated: 2, IsHealthy: true, IsReady: true,
	}))
	require.NotNil(t, GetTableRow(nil, "tenant", models.DedicatedAICluster{
		Name: "dac", Type: "t", UnitShape: "shape", Size: 1, Status: "active",
	}))
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
	rows := filterRows(items, "foo", func(e models.Environment) table.Row {
		return table.Row{e.Type, e.Region}
	})
	assert.Len(t, rows, 1)
	assert.Equal(t, "foo", rows[0][0])
}

func TestGetTableRows_UnknownCategory(t *testing.T) {
	t.Parallel()
	rows := getTableRows(nil, &models.Dataset{}, domain.Category(9999), nil, "", "", true, false)
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
	m := map[string]*models.BaseModel{
		"a": {InternalName: "a", Name: "A"},
		"b": {InternalName: "b", Name: "B"},
	}
	rows := getBaseModels(m, "a")
	assert.Len(t, rows, 1)
	assert.Contains(t, rows[0][0], "A")
}

func TestGetTableRows_AliasCategory(t *testing.T) {
	t.Parallel()
	logger := logging.NewNoOpLogger()
	dataset := &models.Dataset{}
	rows := getTableRows(logger, dataset, domain.Alias, nil, "", "", true, false)
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
	filtered := getTableRows(logger, dataset, domain.Alias, nil, "tenant", "", true, false)
	assert.Len(t, filtered, 1, "filter 'tenant' should return exactly one row")
	assert.Equal(t, "Tenant", filtered[0][0])
}
