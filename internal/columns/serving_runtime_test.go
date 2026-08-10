package columns

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/jingle2008/toolkit/pkg/models"
)

func TestAcceleratorFamilies(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		in   []string
		want []string
	}{
		"collapses tp variants": {
			[]string{"nvidia-a100-80gb-1", "nvidia-a100-80gb-4", "nvidia-a100-80gb-8"},
			[]string{"a100-80gb"},
		},
		"preserves first-seen order across families": {
			[]string{"nvidia-h200-8", "nvidia-a10-1", "nvidia-h200-1", "nvidia-a10-4"},
			[]string{"h200", "a10"},
		},
		// An unfamiliar naming scheme should look odd in the cell, not
		// silently disappear from it.
		"unmatched values pass through whole": {
			[]string{"amd-mi300x", "nvidia-h100-8"},
			[]string{"amd-mi300x", "h100"},
		},
		"empty": {nil, []string{}},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, acceleratorFamilies(tc.in))
		})
	}
}

func TestServingRuntimeHardware(t *testing.T) {
	t.Parallel()

	// Accelerator classes win when both are present, and the count is
	// of raw classes, not collapsed families.
	both := models.ServingRuntime{
		AcceleratorClasses: []string{"nvidia-h100-1", "nvidia-h100-8", "nvidia-a10-2"},
		InstanceTypes:      []string{"BM.GPU.H100.8"},
	}
	assert.Equal(t, "ac: h100, a10 (3)", servingRuntimeHardware(both))

	// Instance types are the fallback, and carry their own marker so the
	// two constraint kinds stay distinguishable.
	assert.Equal(t, "it: BM.GPU.H100.8, BM.GPU.A100-v2.8",
		servingRuntimeHardware(models.ServingRuntime{
			InstanceTypes: []string{"BM.GPU.H100.8", "BM.GPU.A100-v2.8"},
		}))

	assert.Empty(t, servingRuntimeHardware(models.ServingRuntime{}))
}

func TestServingRuntimeImage(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"us-chicago-1.ocir.io/idlsnvn0f2is/official-sgl:v0.4.10.post2.6e6f9c7-cu126": "official-sgl:v0.4.10.post2.6e6f9c7-cu126",
		"official-sgl:v1": "official-sgl:v1",
		"":                "",
	}
	for in, want := range cases {
		assert.Equal(t, want, servingRuntimeImage(models.ServingRuntime{Image: in}), "input %q", in)
	}
}

func TestServingRuntimeSizeRange(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "1.8B–2.8B", servingRuntimeSizeRange(models.ServingRuntime{ModelSizeMin: "1.8B", ModelSizeMax: "2.8B"}))
	assert.Equal(t, "1.8B–", servingRuntimeSizeRange(models.ServingRuntime{ModelSizeMin: "1.8B"}))
	assert.Equal(t, "–2.8B", servingRuntimeSizeRange(models.ServingRuntime{ModelSizeMax: "2.8B"}))
	assert.Empty(t, servingRuntimeSizeRange(models.ServingRuntime{}))
}

func TestServingRuntimeCPUMem(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "10 / 30Gi", servingRuntimeCPUMem(models.ServingRuntime{CPULimit: "10", MemoryLimit: "30Gi"}))
	assert.Empty(t, servingRuntimeCPUMem(models.ServingRuntime{}))
}

// The Set contract: ratios must sum to 1.00 and keys must be unique.
func TestServingRuntimeColumns_SetContract(t *testing.T) {
	t.Parallel()
	assert.InDelta(t, 1.0, ServingRuntimeColumns.RatioSum(), 0.0001)

	seen := map[string]struct{}{}
	for _, k := range ServingRuntimeColumns.Keys() {
		_, dup := seen[k]
		assert.Falsef(t, dup, "duplicate column key %q", k)
		seen[k] = struct{}{}
	}
}

// Filtering must reach the raw values the Hardware cell collapses away,
// or a search for a tensor-parallel variant would miss the row showing it.
func TestServingRuntime_FilterableFieldsIncludeRawValues(t *testing.T) {
	t.Parallel()
	r := models.ServingRuntime{
		Name:               "srt-gemma",
		AcceleratorClasses: []string{"nvidia-h100-8"},
		InstanceTypes:      []string{"BM.GPU.H100.8"},
		Image:              "us-chicago-1.ocir.io/org/official-sgl:v1",
	}
	assert.Contains(t, r.FilterableFields(), "nvidia-h100-8")
	assert.Contains(t, r.FilterableFields(), "BM.GPU.H100.8")
	assert.Contains(t, r.FilterableFields(), "us-chicago-1.ocir.io/org/official-sgl:v1")
}
