package toolkit

import (
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/table"
	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/internal/domain/environment"
	"github.com/jingle2008/toolkit/internal/domain/rows"
	"github.com/jingle2008/toolkit/internal/domain/service"
	"github.com/jingle2008/toolkit/internal/domain/tenant"
	"github.com/jingle2008/toolkit/pkg/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_getHeaders_returns_expected_headers(t *testing.T) {
	t.Parallel()
	headers := getHeaders(domain.Tenant)
	assert.NotNil(t, headers)
	assert.Equal(t, "Name", headers[0].text)
	assert.InEpsilon(t, 0.25, headers[0].ratio, 0.0001)
}

func Test_getTenants_returns_rows(t *testing.T) {
	t.Parallel()
	tenants := []models.Tenant{
		{
			Name:                     "TenantA",
			IDs:                      []string{"idA"},
			LimitOverrides:           1,
			ConsolePropertyOverrides: 2,
			PropertyOverrides:        3,
		},
		{
			Name:                     "TenantB",
			IDs:                      []string{"idB", "idB2"},
			LimitOverrides:           4,
			ConsolePropertyOverrides: 5,
			PropertyOverrides:        6,
		},
	}
	tenantStructs := tenant.Filter(tenants, "")
	rows := make([]table.Row, 0, len(tenantStructs))
	for _, val := range tenantStructs {
		rows = append(rows, table.Row{
			val.Name,
			val.GetTenantID(),
			val.GetOverrides(),
		})
	}
	assert.Len(t, rows, 2)
	assert.Equal(t, table.Row{"TenantA", "idA", "1/2/3"}, rows[0])
	assert.Equal(t, table.Row{"TenantB", "idB (+1)", "4/5/6"}, rows[1])
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
	row := rows.GetTableRow(nil, "TenantX", cluster)
	assert.Equal(t, table.Row{"TenantX", "DAC1", "GPU", "A100", "4", "Active"}, row)
}

func Test_getEnvironments_returns_rows(t *testing.T) {
	t.Parallel()
	envs := []models.Environment{
		{
			Type:   "dev",
			Region: "us-phoenix-1",
			Realm:  "realmA",
		},
		{
			Type:   "prod",
			Region: "us-ashburn-1",
			Realm:  "realmB",
		},
	}
	envStructs := environment.Filter(envs, "")
	rows := make([]table.Row, 0, len(envStructs))
	for _, val := range envStructs {
		rows = append(rows, table.Row{
			val.GetName(),
			val.Realm,
			val.Type,
			val.Region,
		})
	}
	assert.Len(t, rows, 2)
	assert.Equal(t, table.Row{"dev-phx", "realmA", "dev", "us-phoenix-1"}, rows[0])
	assert.Equal(t, table.Row{"prod-iad", "realmB", "prod", "us-ashburn-1"}, rows[1])
}

func Test_getLimitDefinitions_returns_rows(t *testing.T) {
	t.Parallel()
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
	rows := getLimitDefinitions(ldg, "")
	assert.Len(t, rows, 2)
	assert.Equal(t, table.Row{"LimitA", "descA", "scopeA", "1", "10"}, rows[0])
	assert.Equal(t, table.Row{"LimitB", "descB", "scopeB", "2", "20"}, rows[1])
}

func Test_getPropertyDefinitions_returns_rows(t *testing.T) {
	t.Parallel()
	defs := []mockDefinition{
		{name: "PropA", desc: "descA", value: "valA"},
		{name: "PropB", desc: "descB", value: "valB"},
	}
	rows := getPropertyDefinitions(defs, "")
	assert.Len(t, rows, 2)
	assert.Equal(t, table.Row{"PropA", "descA", "valA"}, rows[0])
	assert.Equal(t, table.Row{"PropB", "descB", "valB"}, rows[1])
}

func Test_getServiceTenancies_returns_rows(t *testing.T) {
	t.Parallel()
	tenancies := []models.ServiceTenancy{
		{
			Name:        "svcA",
			Realm:       "realmA",
			Environment: "envA",
			HomeRegion:  "us-phoenix-1",
			Regions:     []string{"us-phoenix-1", "us-ashburn-1"},
		},
		{
			Name:        "svcB",
			Realm:       "realmB",
			Environment: "envB",
			HomeRegion:  "us-ashburn-1",
			Regions:     []string{"us-ashburn-1"},
		},
	}
	tenancyStructs := service.Filter(tenancies, "")
	rows := make([]table.Row, 0, len(tenancyStructs))
	for _, val := range tenancyStructs {
		rows = append(rows, table.Row{
			val.Name,
			val.Realm,
			val.Environment,
			val.HomeRegion,
			strings.Join(val.Regions, ", "),
		})
	}
	assert.Len(t, rows, 2)
	assert.Equal(t, table.Row{"svcA", "realmA", "envA", "us-phoenix-1", "us-phoenix-1, us-ashburn-1"}, rows[0])
	assert.Equal(t, table.Row{"svcB", "realmB", "envB", "us-ashburn-1", "us-ashburn-1"}, rows[1])
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
			Name:      "BM1",
			Version:   "v1",
			Type:      "typeA",
			Category:  "catA",
			MaxTokens: 1024,
			Capabilities: map[string]*models.Capability{
				"cap1": {
					Replicas: 2,
					ChartValues: &models.ChartValues{
						ModelMetaData: &models.ModelMetaData{
							DacShapeConfigs: &models.DacShapeConfigs{
								CompatibleDACShapes: []models.DACShape{
									{Name: "A100", QuotaUnit: 2, Default: true},
								},
							},
						},
					},
				},
				"cap2": {
					Replicas: 0,
				},
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
		"BM1", "v1", "typeA", "2x A100", "[2] cap1, cap2", "catA", "1024", "EXP/INT/LTS/RTD",
	}, rows[0])
}

func Test_getModelArtifacts_returns_rows(t *testing.T) {
	t.Parallel()
	artifacts := []models.ModelArtifact{
		{
			ModelName:       "M1",
			Name:            "artifactA",
			TensorRTVersion: "8.0",
			GpuCount:        2,
			GpuShape:        "A100",
		},
	}
	rows := getModelArtifacts(artifacts, "")
	assert.Len(t, rows, 1)
	assert.Equal(t, table.Row{"M1", "2x A100", "artifactA", "8.0"}, rows[0])
}

func Test_getItemKey_and_getItemKeyString(t *testing.T) {
	t.Parallel()
	row := table.Row{"TenantX", "DAC1", "GPU", "A100", "4", "Active"}
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
					Name:    "LimitA",
					Regions: []string{"us-phoenix-1"},
					Values:  []models.LimitRange{{Min: 1, Max: 10}},
				},
			},
		},
	}
	rows := getTableRows(nil, dataset, domain.LimitTenancyOverride, &domain.ToolkitContext{Name: "TenantA", Category: domain.Tenant}, "")
	assert.Len(t, rows, 1)
	assert.Equal(t, table.Row{"TenantA", "LimitA", "us-phoenix-1", "1", "10"}, rows[0])

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
	row := rows.GetTableRow(nil, "TenantX", node)
	assert.Equal(t, table.Row{"poolA", "node1", "A100.8", "8", "6", "true", "true", "OK"}, row)

	// LimitTenancyOverride
	lto := models.LimitTenancyOverride{
		Name:    "LimitA",
		Regions: []string{"us-phoenix-1"},
		Values:  []models.LimitRange{{Min: 1, Max: 10}},
	}
	row2 := rows.GetTableRow(nil, "TenantA", lto)
	assert.Equal(t, table.Row{"TenantA", "LimitA", "us-phoenix-1", "1", "10"}, row2)

	// PropertyRegionalOverride edge: empty regions and values
	pro := models.PropertyRegionalOverride{
		Name:    "PropX",
		Regions: []string{},
		Values: []struct {
			Value string "json:\"value\""
		}{{Value: "valX"}},
	}
	row3 := rows.GetTableRow(nil, "TenantA", pro)
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
	_ = getTableRows(nil, nil, domain.Tenant, nil, "")
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
		{domain.LimitTenancyOverride, table.Row{"tenant1", "limdef"}, "tenant1/limdef"},
		{domain.ConsolePropertyTenancyOverride, table.Row{"tenant1", "cpdef"}, "tenant1/cpdef"},
		{domain.PropertyTenancyOverride, table.Row{"tenant1", "pdef"}, "tenant1/pdef"},
		{domain.ConsolePropertyRegionalOverride, table.Row{"cpdef"}, "cpdef"},
		{domain.PropertyRegionalOverride, table.Row{"pdef"}, "pdef"},
		{domain.BaseModel, table.Row{"bm", "v1", "type"}, "bm-v1-type"},
		{domain.ModelArtifact, table.Row{"model", "gpu", "artifact"}, "artifact"},
		{domain.Environment, table.Row{"env"}, "env"},
		{domain.ServiceTenancy, table.Row{"svc"}, "svc"},
		{domain.GpuPool, table.Row{"pool"}, "pool"},
		{domain.GpuNode, table.Row{"pool", "node"}, "pool/node"},
		{domain.DedicatedAICluster, table.Row{"tenant1", "dac"}, "tenant1/dac"},
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
		_ = getTableRows(nil, ds, cat, nil, "")
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
			"bm1": {Name: "bm1", Version: "v1", Type: "typeA"},
		},
		ModelArtifacts: []models.ModelArtifact{{ModelName: "bm1", Name: "artifact1"}},
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
			"tenant1": {{Name: "limdef", Regions: []string{"us"}, Values: []models.LimitRange{{Min: 1, Max: 2}}}},
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
	for cat := domain.Category(0); cat <= domain.DedicatedAICluster; cat++ {
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
		_ = getTableRows(nil, ds, cat, nil, "")
	}
}

// --- Added: Comprehensive findItem test for all categories ---

func TestFindItem_AllCategories(t *testing.T) {
	t.Parallel()
	ds := buildFullTestDataset()

	// Table-driven: category, key, want
	tests := []struct {
		category domain.Category
		key      interface{}
		want     interface{}
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
		{domain.BaseModel, models.BaseModelKey{Name: "bm1", Version: "v1", Type: "typeA"}, ds.BaseModelMap["bm1"]},
		{domain.ModelArtifact, "artifact1", &ds.ModelArtifacts[0]},
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
	require.NotNil(t, rows.GetTableRow(nil, "tenant", models.LimitTenancyOverride{
		Name:    "lim",
		Regions: []string{"us"},
		Values: append(ltov.Values[:0], struct {
			Min int "json:\"min\""
			Max int "json:\"max\""
		}{Min: 1, Max: 2}),
	}))
	// Use the actual type for Values field from the model
	cprov := models.ConsolePropertyRegionalOverride{}
	require.NotNil(t, rows.GetTableRow(nil, "tenant", models.ConsolePropertyTenancyOverride{
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
	require.NotNil(t, rows.GetTableRow(nil, "tenant", models.PropertyTenancyOverride{
		Tag: "tenant1",
		PropertyRegionalOverride: models.PropertyRegionalOverride{
			Name:    "p",
			Regions: []string{"us"},
			Values: append(prov.Values[:0], struct {
				Value string "json:\"value\""
			}{Value: "val"}),
		},
	}))
	require.NotNil(t, rows.GetTableRow(nil, "pool", models.GpuNode{
		NodePool: "pool", Name: "node", InstanceType: "type", Allocatable: 10, Allocated: 2, IsHealthy: true, IsReady: true,
	}))
	require.NotNil(t, rows.GetTableRow(nil, "tenant", models.DedicatedAICluster{
		Name: "dac", Type: "t", UnitShape: "shape", Size: 1, Status: "active",
	}))
}
