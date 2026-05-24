package columns

import (
	"testing"

	"github.com/jingle2008/toolkit/pkg/models"
)

func TestGpuPoolColumns(t *testing.T) {
	t.Parallel()
	p := models.GpuPool{
		Name:               "pool-1",
		Shape:              "BM.GPU.A10.4",
		AvailabilityDomain: "AD-1",
		Size:               4,
		ActualSize:         3,
		IsOkeManaged:       true,
		CapacityType:       "ON_DEMAND",
		Status:             "Active",
	}

	got := map[string]string{}
	for _, c := range GpuPoolColumns.Columns {
		got[c.Key] = c.Render(p)
	}

	// GetGPUs splits "BM.GPU.A10.4" → last part "4" → count=4, size=4 → 16
	want := map[string]string{
		"name":          "pool-1",
		"shape":         "BM.GPU.A10.4",
		"ad":            "AD-1",
		"size":          "4",
		"actual-size":   "3",
		"gpus":          "16",
		"oke-managed":   "true",
		"capacity-type": "ON_DEMAND",
		"status":        "Active",
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("col %s: got %q, want %q", k, got[k], v)
		}
	}

	// Verify Default flags.
	defaults := map[string]bool{}
	for _, c := range GpuPoolColumns.Columns {
		defaults[c.Key] = c.Default
	}
	for _, k := range []string{"name", "shape", "ad", "size", "actual-size", "gpus", "oke-managed", "capacity-type", "status"} {
		if !defaults[k] {
			t.Errorf("col %s: expected Default=true", k)
		}
	}

	// Verify ratio sum is ~1.0.
	sum := GpuPoolColumns.RatioSum()
	if sum < 0.98 || sum > 1.02 {
		t.Errorf("ratio sum %.3f outside ±0.02 of 1.0", sum)
	}
}
