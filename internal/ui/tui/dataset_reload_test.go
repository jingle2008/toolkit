package tui

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jingle2008/toolkit/pkg/models"
)

func TestMergeReloadedRepoData_PreservesK8sFields(t *testing.T) {
	t.Parallel()
	d := &models.Dataset{
		Tenants:         []models.Tenant{{Name: "old"}},
		BaseModels:      []models.BaseModel{{Name: "bm1"}},
		ServingRuntimes: []models.ServingRuntime{{Name: "sr1"}},
		GPUPools:        []models.GPUPool{{Name: "p1"}},
		GPUNodeMap:      map[string][]models.GPUNode{"p1": {{Name: "n1"}}},
	}
	bm := d.BaseModels
	srs := d.ServingRuntimes
	pools := d.GPUPools
	nodes := d.GPUNodeMap

	fresh := &models.Dataset{
		Tenants:      []models.Tenant{{Name: "new1"}, {Name: "new2"}},
		Environments: []models.Environment{{Type: "dev"}},
		// k8s-backed fields left nil, as LoadDataset returns them
	}

	mergeReloadedRepoData(d, fresh)

	// Repo-owned fields are replaced by the freshly loaded values.
	require.Len(t, d.Tenants, 2)
	require.Equal(t, "new1", d.Tenants[0].Name)
	require.Len(t, d.Environments, 1)

	// Lazily-loaded k8s fields are preserved untouched.
	require.Equal(t, bm, d.BaseModels)
	require.Equal(t, srs, d.ServingRuntimes)
	require.Equal(t, pools, d.GPUPools)
	require.Equal(t, nodes, d.GPUNodeMap)
}

// liveFields are the lazily-loaded fields the TUI fetches on demand (k8s,
// plus GPUPools from terraform) rather than from LoadDataset. They must
// survive a repo reload and be dropped on an environment switch.
var liveFields = map[string]bool{
	"BaseModels": true, "ServingRuntimes": true, "ImportedModelMap": true,
	"GPUPools": true, "GPUNodeMap": true, "GPUWorkloadMap": true,
	"DedicatedAIClusterMap": true, "InferenceServiceMap": true,
}

// repoFields are populated by LoadDataset.
var repoFields = map[string]bool{
	"LimitDefinitionGroup": true, "ConsolePropertyDefinitionGroup": true, "PropertyDefinitionGroup": true,
	"ConsolePropertyTenancyOverrideMap": true, "LimitTenancyOverrideMap": true, "PropertyTenancyOverrideMap": true,
	"ConsolePropertyRegionalOverrides": true, "LimitRegionalOverrides": true, "PropertyRegionalOverrides": true,
	"Tenants": true, "ModelArtifactMap": true, "Environments": true, "ServiceTenancies": true,
}

// populatedDataset returns a Dataset whose every slice and map field is
// non-nil.
func populatedDataset(t *testing.T) *models.Dataset {
	t.Helper()
	d := &models.Dataset{}
	v := reflect.ValueOf(d).Elem()
	for i := range v.NumField() {
		f := v.Field(i)
		switch f.Kind() { //nolint:exhaustive // Dataset holds only slices, maps and structs
		case reflect.Slice:
			f.Set(reflect.MakeSlice(f.Type(), 1, 1))
		case reflect.Map:
			f.Set(reflect.MakeMap(f.Type()))
		}
	}
	return d
}

// A field forgotten in mergeReloadedRepoData is wiped by every repo file
// change; one forgotten in resetRealmScopedFields shows the previous
// cluster's rows after an environment switch (the category handlers only
// load when the field is nil). Both happened to ServingRuntimes.
func TestDataset_LiveFieldsDriftGuard(t *testing.T) {
	t.Parallel()
	typ := reflect.TypeFor[models.Dataset]()
	for i := range typ.NumField() {
		name := typ.Field(i).Name
		require.Truef(t, liveFields[name] != repoFields[name],
			"Dataset.%s must be classified as exactly one of liveFields / repoFields", name)
	}

	merged := populatedDataset(t)
	mergeReloadedRepoData(merged, &models.Dataset{})
	reset := populatedDataset(t)
	resetRealmScopedFields(reset)
	for name := range liveFields {
		require.Falsef(t, reflect.ValueOf(merged).Elem().FieldByName(name).IsNil(),
			"mergeReloadedRepoData must preserve %s", name)
		require.Truef(t, reflect.ValueOf(reset).Elem().FieldByName(name).IsNil(),
			"resetRealmScopedFields must drop %s", name)
	}
}

func TestResetRealmScopedFields(t *testing.T) {
	t.Parallel()
	d := populatedDataset(t)
	resetRealmScopedFields(d)
	v := reflect.ValueOf(d).Elem()
	for i := range v.NumField() {
		name := v.Type().Field(i).Name
		realmScoped := map[string]bool{
			"LimitTenancyOverrideMap": true, "ConsolePropertyTenancyOverrideMap": true,
			"PropertyTenancyOverrideMap": true, "Tenants": true, "LimitRegionalOverrides": true,
			"ConsolePropertyRegionalOverrides": true, "PropertyRegionalOverrides": true,
		}
		if realmScoped[name] || liveFields[name] {
			assert.Truef(t, v.Field(i).IsNil(), "resetRealmScopedFields must drop %s", name)
		}
	}
}

// A switch within the realm keeps its tenants and overrides and drops the
// rest.
func TestResetEnvironmentScopedFields(t *testing.T) {
	t.Parallel()
	d := &models.Dataset{
		Tenants:                 []models.Tenant{{Name: "t"}},
		LimitTenancyOverrideMap: map[string][]models.LimitTenancyOverride{"x": nil},
		LimitRegionalOverrides:  []models.LimitRegionalOverride{{}},
		GPUPools:                []models.GPUPool{{}},
		DedicatedAIClusterMap:   map[string][]models.DedicatedAICluster{"x": nil},
	}
	resetEnvironmentScopedFields(d)
	assert.NotNil(t, d.Tenants)
	assert.NotNil(t, d.LimitTenancyOverrideMap)
	assert.NotNil(t, d.LimitRegionalOverrides)
	assert.Nil(t, d.GPUPools)
	assert.Nil(t, d.DedicatedAIClusterMap)
}
