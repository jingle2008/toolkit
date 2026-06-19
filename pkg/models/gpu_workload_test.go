package models

import "testing"

func TestGPUWorkload(t *testing.T) {
	t.Parallel()
	w := GPUWorkload{
		Name: "pod-1", Node: "10.0.0.1", TenantID: "suffix1",
		Namespace: "ns1", Model: "gpt", Runtime: "vllm", GPUs: 2, Mode: "RawDeployment",
	}
	if w.GetName() != "pod-1" {
		t.Errorf("GetName = %q", w.GetName())
	}
	if w.IsFaulty() {
		t.Error("IsFaulty should be false")
	}
	if got := w.TenancyOCID("oc1"); got != "ocid1.tenancy.oc1..suffix1" {
		t.Errorf("TenancyOCID = %q", got)
	}
	// Unresolved owner → raw suffix.
	if got := w.TenantName(); got != "suffix1" {
		t.Errorf("TenantName (unresolved) = %q", got)
	}
	// Resolved owner → tenant name.
	w.Owner = &Tenant{Name: "acme"}
	if got := w.TenantName(); got != "acme" {
		t.Errorf("TenantName (resolved) = %q", got)
	}
	// FilterableFields includes identity fields.
	fields := w.FilterableFields()
	want := []string{"pod-1", "10.0.0.1", "suffix1", "ns1", "gpt", "vllm", "RawDeployment"}
	for _, exp := range want {
		found := false
		for _, f := range fields {
			if f == exp {
				found = true
			}
		}
		if !found {
			t.Errorf("FilterableFields missing %q; got %v", exp, fields)
		}
	}
}
