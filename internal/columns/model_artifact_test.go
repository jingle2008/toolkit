package columns

import (
	"testing"

	"github.com/jingle2008/toolkit/pkg/models"
)

func TestModelArtifactColumns(t *testing.T) {
	t.Parallel()
	a := models.ModelArtifact{
		Name:            "artifact-v1",
		ModelName:       "llama3",
		GpuCount:        8,
		GpuShape:        "BM.GPU.H100.8",
		TensorRTVersion: "8.6.1",
	}

	// Use a key distinct from a.ModelName so the test can distinguish
	// the Render closure using `k` vs `a.ModelName`. By loader invariant
	// they are equal at runtime, but the canonical spec specifies the
	// item field; this assertion catches a future regression that
	// silently swaps to `k`.
	got := map[string]string{}
	for _, c := range ModelArtifactColumns.Columns {
		got[c.Key] = c.Render("key-differs-from-modelname", a)
	}

	want := map[string]string{
		"name":                "artifact-v1",
		"model-internal-name": "llama3",
		"gpu-config":          a.GetGpuConfig(),
		"tensorrt":            "8.6.1",
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("col %s: got %q, want %q", k, got[k], v)
		}
	}

	// Verify Default flags — all four are Default=true.
	defaults := map[string]bool{}
	for _, c := range ModelArtifactColumns.Columns {
		defaults[c.Key] = c.Default
	}
	defaultTrue := []string{"name", "model-internal-name", "gpu-config", "tensorrt"}
	for _, k := range defaultTrue {
		if !defaults[k] {
			t.Errorf("col %s: expected Default=true", k)
		}
	}

	// Verify ratio sum is ~1.0.
	sum := ModelArtifactColumns.RatioSum()
	if sum < 0.98 || sum > 1.02 {
		t.Errorf("ratio sum %.3f outside ±0.02 of 1.0", sum)
	}
}
