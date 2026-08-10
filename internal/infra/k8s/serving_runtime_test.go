package k8s

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"

	"github.com/jingle2008/toolkit/pkg/models"
)

// srtSample is the shape of a real ClusterServingRuntime, trimmed to the
// fields the parser reads. It deliberately keeps the duplication the
// real CR has — image, affinity and limits appear both under
// engineConfig and at the top level of spec — so the precedence rule is
// exercised by the happy path rather than only by synthetic variants.
const srtSample = `
apiVersion: ome.io/v1beta1
kind: ClusterServingRuntime
metadata:
  name: srt-gemma-2-2b-it
spec:
  acceleratorRequirements:
    acceleratorClasses:
    - nvidia-a100-80gb-1
    - nvidia-a100-80gb-8
    - nvidia-h100-4
    - nvidia-a10-2
  affinity:
    nodeAffinity:
      requiredDuringSchedulingIgnoredDuringExecution:
        nodeSelectorTerms:
        - matchExpressions:
          - key: node.kubernetes.io/instance-type
            operator: In
            values:
            - BM.GPU.A100-v2.8
            - BM.GPU.H100.8
  containers:
  - name: ome-container
    image: us-chicago-1.ocir.io/idlsnvn0f2is/official-sgl:v0.4.10.post2.6e6f9c7-cu126
    resources:
      limits:
        cpu: "10"
        memory: 30Gi
        nvidia.com/gpu: "1"
  disabled: false
  engineConfig:
    affinity:
      nodeAffinity:
        requiredDuringSchedulingIgnoredDuringExecution:
          nodeSelectorTerms:
          - matchExpressions:
            - key: node.kubernetes.io/instance-type
              operator: In
              values:
              - BM.GPU.A100-v2.8
              - BM.GPU.H100.8
    runner:
      name: ome-container
      image: us-chicago-1.ocir.io/idlsnvn0f2is/official-sgl:v0.4.10.post2.6e6f9c7-cu126
      resources:
        limits:
          cpu: "10"
          memory: 30Gi
          nvidia.com/gpu: "1"
  modelSizeRange:
    max: 2.8B
    min: 1.8B
  supportedModelFormats:
  - autoSelect: true
    modelArchitecture: Gemma2ForCausalLM
`

func parseYAML(t *testing.T, src string) *unstructured.Unstructured {
	t.Helper()
	var obj map[string]any
	require.NoError(t, yaml.Unmarshal([]byte(src), &obj))
	return &unstructured.Unstructured{Object: obj}
}

func TestParseServingRuntime_Sample(t *testing.T) {
	t.Parallel()
	got := parseServingRuntime(context.Background(), parseYAML(t, srtSample))

	assert.Equal(t, models.ServingRuntime{
		Name: "srt-gemma-2-2b-it",
		AcceleratorClasses: []string{
			"nvidia-a100-80gb-1", "nvidia-a100-80gb-8", "nvidia-h100-4", "nvidia-a10-2",
		},
		InstanceTypes: []string{"BM.GPU.A100-v2.8", "BM.GPU.H100.8"},
		Image:         "us-chicago-1.ocir.io/idlsnvn0f2is/official-sgl:v0.4.10.post2.6e6f9c7-cu126",
		ModelSizeMin:  "1.8B",
		ModelSizeMax:  "2.8B",
		CPULimit:      "10",
		MemoryLimit:   "30Gi",
		GPULimit:      "1",
		Disabled:      false,
		AutoSelect:    true,
	}, got)
}

// engineConfig wins when the two sources disagree — the rule the whole
// extraction hangs on, and invisible in the sample where they agree.
func TestParseServingRuntime_EngineConfigWins(t *testing.T) {
	t.Parallel()
	src := `
metadata:
  name: srt
spec:
  containers:
  - name: ome-container
    image: reg/org/legacy:v1
    resources:
      limits: {cpu: "2", memory: 4Gi, nvidia.com/gpu: "8"}
  affinity:
    nodeAffinity:
      requiredDuringSchedulingIgnoredDuringExecution:
        nodeSelectorTerms:
        - matchExpressions:
          - key: node.kubernetes.io/instance-type
            operator: In
            values: [BM.GPU.LEGACY.1]
  engineConfig:
    runner:
      name: ome-container
      image: reg/org/current:v2
      resources:
        limits: {cpu: "10", memory: 30Gi, nvidia.com/gpu: "1"}
    affinity:
      nodeAffinity:
        requiredDuringSchedulingIgnoredDuringExecution:
          nodeSelectorTerms:
          - matchExpressions:
            - key: node.kubernetes.io/instance-type
              operator: In
              values: [BM.GPU.H100.8]
`
	got := parseServingRuntime(context.Background(), parseYAML(t, src))
	assert.Equal(t, "reg/org/current:v2", got.Image)
	assert.Equal(t, "10", got.CPULimit)
	assert.Equal(t, "1", got.GPULimit)
	assert.Equal(t, []string{"BM.GPU.H100.8"}, got.InstanceTypes)
}

// With no engineConfig the top-level container is the only source, and
// the ome-container must be picked out from among sidecars.
func TestParseServingRuntime_TopLevelFallbackSkipsSidecars(t *testing.T) {
	t.Parallel()
	src := `
metadata:
  name: srt
spec:
  containers:
  - name: istio-proxy
    image: reg/org/sidecar:v9
    resources:
      limits: {cpu: "1", memory: 1Gi}
  - name: ome-container
    image: reg/org/server:v2
    resources:
      limits: {cpu: "10", memory: 30Gi, nvidia.com/gpu: "4"}
`
	got := parseServingRuntime(context.Background(), parseYAML(t, src))
	assert.Equal(t, "reg/org/server:v2", got.Image)
	assert.Equal(t, "4", got.GPULimit)
}

/*
Resource quantities arrive as JSON numbers as often as strings — in
this fleet every `cpu` is a number and `nvidia.com/gpu` usually is —
and reading them with NestedString silently produced "" rather than
failing. The original fixture quoted every quantity, so the tests
agreed with each other and disagreed with the cluster. This pins the
unquoted forms.
*/
func TestParseServingRuntime_NumericQuantities(t *testing.T) {
	t.Parallel()
	src := `
metadata:
  name: srt
spec:
  engineConfig:
    runner:
      name: ome-container
      image: reg/org/img:v1
      resources:
        limits:
          cpu: 10
          memory: 80Gi
          nvidia.com/gpu: 1
`
	got := parseServingRuntime(context.Background(), parseYAML(t, src))
	assert.Equal(t, "10", got.CPULimit, "integer cpu must not render as empty or 10.000000")
	assert.Equal(t, "80Gi", got.MemoryLimit)
	assert.Equal(t, "1", got.GPULimit)
}

// The shape most of this fleet actually uses: only the GPU is capped,
// cpu and memory appear as requests. Both sets must be captured so the
// table can fall back without mislabelling a request as a limit.
func TestParseServingRuntime_RequestsCapturedSeparately(t *testing.T) {
	t.Parallel()
	src := `
metadata:
  name: srt
spec:
  engineConfig:
    runner:
      resources:
        limits:
          nvidia.com/gpu: "2"
        requests:
          cpu: 16
          memory: 120Gi
          nvidia.com/gpu: "2"
`
	got := parseServingRuntime(context.Background(), parseYAML(t, src))
	assert.Equal(t, "2", got.GPULimit)
	assert.Empty(t, got.CPULimit, "a request must not be reported as a limit")
	assert.Empty(t, got.MemoryLimit)
	assert.Equal(t, "16", got.CPURequest)
	assert.Equal(t, "120Gi", got.MemoryRequest)
	assert.Equal(t, "2", got.GPURequest)
}

func TestParseServingRuntime_FractionalCPU(t *testing.T) {
	t.Parallel()
	src := `
metadata:
  name: srt
spec:
  engineConfig:
    runner:
      resources:
        limits: {cpu: 0.5}
`
	got := parseServingRuntime(context.Background(), parseYAML(t, src))
	assert.Equal(t, "0.5", got.CPULimit)
}

func TestParseServingRuntime_OptionalFieldsAbsent(t *testing.T) {
	t.Parallel()
	got := parseServingRuntime(context.Background(), parseYAML(t, "metadata:\n  name: bare\nspec: {}\n"))

	assert.Equal(t, "bare", got.Name)
	assert.Empty(t, got.AcceleratorClasses)
	assert.Empty(t, got.InstanceTypes)
	assert.Empty(t, got.Image)
	assert.Empty(t, got.ModelSizeMin)
	assert.Empty(t, got.GPULimit)
	assert.False(t, got.Disabled)
	assert.False(t, got.AutoSelect)
}

func TestParseServingRuntime_Disabled(t *testing.T) {
	t.Parallel()
	got := parseServingRuntime(context.Background(), parseYAML(t, "metadata:\n  name: off\nspec:\n  disabled: true\n"))
	assert.True(t, got.Disabled)
	assert.True(t, got.IsFaulty(), "a disabled runtime must read as faulty")
}

func TestParseServingRuntime_AutoSelectIsAnyFormat(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		formats string
		want    bool
	}{
		"none set":   {"  - autoSelect: false\n  - modelArchitecture: X\n", false},
		"second set": {"  - autoSelect: false\n  - autoSelect: true\n", true},
		"empty list": {"", false},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			src := "metadata:\n  name: srt\nspec:\n"
			if tc.formats != "" {
				src += "  supportedModelFormats:\n" + tc.formats
			}
			got := parseServingRuntime(context.Background(), parseYAML(t, src))
			assert.Equal(t, tc.want, got.AutoSelect)
		})
	}
}

// Only required In-terms on the instance-type key count. A preferred
// affinity or a NotIn term describes something other than "this runtime
// may only land here", and rendering it as a restriction would mislead.
func TestParseServingRuntime_IgnoresSoftAndNegatedAffinity(t *testing.T) {
	t.Parallel()
	src := `
metadata:
  name: srt
spec:
  affinity:
    nodeAffinity:
      preferredDuringSchedulingIgnoredDuringExecution:
      - weight: 1
        preference:
          matchExpressions:
          - key: node.kubernetes.io/instance-type
            operator: In
            values: [BM.GPU.PREFERRED.1]
      requiredDuringSchedulingIgnoredDuringExecution:
        nodeSelectorTerms:
        - matchExpressions:
          - key: node.kubernetes.io/instance-type
            operator: NotIn
            values: [BM.GPU.EXCLUDED.1]
          - key: some.other/label
            operator: In
            values: [irrelevant]
`
	got := parseServingRuntime(context.Background(), parseYAML(t, src))
	assert.Empty(t, got.InstanceTypes)
}
