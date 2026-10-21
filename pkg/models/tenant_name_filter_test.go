package models

import (
	"slices"
	"testing"
)

// The Tenant column of GPU workloads and inference services shows the
// resolved tenant name, but their maps are keyed by node and namespace, so
// the group key can't carry it: FilterableFields must, or the column isn't
// searchable by what it shows.
func TestFilterableFields_IncludeResolvedTenantName(t *testing.T) {
	t.Parallel()
	owner := &Tenant{Name: "acme"}
	cases := map[string][]string{
		"GPUWorkload":      GPUWorkload{Name: "pod-1", TenantID: "suffix1", Owner: owner}.FilterableFields(),
		"InferenceService": InferenceService{Name: "isvc-1", TenantID: "suffix1", Owner: owner}.FilterableFields(),
	}
	for name, fields := range cases {
		for _, want := range []string{"acme", "suffix1"} {
			if !slices.Contains(fields, want) {
				t.Errorf("%s FilterableFields missing %q; got %v", name, want, fields)
			}
		}
	}
}
