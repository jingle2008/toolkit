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

func TestResetScopedData(t *testing.T) {
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
		GpuPools:                          []GpuPool{{}},
		GpuNodeMap:                        map[string][]GpuNode{"x": nil},
		DedicatedAIClusterMap:             map[string][]DedicatedAICluster{"x": nil},
	}
	d.ResetScopedData()
	if d.LimitTenancyOverrideMap != nil ||
		d.ConsolePropertyTenancyOverrideMap != nil ||
		d.PropertyTenancyOverrideMap != nil ||
		d.Tenants != nil ||
		d.LimitRegionalOverrides != nil ||
		d.ConsolePropertyRegionalOverrides != nil ||
		d.PropertyRegionalOverrides != nil ||
		d.BaseModels != nil ||
		d.GpuPools != nil ||
		d.GpuNodeMap != nil ||
		d.DedicatedAIClusterMap != nil {
		t.Errorf("ResetScopedData did not nil all fields")
	}
}
