package columns

import (
	"testing"

	"github.com/jingle2008/toolkit/pkg/models"
)

func TestGpuNodeColumns(t *testing.T) {
	t.Parallel()
	n := models.GpuNode{
		Name:         "node-1",
		NodePool:     "pool-A",
		InstanceType: "BM.GPU4.8",
		Allocatable:  8,
		Allocated:    3,
		IsReady:      true,
		Age:          "1d",
	}

	got := map[string]string{}
	for _, c := range GpuNodeColumns.Columns {
		got[c.Key] = c.Render("pool-A", n)
	}

	want := map[string]string{
		"name":    "node-1",
		"pool":    "pool-A",
		"type":    "BM.GPU4.8",
		"total":   "8",
		"free":    "5",
		"healthy": "true",
		"ready":   "true",
		"age":     "1d",
		"status":  n.GetStatus(),
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("col %s: got %q, want %q", k, got[k], v)
		}
	}

	// Verify ratio sum is ~1.0.
	sum := GpuNodeColumns.RatioSum()
	if sum < 0.98 || sum > 1.02 {
		t.Errorf("ratio sum %.3f outside ±0.02 of 1.0", sum)
	}
}
