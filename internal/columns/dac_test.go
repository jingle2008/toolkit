package columns

import (
	"testing"

	"github.com/jingle2008/toolkit/pkg/models"
)

func TestDacColumns(t *testing.T) {
	t.Parallel()
	d := models.DedicatedAICluster{
		Name:          "dac-1",
		Type:          "LARGE",
		ModelName:     "llama3",
		UnitShape:     "BM.GPU.H100.8",
		Profile:       "",
		Size:          4,
		Age:           "2d",
		Status:        "ACTIVE",
		TotalReplicas: 10,
		IdleReplicas:  3,
	}

	got := map[string]string{}
	for _, c := range DacColumns.Columns {
		got[c.Key] = c.Render("tenant-abc", d)
	}

	want := map[string]string{
		"name":          "dac-1",
		"tenant":        "tenant-abc",
		"internal":      d.GetOwnerState(),
		"usage":         d.GetUsage(),
		"type":          "LARGE",
		"model":         "llama3",
		"shape-profile": "BM.GPU.H100.8",
		"size":          "4",
		"age":           "2d",
		"status":        "ACTIVE",
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("col %s: got %q, want %q", k, got[k], v)
		}
	}

	// dacUnitShapeOrProfile falls back to Profile when UnitShape is empty.
	dProfile := models.DedicatedAICluster{Profile: "standard"}
	if dacUnitShapeOrProfile(dProfile) != "standard" {
		t.Errorf("dacUnitShapeOrProfile: expected %q, got %q", "standard", dacUnitShapeOrProfile(dProfile))
	}

	// Verify ratio sum is ~1.0.
	sum := DacColumns.RatioSum()
	if sum < 0.98 || sum > 1.02 {
		t.Errorf("ratio sum %.3f outside ±0.02 of 1.0", sum)
	}
}
