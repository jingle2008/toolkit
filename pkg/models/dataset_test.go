package models

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBuildTenantIDSuffixMap(t *testing.T) {
	t.Parallel()
	d := &Dataset{
		Tenants: []Tenant{
			{Name: "TenantA", ID: "ocid1.tenancy.oc1..aaaa"},
			{Name: "TenantB", ID: "ocid1.tenancy.oc1..bbbb"},
		},
	}
	got := d.buildTenantIDSuffixMap()
	want := map[string]int{
		"aaaa": 0,
		"bbbb": 1,
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("buildTenantIDSuffixMap() = %v, want %v", got, want)
	}
}

func TestSetDedicatedAIClusterMap(t *testing.T) {
	t.Parallel()
	tenantA := Tenant{Name: "TenantA", ID: "ocid1.tenancy.oc1..aaaa"}
	tenantB := Tenant{Name: "TenantB", ID: "ocid1.tenancy.oc1..bbbb"}
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
	tenantA := Tenant{Name: "TenantA", ID: "ocid1.tenancy.oc1..aaaa"}
	tenantB := Tenant{Name: "TenantB", ID: "ocid1.tenancy.oc1..bbbb"}
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
		Tenants: []Tenant{{Name: "acme", ID: "ocid1.tenancy.oc1..suffix1"}},
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

// A pod without a tenancy-id label in a DAC's namespace takes the DAC's
// tenant, whichever of the two loads first; a labelled pod keeps its
// own, and a pod outside any DAC stays unowned.
func TestSetGPUWorkloadMap_TenantFromDAC(t *testing.T) {
	t.Parallel()
	pods := func() map[string][]GPUWorkload {
		return map[string][]GPUWorkload{"node-a": {
			{Name: "reservation", Namespace: "dac-1"},
			{Name: "labelled", Namespace: "dac-1", TenantID: "suffix2"},
			{Name: "shared", Namespace: "llama"},
		}}
	}
	dacs := map[string][]DedicatedAICluster{"suffix1": {{Name: "dac-1", TenantID: "suffix1"}}}
	tenants := []Tenant{{Name: "acme", ID: "ocid1.tenancy.oc1..suffix1"}, {Name: "beta", ID: "ocid1.tenancy.oc1..suffix2"}}

	check := func(d *Dataset) {
		t.Helper()
		got := map[string]string{}
		for _, w := range d.GPUWorkloadMap["node-a"] {
			got[w.Name] = w.TenantName()
		}
		assert.Equal(t, map[string]string{"reservation": "acme", "labelled": "beta", "shared": ""}, got)
	}

	dacsFirst := &Dataset{Tenants: tenants}
	dacsFirst.SetDedicatedAIClusterMap(dacs)
	dacsFirst.SetGPUWorkloadMap(pods())
	check(dacsFirst)

	podsFirst := &Dataset{Tenants: tenants}
	podsFirst.SetGPUWorkloadMap(pods())
	podsFirst.SetDedicatedAIClusterMap(dacs)
	check(podsFirst)
}

func TestSetInferenceServiceMap_ResolvesOwnerKeepsNamespaceKey(t *testing.T) {
	t.Parallel()
	d := &Dataset{
		Tenants: []Tenant{{Name: "acme", ID: "ocid1.tenancy.oc1..suffix1"}},
	}
	d.SetInferenceServiceMap(map[string][]InferenceService{
		"amaaaaaadac1": {{Name: "e1", Namespace: "amaaaaaadac1", TenantID: "suffix1"}},
		"embed-v4":     {{Name: "embed-v4", Namespace: "embed-v4"}},
	})
	if got := d.InferenceServiceMap["amaaaaaadac1"]; len(got) != 1 || got[0].Owner == nil || got[0].Owner.Name != "acme" {
		t.Fatalf("owner not resolved under namespace key: %v", d.InferenceServiceMap)
	}
	if got := d.InferenceServiceMap["embed-v4"]; len(got) != 1 || got[0].Owner != nil {
		t.Errorf("on-demand endpoint should have no owner: %+v", got)
	}
}

func keys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func TestFindBaseModelByName(t *testing.T) {
	t.Parallel()
	d := &Dataset{
		BaseModels: []BaseModel{{Name: "base-a", DisplayName: "openai.gpt-5.5"}},
		ImportedModelMap: map[string][]ImportedModel{
			"t": {{BaseModel: BaseModel{Name: "imported-b"}}},
		},
	}
	assert.Equal(t, "openai.gpt-5.5", d.FindBaseModelByName("base-a").DisplayName)
	assert.Nil(t, d.FindBaseModelByName("imported-b"), "imported models are not in the base catalog")
	assert.Nil(t, d.FindBaseModelByName("missing"))
	assert.Nil(t, d.FindBaseModelByName(""))
}
