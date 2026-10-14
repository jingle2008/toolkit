package models

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDataset_MergeReloadedRepoData_PreservesK8sFields(t *testing.T) {
	t.Parallel()
	d := &Dataset{
		Tenants:    []Tenant{{Name: "old"}},
		BaseModels: []BaseModel{{Name: "bm1"}},
		GPUPools:   []GPUPool{{Name: "p1"}},
		GPUNodeMap: map[string][]GPUNode{"p1": {{Name: "n1"}}},
	}
	bm := d.BaseModels
	pools := d.GPUPools
	nodes := d.GPUNodeMap

	fresh := &Dataset{
		Tenants:      []Tenant{{Name: "new1"}, {Name: "new2"}},
		Environments: []Environment{{Type: "dev"}},
		// k8s-backed fields left nil, as LoadDataset returns them
	}

	d.MergeReloadedRepoData(fresh)

	// Repo-owned fields are replaced by the freshly loaded values.
	require.Len(t, d.Tenants, 2)
	require.Equal(t, "new1", d.Tenants[0].Name)
	require.Len(t, d.Environments, 1)

	// Lazily-loaded k8s fields are preserved untouched.
	require.Equal(t, bm, d.BaseModels)
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

// populated returns a Dataset whose every slice and map field is non-nil.
func populated(t *testing.T) *Dataset {
	t.Helper()
	d := &Dataset{}
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

// A field forgotten in MergeReloadedRepoData is wiped by every repo file
// change; one forgotten in ResetRealmScopedFields shows the previous
// cluster's rows after an environment switch (the category handlers only
// load when the field is nil). Both happened to ServingRuntimes.
func TestDataset_LiveFieldsDriftGuard(t *testing.T) {
	t.Parallel()
	typ := reflect.TypeFor[Dataset]()
	for i := range typ.NumField() {
		name := typ.Field(i).Name
		require.Truef(t, liveFields[name] != repoFields[name],
			"Dataset.%s must be classified as exactly one of liveFields / repoFields", name)
	}

	merged := populated(t)
	merged.MergeReloadedRepoData(&Dataset{})
	reset := populated(t)
	reset.ResetRealmScopedFields()
	for name := range liveFields {
		require.Falsef(t, reflect.ValueOf(merged).Elem().FieldByName(name).IsNil(),
			"MergeReloadedRepoData must preserve %s", name)
		require.Truef(t, reflect.ValueOf(reset).Elem().FieldByName(name).IsNil(),
			"ResetRealmScopedFields must drop %s", name)
	}
}
