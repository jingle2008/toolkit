package columns

import (
	"testing"

	"github.com/jingle2008/toolkit/pkg/models"
)

func TestInferenceServiceColumns(t *testing.T) {
	t.Parallel()
	// Namespace MUST be at index 1 (grouped key invariant).
	if InferenceServiceColumns.Columns[1].Title != "Namespace" {
		t.Fatalf("col[1] = %q, want Namespace", InferenceServiceColumns.Columns[1].Title)
	}
	s := models.InferenceService{
		Name: "e1", Namespace: "amaaaaaadac1", Type: models.InferenceServiceDedicated,
		TenantID: "suffix1", Model: "amaaaaaam1", ImportedModel: true, Runtime: "vllm",
		MinReplicas: new(1), MaxReplicas: new(3), Ready: "True", CreatedAt: daysAgo(3),
		Owner: &models.Tenant{Name: "acme"},
	}
	got := map[string]string{}
	export := map[string]string{}
	for _, c := range InferenceServiceColumns.Columns {
		got[c.Key] = c.Render("amaaaaaadac1", s)
		if c.RenderForExport != nil {
			export[c.Key] = c.RenderForExport("oc1", "us-chicago-1", "amaaaaaadac1", s)
		}
	}
	want := map[string]string{
		"name": "e1", "namespace": "amaaaaaadac1", "tenant": "acme",
		// No imported-model marker: the name alone (an OCID) says it.
		"model": "amaaaaaam1", "runtime": "vllm", "replicas": "1/3", "ready": "True", "age": "3d",
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("col %s = %q, want %q", k, got[k], v)
		}
	}
	if export["tenant"] != "ocid1.tenancy.oc1..suffix1" {
		t.Errorf("export tenant = %q", export["tenant"])
	}
	if _, ok := export["model"]; ok {
		t.Error("model needs no export override: display and export are both the bare name")
	}
	if _, ok := got["type"]; ok {
		t.Error("type column should be dropped (inferable from tenant/namespace)")
	}
	if s := InferenceServiceColumns.RatioSum(); s < 0.98 || s > 1.02 {
		t.Errorf("ratio sum %.3f", s)
	}
}

// An on-demand endpoint has no tenant, so the tenant cell and its export
// are blank rather than a dangling "ocid1.tenancy.oc1..".
func TestInferenceServiceColumns_OnDemandTenantBlank(t *testing.T) {
	t.Parallel()
	od := models.InferenceService{Name: "embed-v4", Namespace: "embed-v4", Type: models.InferenceServiceOnDemand, Model: "embed-v4"}
	for _, c := range InferenceServiceColumns.Columns {
		if c.Key != "tenant" {
			continue
		}
		if v := c.Render("embed-v4", od); v != "" {
			t.Errorf("on-demand tenant = %q, want blank", v)
		}
		if v := c.RenderForExport("oc1", "", "embed-v4", od); v != "" {
			t.Errorf("on-demand tenant export = %q, want blank", v)
		}
	}
}
