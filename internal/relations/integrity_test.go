package relations

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/pkg/models"
)

// Every scoped category has rows for the checks, and every parent has a
// name set to check against.
func TestChildren_CoverScopedCategories(t *testing.T) {
	t.Parallel()
	for _, cat := range domain.Categories {
		_, ok := children[cat]
		assert.Equalf(t, len(cat.ScopedBy()) > 0, ok, "%s: children entry iff scoped", cat)
	}
}

func TestParentNames_CoverParents(t *testing.T) {
	t.Parallel()
	for _, r := range domain.Relations() {
		_, ok := parentNames(&models.Dataset{}, r.Parent)
		assert.Truef(t, ok, "%s has no parentNames case", r.Parent)
	}
}

func checkByName(t *testing.T, checks []Check, name string) Check {
	t.Helper()
	for _, c := range checks {
		if c.Name == name {
			return c
		}
	}
	require.Failf(t, "no check", "%q", name)
	return Check{}
}

// integrityDataset is consistent except for the dangling rows each test
// asserts on.
func integrityDataset() *models.Dataset {
	acme := &models.Tenant{Name: "Acme", IDs: []string{"ocid1.tenancy.oc1..acme"}}
	return &models.Dataset{
		Tenants:                           []models.Tenant{*acme},
		LimitDefinitionGroup:              models.LimitDefinitionGroup{Values: []models.LimitDefinition{{Name: "l1"}}},
		LimitTenancyOverrideMap:           map[string][]models.LimitTenancyOverride{"Acme": {{LimitRegionalOverride: models.LimitRegionalOverride{Name: "l1"}}, {LimitRegionalOverride: models.LimitRegionalOverride{Name: "gone"}}}},
		LimitRegionalOverrides:            []models.LimitRegionalOverride{{Name: "l1"}, {Name: "also-gone"}},
		BaseModels:                        []models.BaseModel{{Name: "llama"}},
		ImportedModelMap:                  map[string][]models.ImportedModel{"Acme": {{BaseModel: models.BaseModel{Name: "amaaaaaa-im"}, Namespace: "amaaaaaa-dac1", TenantID: "acme", Owner: acme}}},
		ServingRuntimes:                   []models.ServingRuntime{{Name: "vllm"}},
		GPUPools:                          []models.GPUPool{{Name: "pool-a"}},
		GPUNodeMap:                        map[string][]models.GPUNode{"pool-a": {{Name: "node-a"}}, "pool-x": {{Name: "node-x"}}},
		DedicatedAIClusterMap:             map[string][]models.DedicatedAICluster{"Acme": {{Name: "amaaaaaa-dac1", TenantID: "acme", Owner: acme}}},
		ConsolePropertyTenancyOverrideMap: map[string][]models.ConsolePropertyTenancyOverride{},
		PropertyTenancyOverrideMap:        map[string][]models.PropertyTenancyOverride{},
		GPUWorkloadMap: map[string][]models.GPUWorkload{"node-a": {
			{Name: "pod-1", Namespace: "amaaaaaa-dac1", Runtime: "vllm", TenantID: "acme", Owner: acme},
			{Name: "pod-2", Namespace: "amaaaaaa-gone", Runtime: "srt-gone"},
			{Name: "pod-3", Namespace: "llama"}, // on-demand: not a DAC namespace
			{Name: "pod-4", Namespace: "llama", TenantID: "unknown-tenancy"},
		}},
		InferenceServiceMap: map[string][]models.InferenceService{
			"amaaaaaa-dac1": {{Name: "isvc-1", Namespace: "amaaaaaa-dac1", Type: models.InferenceServiceDedicated, Model: "llama", Runtime: "vllm"}},
			"amaaaaaa-gone": {{Name: "isvc-2", Namespace: "amaaaaaa-gone", Type: models.InferenceServiceDedicated, Model: "amaaaaaa-deleted"}},
			"llama":         {{Name: "isvc-3", Namespace: "llama", Type: models.InferenceServiceOnDemand, Model: "llama"}},
			"amaaaaaa-old":  {{Name: "isvc-4", Namespace: "amaaaaaa-old", Type: models.InferenceServiceDedicated, Model: "legacy-model", Runtime: "legacy-rt", Legacy: true}},
		},
	}
}

func TestCheckIntegrity(t *testing.T) {
	t.Parallel()
	checks := CheckIntegrity(integrityDataset())

	isvcDAC := checkByName(t, checks, "InferenceService → DedicatedAICluster")
	assert.Equal(t, 3, isvcDAC.Checked, "on-demand endpoints don't reference a DAC")
	assert.Equal(t, []Dangling{
		{Row: "amaaaaaa-gone/isvc-2", Missing: "amaaaaaa-gone"},
		{Row: "amaaaaaa-old/isvc-4", Missing: "amaaaaaa-old"},
	}, isvcDAC.Dangling)

	isvcIM := checkByName(t, checks, "InferenceService → ImportedModel")
	assert.Equal(t, []Dangling{{Row: "amaaaaaa-gone/isvc-2", Missing: "amaaaaaa-deleted"}}, isvcIM.Dangling)
	assert.Empty(t, checkByName(t, checks, "InferenceService → BaseModel").Dangling, "legacy models aren't checked")

	isvcSR := checkByName(t, checks, "InferenceService → ServingRuntime")
	assert.Empty(t, isvcSR.Dangling, "legacy runtimes aren't checked")
	assert.Equal(t, "2 endpoint(s) name no runtime; OME selects one at deploy time", isvcSR.Note)

	gwDAC := checkByName(t, checks, "GPUWorkload → DedicatedAICluster")
	assert.Equal(t, 2, gwDAC.Checked, "only DAC-named namespaces reference a DAC")
	assert.Equal(t, []Dangling{{Row: "amaaaaaa-gone/pod-2", Missing: "amaaaaaa-gone"}}, gwDAC.Dangling)
	assert.Equal(t, []Dangling{{Row: "amaaaaaa-gone/pod-2", Missing: "srt-gone"}},
		checkByName(t, checks, "GPUWorkload → ServingRuntime").Dangling)

	assert.Equal(t, []Dangling{{Row: "node-x", Missing: "pool-x"}}, checkByName(t, checks, "GPUNode → GPUPool").Dangling)
	assert.Equal(t, []Dangling{{Row: "Acme/gone", Missing: "gone"}},
		checkByName(t, checks, "LimitTenancyOverride → LimitDefinition").Dangling)
	assert.Equal(t, []Dangling{{Row: "also-gone", Missing: "also-gone"}},
		checkByName(t, checks, "LimitRegionalOverride → LimitDefinition").Dangling)

	for _, c := range checks {
		if c.Name == "tenancy resolution" {
			continue
		}
		assert.NotEqualf(t, domain.Tenant, c.Parent, "%s: Tenant relations are covered by the tenancy check", c.Name)
		assert.Equal(t, SeverityFail, c.Severity, c.Name)
	}
}

func TestCheckIntegrity_Tenancy(t *testing.T) {
	t.Parallel()
	checks := CheckIntegrity(integrityDataset())
	tc := checks[len(checks)-1]
	assert.Equal(t, "tenancy resolution", tc.Name)
	assert.Equal(t, SeverityWarn, tc.Severity)
	assert.Equal(t, 4, tc.Checked, "rows with a tenancy-id")
	assert.Equal(t, []Dangling{{Row: "GPUWorkload llama/pod-4", Missing: "unknown-tenancy"}}, tc.Dangling)
}

// A consistent dataset has no dangling rows.
func TestCheckIntegrity_Clean(t *testing.T) {
	t.Parallel()
	ds := integrityDataset()
	ds.LimitTenancyOverrideMap["Acme"] = ds.LimitTenancyOverrideMap["Acme"][:1]
	ds.LimitRegionalOverrides = ds.LimitRegionalOverrides[:1]
	delete(ds.GPUNodeMap, "pool-x")
	ds.GPUWorkloadMap["node-a"] = ds.GPUWorkloadMap["node-a"][:1]
	delete(ds.InferenceServiceMap, "amaaaaaa-gone")
	delete(ds.InferenceServiceMap, "amaaaaaa-old")
	for _, c := range CheckIntegrity(ds) {
		assert.Emptyf(t, c.Dangling, "%s", c.Name)
	}
}

func TestDanglingRows(t *testing.T) {
	t.Parallel()
	ds := integrityDataset()
	isvc := func(ns, name string) models.InferenceService {
		for _, s := range ds.InferenceServiceMap[ns] {
			if s.Name == name {
				return s
			}
		}
		require.Failf(t, "no endpoint", "%s/%s", ns, name)
		return models.InferenceService{}
	}

	dangling := DanglingRows(ds, domain.InferenceService)
	require.NotNil(t, dangling)
	assert.True(t, dangling("amaaaaaa-gone", isvc("amaaaaaa-gone", "isvc-2")), "DAC and model missing")
	assert.True(t, dangling("amaaaaaa-old", isvc("amaaaaaa-old", "isvc-4")), "DAC missing")
	assert.False(t, dangling("amaaaaaa-dac1", isvc("amaaaaaa-dac1", "isvc-1")))
	assert.False(t, dangling("llama", isvc("llama", "isvc-3")), "on-demand: no DAC to miss")

	// A row's identity can include its group: a tenancy override is
	// tenant/name.
	limits := DanglingRows(ds, domain.LimitTenancyOverride)
	require.NotNil(t, limits)
	gone := ds.LimitTenancyOverrideMap["Acme"][1]
	assert.True(t, limits("Acme", gone))
	assert.False(t, limits("Other", gone))

	assert.Nil(t, DanglingRows(ds, domain.DedicatedAICluster), "Tenant relations (WARN) don't count")
	assert.Nil(t, DanglingRows(ds, domain.Tenant), "parents nothing")
	assert.Nil(t, DanglingRows(nil, domain.InferenceService))
}

// The TUI loads categories on demand: a relation whose parent isn't
// loaded, or loaded with sources missing, can't tell a missing parent
// from an unloaded one, so it isn't checked.
func TestDanglingRows_UnloadedParents(t *testing.T) {
	t.Parallel()
	ds := integrityDataset()
	ds.DedicatedAIClusterMap = nil
	dangling := DanglingRows(ds, domain.InferenceService)
	require.NotNil(t, dangling)
	old := ds.InferenceServiceMap["amaaaaaa-old"][0]
	assert.False(t, dangling("amaaaaaa-old", old), "only its DAC is missing, and DACs aren't loaded")
	gone := ds.InferenceServiceMap["amaaaaaa-gone"][0]
	assert.True(t, dangling("amaaaaaa-gone", gone), "its imported model is still missing")

	nodeX := ds.GPUNodeMap["pool-x"][0]
	require.NotNil(t, DanglingRows(ds, domain.GPUNode))
	assert.True(t, DanglingRows(ds, domain.GPUNode)("pool-x", nodeX))
	assert.Nil(t, DanglingRows(ds, domain.GPUNode, domain.GPUPool), "pools loaded partially")

	ds.GPUPools = nil
	assert.Nil(t, DanglingRows(ds, domain.GPUNode), "pools not loaded")
}
