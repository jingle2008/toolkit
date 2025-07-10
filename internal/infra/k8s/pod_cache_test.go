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

func makeUnstructuredPod(labels, annos map[string]string, name, ns string) *unstructured.Unstructured {
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
			"namespace":   ns,
			"labels":      labelsIfc,
			"annotations": annosIfc,
		},
	}
	return &unstructured.Unstructured{Object: obj}
}

func TestGetPodStats(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	idle := makeUnstructuredPod(map[string]string{appLabel: reservationLabel}, nil, "idle", "ns1")
	// workload pod with annotation
	workAnn := makeUnstructuredPod(
		map[string]string{servingLabelV1: "dummy"},
		map[string]string{baseModelLabelV2: "m1"},
		"w1", "ns1")
	// workload pod missing model/component
	bad := makeUnstructuredPod(map[string]string{}, nil, "bad", "ns1")

	cache := PodCache{byNS: map[string][]*unstructured.Unstructured{
		"ns1": {idle, workAnn, bad},
	}}
	stats := cache.getPodStats(ctx, "ns1")
	assert.Equal(t, 1, stats.IdlePods)
	assert.Equal(t, 3, stats.TotalPods)
	assert.Equal(t, "m1", stats.ModelName)
	assert.Equal(t, "Hosting", stats.Type)
}
