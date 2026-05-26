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
		GPUCount:        8,
		GPUShape:        "BM.GPU.H100.8",
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
		"gpu-config":          a.GetGPUConfig(),
		"tensorrt":            "8.6.1",
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("col %s: got %q, want %q", k, got[k], v)
		}
	}

	// Verify ratio sum is ~1.0.
	sum := ModelArtifactColumns.RatioSum()
	if sum < 0.98 || sum > 1.02 {
		t.Errorf("ratio sum %.3f outside ±0.02 of 1.0", sum)
	}
}

// TestModelArtifactColumns_ModelNameAtIndex1 pins a cross-package
// invariant: the TUI's getItemKey treats row[1] of a ModelArtifact
// row as the parent BaseModel scope (it equals the
// ModelArtifactMap key by loader design). If "Model Internal Name"
// ever moves to a different column position, getItemKey would mint
// the wrong ScopedItemKey and findItem(ModelArtifact, ...) would
// return nil for live selections. Fail fast at the column-registry
// layer so the regression is caught here, not as a silent UI bug.
func TestModelArtifactColumns_ModelNameAtIndex1(t *testing.T) {
	t.Parallel()
	if got := ModelArtifactColumns.Columns[1].Title; got != "Model Internal Name" {
		t.Fatalf("Columns[1].Title = %q, want %q — getItemKey(ModelArtifact) depends on this position; see internal/ui/tui/table_utils.go", got, "Model Internal Name")
	}
}
