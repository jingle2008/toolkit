package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestServingRuntimeStatus(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "live", ServingRuntime{AutoSelect: true}.Status())
	assert.Equal(t, "off", ServingRuntime{}.Status())
	// Disabled wins over either auto-select state.
	assert.Equal(t, "disabled", ServingRuntime{Disabled: true, AutoSelect: true}.Status())
	assert.Equal(t, "disabled", ServingRuntime{Disabled: true}.Status())
}

// Every status word is searchable, the normal case included.
func TestServingRuntime_FilterableFieldsIncludeStatus(t *testing.T) {
	t.Parallel()
	assert.Contains(t, ServingRuntime{AutoSelect: true}.FilterableFields(), "live")
	assert.Contains(t, ServingRuntime{Disabled: true}.FilterableFields(), "disabled")
}

func TestServingRuntimeGPUFamilies(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		in   ServingRuntime
		want []string
	}{
		"collapses tp variants": {
			ServingRuntime{AcceleratorClasses: []string{"nvidia-a100-80gb-1", "nvidia-a100-80gb-4", "nvidia-a100-80gb-8"}},
			[]string{"a100-80gb"},
		},
		"preserves first-seen order across families": {
			ServingRuntime{AcceleratorClasses: []string{"nvidia-h200-8", "nvidia-a10-1", "nvidia-h200-1", "nvidia-a10-4"}},
			[]string{"h200", "a10"},
		},
		// An unfamiliar naming scheme should look odd in the cell, not
		// silently disappear from it.
		"unmatched classes pass through whole": {
			ServingRuntime{AcceleratorClasses: []string{"amd-mi300x", "nvidia-h100-8"}},
			[]string{"amd-mi300x", "h100"},
		},
		"accelerator classes win over instance types": {
			ServingRuntime{AcceleratorClasses: []string{"nvidia-h100-8"}, InstanceTypes: []string{"BM.GPU.H200-NC.8"}},
			[]string{"h100"},
		},
		// Shapes that share a GPU collapse into one entry.
		"instance types map to families": {
			ServingRuntime{InstanceTypes: []string{"BM.GPU.B4.8", "BM.GPU4.8", "BM.GPU.H100.8", "BM.GPU.A100-v2.8"}},
			[]string{"a100-40gb", "h100", "a100-80gb"},
		},
		"unknown shapes pass through whole": {
			ServingRuntime{InstanceTypes: []string{"BM.GPU.H200-NC.8", "VM.GPU.X.1"}},
			[]string{"h200", "VM.GPU.X.1"},
		},
		"empty": {ServingRuntime{}, []string{}},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, tc.in.GPUFamilies())
		})
	}
}

// A family mapped from an instance type is searchable by the name the
// Hardware cell shows, not only by the raw shape.
func TestServingRuntime_FilterableFieldsIncludeGPUFamilies(t *testing.T) {
	t.Parallel()
	assert.Contains(t, ServingRuntime{InstanceTypes: []string{"BM.GPU4.8"}}.FilterableFields(), "a100-40gb")
}
