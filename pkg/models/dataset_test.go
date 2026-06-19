package models

import (
	"reflect"
	"testing"
)

func TestBuildTenantIDSuffixMap(t *testing.T) {
	t.Parallel()
	d := &Dataset{
		Tenants: []Tenant{
			{Name: "TenantA", IDs: []string{"ocid1.tenancy.oc1..aaaa", "ocid1.tenancy.oc1..aaab"}},
			{Name: "TenantB", IDs: []string{"ocid1.tenancy.oc1..bbbb"}},
		},
	}
	got := d.buildTenantIDSuffixMap()
	want := map[string]int{
		"aaaa": 0,
		"aaab": 0,
		"bbbb": 1,
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("buildTenantIDSuffixMap() = %v, want %v", got, want)
	}
}

func TestSetDedicatedAIClusterMap(t *testing.T) {
	t.Parallel()
	tenantA := Tenant{Name: "TenantA", IDs: []string{"ocid1.tenancy.oc1..aaaa"}}
	tenantB := Tenant{Name: "TenantB", IDs: []string{"ocid1.tenancy.oc1..bbbb"}}
	d := &Dataset{
		Tenants: []Tenant{tenantA, tenantB},
	}
	// Key matches suffix for TenantA, and a key that doesn't match any tenant
	input := map[string][]DedicatedAICluster{
		"aaaa": {
			{Name: "dac1"},
		},
		"other": {
			{Name: "dac2"},
		},
	}
	d.SetDedicatedAIClusterMap(input)
	// Should rewrite "aaaa" to "TenantA", leave "other" as is
	if _, ok := d.DedicatedAIClusterMap["TenantA"]; !ok {
		t.Errorf("expected key 'TenantA' in DedicatedAIClusterMap")
	}
	if _, ok := d.DedicatedAIClusterMap["other"]; !ok {
		t.Errorf("expected key 'other' in DedicatedAIClusterMap")
	}
	// Owner pointer should be set for "TenantA"
	for _, dac := range d.DedicatedAIClusterMap["TenantA"] {
		if dac.Owner == nil || dac.Owner.Name != "TenantA" {
			t.Errorf("Owner not set correctly for TenantA: got %+v", dac.Owner)
		}
	}
	// Owner pointer should be nil for "other"
	for _, dac := range d.DedicatedAIClusterMap["other"] {
		if dac.Owner != nil {
			t.Errorf("Owner should be nil for 'other', got %+v", dac.Owner)
		}
	}
}

// TestSetImportedModelMap mirrors TestSetDedicatedAIClusterMap. Verifies
// the matched-by-suffix path re-keys by Tenant.Name and sets Owner;
// the unmatched key is preserved and Owner stays nil.
func TestSetImportedModelMap(t *testing.T) {
	t.Parallel()
	tenantA := Tenant{Name: "TenantA", IDs: []string{"ocid1.tenancy.oc1..aaaa"}}
	tenantB := Tenant{Name: "TenantB", IDs: []string{"ocid1.tenancy.oc1..bbbb"}}
	d := &Dataset{
		Tenants: []Tenant{tenantA, tenantB},
	}
	input := map[string][]ImportedModel{
		"aaaa": {
			{BaseModel: BaseModel{Name: "im1"}, Namespace: "team-a", TenantID: "aaaa"},
		},
		"other": {
			{BaseModel: BaseModel{Name: "im2"}, TenantID: "other"},
		},
	}
	d.SetImportedModelMap(input)

	// Matched key "aaaa" rewrites to Tenant.Name; Owner pointer set.
	if _, ok := d.ImportedModelMap["TenantA"]; !ok {
		t.Errorf("expected key 'TenantA' in ImportedModelMap, got keys: %v", keys(d.ImportedModelMap))
	}
	for _, im := range d.ImportedModelMap["TenantA"] {
		if im.Owner == nil || im.Owner.Name != "TenantA" {
			t.Errorf("Owner not set correctly for TenantA: got %+v", im.Owner)
		}
	}

	// Unmatched key passes through with Owner nil.
	if _, ok := d.ImportedModelMap["other"]; !ok {
		t.Errorf("expected key 'other' in ImportedModelMap")
	}
	for _, im := range d.ImportedModelMap["other"] {
		if im.Owner != nil {
			t.Errorf("Owner should be nil for 'other', got %+v", im.Owner)
		}
	}
}

func TestSetGPUWorkloadMap_ResolvesOwnerKeepsNodeKey(t *testing.T) {
	t.Parallel()
	d := &Dataset{
		Tenants: []Tenant{{Name: "acme", IDs: []string{"ocid1.tenancy.oc1..suffix1"}}},
	}
	d.SetGPUWorkloadMap(map[string][]GPUWorkload{
		"node-a": {{Name: "p1", Node: "node-a", TenantID: "suffix1"}},
		"node-b": {{Name: "p2", Node: "node-b", TenantID: "unknown"}},
	})
	// Keyed by node, not re-keyed by tenant.
	if _, ok := d.GPUWorkloadMap["node-a"]; !ok {
		t.Fatalf("expected key node-a; got %v", d.GPUWorkloadMap)
	}
	// Owner resolved for matching suffix.
	if d.GPUWorkloadMap["node-a"][0].Owner == nil || d.GPUWorkloadMap["node-a"][0].Owner.Name != "acme" {
		t.Errorf("owner not resolved: %+v", d.GPUWorkloadMap["node-a"][0].Owner)
	}
	// Unmatched suffix → nil owner, key preserved.
	if d.GPUWorkloadMap["node-b"][0].Owner != nil {
		t.Errorf("expected nil owner for unknown suffix")
	}
}

func keys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

/*
TestResetRealmScopedFields checks that ResetRealmScopedFields nils all relevant fields.
*/
//nolint:cyclop // test is clear and further splitting would reduce readability
func TestResetRealmScopedFields(t *testing.T) {
	t.Parallel()
	d := &Dataset{
		LimitTenancyOverrideMap:           map[string][]LimitTenancyOverride{"x": nil},
		ConsolePropertyTenancyOverrideMap: map[string][]ConsolePropertyTenancyOverride{"x": nil},
		PropertyTenancyOverrideMap:        map[string][]PropertyTenancyOverride{"x": nil},
		Tenants:                           []Tenant{{Name: "t"}},
		LimitRegionalOverrides:            []LimitRegionalOverride{{}},
		ConsolePropertyRegionalOverrides:  []ConsolePropertyRegionalOverride{{}},
		PropertyRegionalOverrides:         []PropertyRegionalOverride{{}},
		BaseModels:                        []BaseModel{},
		ImportedModelMap:                  map[string][]ImportedModel{"x": nil},
		GPUPools:                          []GPUPool{{}},
		GPUNodeMap:                        map[string][]GPUNode{"x": nil},
		GPUWorkloadMap:                    map[string][]GPUWorkload{"x": nil},
		DedicatedAIClusterMap:             map[string][]DedicatedAICluster{"x": nil},
	}
	d.ResetRealmScopedFields()
	if d.LimitTenancyOverrideMap != nil ||
		d.ConsolePropertyTenancyOverrideMap != nil ||
		d.PropertyTenancyOverrideMap != nil ||
		d.Tenants != nil ||
		d.LimitRegionalOverrides != nil ||
		d.ConsolePropertyRegionalOverrides != nil ||
		d.PropertyRegionalOverrides != nil ||
		d.BaseModels != nil ||
		d.ImportedModelMap != nil ||
		d.GPUPools != nil ||
		d.GPUNodeMap != nil ||
		d.GPUWorkloadMap != nil ||
		d.DedicatedAIClusterMap != nil {
		t.Errorf("ResetRealmScopedFields did not nil all fields")
	}
}
