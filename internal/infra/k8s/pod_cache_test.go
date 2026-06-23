package k8s

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestGetUniqeKey(t *testing.T) {
	t.Parallel()
	logger := &mockLogger{}
	// empty map
	assert.Equal(t, "", getUniqeKey(logger, map[string]struct{}{}, "label"))
	// single entry
	m := map[string]struct{}{"foo": {}}
	assert.Equal(t, "foo", getUniqeKey(logger, m, "label"))
	// multiple entries
	m2 := map[string]struct{}{"foo": {}, "bar": {}}
	assert.Equal(t, "", getUniqeKey(logger, m2, "label"))
}

func makeUnstructuredPod(labels, annos map[string]string, name string) *unstructured.Unstructured {
	// Convert to map[string]interface{} as expected by unstructured helpers.
	labelsIfc := make(map[string]any, len(labels))
	for k, v := range labels {
		labelsIfc[k] = v
	}
	annosIfc := make(map[string]any, len(annos))
	for k, v := range annos {
		annosIfc[k] = v
	}

	obj := map[string]any{
		"metadata": map[string]any{
			"name":        name,
			"namespace":   "ns1",
			"labels":      labelsIfc,
			"annotations": annosIfc,
		},
	}
	return &unstructured.Unstructured{Object: obj}
}

// withGPURequest gives a pod a single container requesting one
// nvidia.com/gpu, mirroring how real GPU workloads declare GPU usage.
func withGPURequest(p *unstructured.Unstructured) *unstructured.Unstructured {
	containers := []any{
		map[string]any{
			"name": "main",
			"resources": map[string]any{
				"requests": map[string]any{
					string(gpuProperty): "1",
				},
			},
		},
	}
	_ = unstructured.SetNestedSlice(p.Object, containers, "spec", "containers")
	return p
}

func TestGetPodStats(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	idle := withGPURequest(makeUnstructuredPod(map[string]string{appLabel: reservationLabel}, nil, "idle"))
	// workload pod with annotation
	workAnn := withGPURequest(makeUnstructuredPod(
		map[string]string{servingLabelV1: "dummy"},
		map[string]string{baseModelLabelV2: "m1"},
		"w1",
	))
	// workload pod missing model/component (still requests GPU, counts)
	bad := withGPURequest(makeUnstructuredPod(map[string]string{}, nil, "bad"))
	// serving pod that requests no GPU — excluded from the counts
	nonGPU := makeUnstructuredPod(
		map[string]string{servingLabelV1: "dummy"},
		map[string]string{baseModelLabelV2: "m2"},
		"non-gpu",
	)

	cache := PodCache{byNS: map[string][]*unstructured.Unstructured{
		"ns1": {idle, workAnn, bad, nonGPU},
	}}
	stats := cache.getPodStats(ctx, "ns1")
	assert.Equal(t, 1, stats.IdlePods)
	assert.Equal(t, 3, stats.TotalPods)
	assert.Equal(t, "m1", stats.ModelName)
	assert.Equal(t, "Hosting", stats.Type)
}
