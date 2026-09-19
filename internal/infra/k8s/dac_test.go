package k8s

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic/fake"
	cgotesting "k8s.io/client-go/testing"
)

func TestTenantIDFromLabels(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		labels map[string]any
		want   string
	}{
		{
			name:   "string tenancy-id",
			labels: map[string]any{"tenancy-id": "tid"},
			want:   "tid",
		},
		{
			name:   "missing tenancy-id",
			labels: map[string]any{},
			want:   "UNKNOWN_TENANCY",
		},
		{
			name:   "non-string tenancy-id",
			labels: map[string]any{"tenancy-id": 123},
			want:   "UNKNOWN_TENANCY",
		},
		{
			name:   "nil labels",
			labels: nil,
			want:   "UNKNOWN_TENANCY",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, tenantIDFromLabels(tt.labels))
		})
	}
}

// dacListKinds registers every resource the DAC listers touch with the
// fake dynamic client (it panics on LIST of an unregistered resource).
func dacListKinds() map[schema.GroupVersionResource]string {
	return map[schema.GroupVersionResource]string{
		{Group: "ome.oracle.com", Version: "v1alpha1", Resource: "dedicatedaiclusters"}: "DedicatedAIClusterList",
		{Group: "ome.io", Version: "v1beta1", Resource: "dedicatedaiclusters"}:          "DedicatedAIClusterList",
		{Group: "ome.io", Version: "v1beta1", Resource: "dedicatedaiclusterprofiles"}:   "DedicatedAIClusterProfileList",
		{Group: "", Version: "v1", Resource: "pods"}:                                    "PodList",
	}
}

func makeUnstructuredDACProfile(name string, count int64) *unstructured.Unstructured {
	obj := map[string]any{
		"apiVersion": "ome.io/v1beta1",
		"kind":       "DedicatedAIClusterProfile",
		"metadata": map[string]any{
			"name": name,
		},
		"spec": map[string]any{
			"count": count,
		},
	}
	return &unstructured.Unstructured{Object: obj}
}

func makeUnstructuredDACV1(name, dacType string, size int64, status, tenantID string) *unstructured.Unstructured {
	obj := map[string]any{
		"apiVersion": "ome.oracle.com/v1alpha1",
		"kind":       "DedicatedAICluster",
		"metadata": map[string]any{
			"name":   name,
			"labels": map[string]any{"tenancy-id": tenantID},
		},
		"spec": map[string]any{
			"type":      dacType,
			"unitShape": "shape",
			"size":      size,
		},
		"status": map[string]any{
			"status": status,
		},
	}
	return &unstructured.Unstructured{Object: obj}
}

func makeUnstructuredDACV2(name, profile string, count int64, tenantID string) *unstructured.Unstructured {
	obj := map[string]any{
		"apiVersion": "ome.io/v1beta1",
		"kind":       "DedicatedAICluster",
		"metadata": map[string]any{
			"name":   name,
			"labels": map[string]any{"tenancy-id": tenantID},
		},
		"spec": map[string]any{
			"profile": profile,
			"count":   count,
		},
		"status": map[string]any{
			"dacLifecycleState": "active",
		},
	}
	return &unstructured.Unstructured{Object: obj}
}

func TestListDedicatedAIClusters_HappyPath(t *testing.T) {
	t.Parallel()
	scheme := runtime.NewScheme()
	// Seed both v1 and v2 objects
	objs := []runtime.Object{
		makeUnstructuredDACV1("dac1", "GPU", 2, "ready", "tid1"),
		makeUnstructuredDACV2("dac2", "profileA", 3, "tid2"),
	}
	listKinds := dacListKinds()
	client := fake.NewSimpleDynamicClientWithCustomListKinds(scheme, listKinds, objs...)

	ctx := context.Background()
	clusters, err := listDedicatedAIClusters(ctx, client)
	require.NoError(t, err)
	assert.Len(t, clusters, 2)
}

func TestListDedicatedAIClusters_ErrorCases(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		gvr    schema.GroupVersionResource
		errMsg string
	}{
		{
			name:   "ErrorV1",
			gvr:    schema.GroupVersionResource{Group: "ome.oracle.com", Version: "v1alpha1", Resource: "dedicatedaiclusters"},
			errMsg: "v1 error",
		},
		{
			name:   "ErrorV2",
			gvr:    schema.GroupVersionResource{Group: "ome.io", Version: "v1beta1", Resource: "dedicatedaiclusters"},
			errMsg: "v2 error",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			scheme := runtime.NewScheme()
			listKinds := dacListKinds()
			client := fake.NewSimpleDynamicClientWithCustomListKinds(scheme, listKinds)
			// Patch the client to return error for the test case GVR
			client.PrependReactor("list", "dedicatedaiclusters", func(action cgotesting.Action) (handled bool, ret runtime.Object, err error) {
				if action.GetResource() == tc.gvr {
					return true, nil, errors.New(tc.errMsg)
				}
				return false, nil, nil
			})

			ctx := context.Background()
			_, err := listDedicatedAIClusters(ctx, client)
			assert.Error(t, err)
		})
	}
}

func TestLoadDedicatedAIClusters_HappyPath(t *testing.T) {
	t.Parallel()
	scheme := runtime.NewScheme()
	objs := []runtime.Object{
		makeUnstructuredDACV1("dac1", "GPU", 2, "ready", "tid1"),
		makeUnstructuredDACV2("dac2", "profileA", 3, "tid2"),
	}
	listKinds := dacListKinds()
	client := fake.NewSimpleDynamicClientWithCustomListKinds(scheme, listKinds, objs...)

	ctx := context.Background()
	result, err := LoadDedicatedAIClusters(ctx, client)
	require.NoError(t, err)
	assert.Contains(t, result, "tid1")
	assert.Contains(t, result, "tid2")
}

func TestLoadDedicatedAIClusters_Empty(t *testing.T) {
	t.Parallel()
	scheme := runtime.NewScheme()
	listKinds := dacListKinds()
	client := fake.NewSimpleDynamicClientWithCustomListKinds(scheme, listKinds)
	ctx := context.Background()
	result, err := LoadDedicatedAIClusters(ctx, client)
	require.NoError(t, err)
	assert.Empty(t, result)
}

func TestListDedicatedAIClusters_MalformedObject(t *testing.T) {
	t.Parallel()
	scheme := runtime.NewScheme()
	// Object missing spec/status fields
	obj := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "ome.oracle.com/v1alpha1",
		"kind":       "DedicatedAICluster",
		"metadata": map[string]any{
			"name":   "dac-bad",
			"labels": map[string]any{"tenancy-id": "tid-bad"},
		},
	}}
	listKinds := dacListKinds()
	client := fake.NewSimpleDynamicClientWithCustomListKinds(scheme, listKinds, obj)

	ctx := context.Background()
	clusters, err := listDedicatedAIClusters(ctx, client)
	require.NoError(t, err)
	assert.Len(t, clusters, 1) // Should still return, with defaults
}

func TestListDedicatedAIClustersV2_SizeInProfileUnits(t *testing.T) {
	t.Parallel()
	scheme := runtime.NewScheme()
	objs := []runtime.Object{
		makeUnstructuredDACProfile("multi-pod", 2),
		makeUnstructuredDACProfile("single-pod", 1),
		// 4 pods / 2 pods-per-unit = 2 units
		makeUnstructuredDACV2("dac-multi", "multi-pod", 4, "tid1"),
		// 3 pods / 1 pod-per-unit = 3 units
		makeUnstructuredDACV2("dac-single", "single-pod", 3, "tid1"),
		// partial unit rounds up: 3 pods / 2 pods-per-unit = 2 units
		makeUnstructuredDACV2("dac-partial", "multi-pod", 3, "tid1"),
		// unknown profile falls back to raw pod count
		makeUnstructuredDACV2("dac-orphan", "no-such-profile", 5, "tid1"),
	}
	client := fake.NewSimpleDynamicClientWithCustomListKinds(scheme, dacListKinds(), objs...)

	clusters, err := listDedicatedAIClustersV2(context.Background(), client,
		PodCache{byNS: map[string][]*unstructured.Unstructured{}})
	require.NoError(t, err)

	sizes := make(map[string]int, len(clusters))
	for _, c := range clusters {
		sizes[c.Name] = c.Size
	}
	assert.Equal(t, map[string]int{
		"dac-multi":   2,
		"dac-single":  3,
		"dac-partial": 2,
		"dac-orphan":  5,
	}, sizes)
}

func TestListDedicatedAIClustersV2_MalformedObject(t *testing.T) {
	t.Parallel()
	scheme := runtime.NewScheme()
	// Object missing spec/status fields
	obj := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "ome.io/v1beta1",
		"kind":       "DedicatedAICluster",
		"metadata": map[string]any{
			"name":   "dac-bad-v2",
			"labels": map[string]any{"tenancy-id": "tid-bad-v2"},
		},
	}}
	listKinds := dacListKinds()
	client := fake.NewSimpleDynamicClientWithCustomListKinds(scheme, listKinds, obj)

	ctx := context.Background()
	clusters, err := listDedicatedAIClustersV2(ctx, client, PodCache{byNS: map[string][]*unstructured.Unstructured{}})
	require.NoError(t, err)
	assert.Len(t, clusters, 1)
}
