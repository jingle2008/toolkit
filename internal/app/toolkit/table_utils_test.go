package toolkit

import (
	"testing"

	"github.com/charmbracelet/bubbles/table"
	"github.com/jingle2008/toolkit/pkg/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_getHeaders_returns_expected_headers(t *testing.T) {
	t.Parallel()
	headers := getHeaders(Tenant)
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
	rows := getTenants(tenants, "")
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
	row := getTableRow("TenantX", cluster)
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
	rows := getEnvironments(envs, "")
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
	rows := getServiceTenancies(tenancies, "")
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
	key := getItemKey(DedicatedAICluster, row)
	assert.Equal(t, models.ScopedItemKey{Scope: "TenantX", Name: "DAC1"}, key)
	keyStr := getItemKeyString(DedicatedAICluster, key)
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
	item := findItem(dataset, Tenant, key)
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
					Values: []struct {
						Min int `json:"min"`
						Max int `json:"max"`
					}{{Min: 1, Max: 10}},
				},
			},
		},
	}
	rows := getTableRows(dataset, LimitTenancyOverride, &AppContext{Name: "TenantA", Category: Tenant}, "")
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
	row := getTableRow("TenantX", node)
	assert.Equal(t, table.Row{"poolA", "node1", "A100.8", "8", "6", "true", "true", "OK"}, row)

	// LimitTenancyOverride
	lto := models.LimitTenancyOverride{
		Name:    "LimitA",
		Regions: []string{"us-phoenix-1"},
		Values: []struct {
			Min int `json:"min"`
			Max int `json:"max"`
		}{{Min: 1, Max: 10}},
	}
	row2 := getTableRow("TenantA", lto)
	assert.Equal(t, table.Row{"TenantA", "LimitA", "us-phoenix-1", "1", "10"}, row2)

	// PropertyRegionalOverride edge: empty regions and values
	pro := models.PropertyRegionalOverride{
		Name:    "PropX",
		Regions: []string{},
		Values: []struct {
			Value string "json:\"value\""
		}{{Value: "valX"}},
	}
	row3 := getTableRow("TenantA", pro)
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
	_ = getTableRows(nil, Tenant, nil, "")
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
	// Table-driven: category, row, expected string
	tests := []struct {
		category Category
		row      table.Row
		keyStr   string
	}{
		{Tenant, table.Row{"tenant1"}, "tenant1"},
		{LimitDefinition, table.Row{"limdef"}, "limdef"},
		{ConsolePropertyDefinition, table.Row{"cpdef"}, "cpdef"},
		{PropertyDefinition, table.Row{"pdef"}, "pdef"},
		{LimitTenancyOverride, table.Row{"tenant1", "limdef"}, "tenant1/limdef"},
		{ConsolePropertyTenancyOverride, table.Row{"tenant1", "cpdef"}, "tenant1/cpdef"},
		{PropertyTenancyOverride, table.Row{"tenant1", "pdef"}, "tenant1/pdef"},
		{ConsolePropertyRegionalOverride, table.Row{"cpdef"}, "cpdef"},
		{PropertyRegionalOverride, table.Row{"pdef"}, "pdef"},
		{BaseModel, table.Row{"bm", "v1", "type"}, "bm-v1-type"},
		{ModelArtifact, table.Row{"model", "gpu", "artifact"}, "artifact"},
		{Environment, table.Row{"env"}, "env"},
		{ServiceTenancy, table.Row{"svc"}, "svc"},
		{GpuPool, table.Row{"pool"}, "pool"},
		{GpuNode, table.Row{"pool", "node"}, "pool/node"},
		{DedicatedAICluster, table.Row{"tenant1", "dac"}, "tenant1/dac"},
	}
	for _, tt := range tests {
		key := getItemKey(tt.category, tt.row)
		str := getItemKeyString(tt.category, key)
		require.Equal(t, tt.keyStr, str, "category %v", tt.category)
	}
}

func TestGetHeadersAndTableRows(t *testing.T) {
	// Cover all categories for getHeaders and getTableRows
	categories := []Category{
		Tenant, LimitDefinition, ConsolePropertyDefinition, PropertyDefinition,
		LimitTenancyOverride, ConsolePropertyTenancyOverride, PropertyTenancyOverride,
		ConsolePropertyRegionalOverride, PropertyRegionalOverride, BaseModel, ModelArtifact,
		Environment, ServiceTenancy, GpuPool, GpuNode, DedicatedAICluster,
	}
	ds := &models.Dataset{}
	for _, cat := range categories {
		_ = getHeaders(cat)
		_ = getTableRows(ds, cat, nil, "")
	}
}

func TestGetTableRow(t *testing.T) {
	// Each supported type should yield a non-nil row
	// Use the actual type for Values field from the model
	ltov := models.LimitTenancyOverride{}
	require.NotNil(t, getTableRow("tenant", models.LimitTenancyOverride{
		Name:    "lim",
		Regions: []string{"us"},
		Values: append(ltov.Values[:0], struct {
			Min int "json:\"min\""
			Max int "json:\"max\""
		}{Min: 1, Max: 2}),
	}))
	// Use the actual type for Values field from the model
	cprov := models.ConsolePropertyRegionalOverride{}
	require.NotNil(t, getTableRow("tenant", models.ConsolePropertyTenancyOverride{
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
	require.NotNil(t, getTableRow("tenant", models.PropertyTenancyOverride{
		Tag: "tenant1",
		PropertyRegionalOverride: models.PropertyRegionalOverride{
			Name:    "p",
			Regions: []string{"us"},
			Values: append(prov.Values[:0], struct {
				Value string "json:\"value\""
			}{Value: "val"}),
		},
	}))
	require.NotNil(t, getTableRow("pool", models.GpuNode{
		NodePool: "pool", Name: "node", InstanceType: "type", Allocatable: 10, Allocated: 2, IsHealthy: true, IsReady: true,
	}))
	require.NotNil(t, getTableRow("tenant", models.DedicatedAICluster{
		Name: "dac", Type: "t", UnitShape: "shape", Size: 1, Status: "active",
	}))
}
