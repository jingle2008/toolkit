package columns

import (
	"testing"

	"github.com/jingle2008/toolkit/pkg/models"
)

func TestGPUWorkloadColumns(t *testing.T) {
	t.Parallel()
	// Node MUST be at index 1 (grouped key invariant).
	if GPUWorkloadColumns.Columns[1].Title != "Node" {
		t.Fatalf("col[1] = %q, want Node", GPUWorkloadColumns.Columns[1].Title)
	}
	w := models.GPUWorkload{
		Name: "p1", Node: "node-a", TenantID: "suffix1", Namespace: "ns1",
		Model: "gpt", Runtime: "vllm", GPUs: 2, Mode: "RawDeployment",
		Owner: &models.Tenant{Name: "acme"},
	}
	got := map[string]string{}
	for _, c := range GPUWorkloadColumns.Columns {
		got[c.Key] = c.Render("node-a", w)
	}
	want := map[string]string{
		"name": "p1", "node": "node-a", "tenant": "acme", "namespace": "ns1",
		"model": "gpt", "runtime": "vllm", "gpus": "2", "mode": "RawDeployment",
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("col %s = %q, want %q", k, got[k], v)
		}
	}
	if s := GPUWorkloadColumns.RatioSum(); s < 0.98 || s > 1.02 {
		t.Errorf("ratio sum %.3f", s)
	}
}
