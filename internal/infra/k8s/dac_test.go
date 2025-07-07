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

func makeUnstructuredDACV2(name, profile string, count int64, status, tenantID string) *unstructured.Unstructured {
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
			"dacLifecycleState": status,
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
		makeUnstructuredDACV2("dac2", "profileA", 3, "active", "tid2"),
	}
	listKinds := map[schema.GroupVersionResource]string{
		{Group: "ome.oracle.com", Version: "v1alpha1", Resource: "dedicatedaiclusters"}: "DedicatedAIClusterList",
		{Group: "ome.io", Version: "v1beta1", Resource: "dedicatedaiclusters"}:          "DedicatedAIClusterList",
		{Group: "", Version: "v1", Resource: "pods"}:                                    "PodList",
	}
	client := fake.NewSimpleDynamicClientWithCustomListKinds(scheme, listKinds, objs...)

	ctx := context.Background()
	clusters, err := ListDedicatedAIClusters(ctx, client)
	require.NoError(t, err)
	assert.Len(t, clusters, 2)
}

func TestListDedicatedAIClusters_ErrorV1(t *testing.T) {
	t.Parallel()
	scheme := runtime.NewScheme()
	listKinds := map[schema.GroupVersionResource]string{
		{Group: "ome.oracle.com", Version: "v1alpha1", Resource: "dedicatedaiclusters"}: "DedicatedAIClusterList",
		{Group: "ome.io", Version: "v1beta1", Resource: "dedicatedaiclusters"}:          "DedicatedAIClusterList",
		{Group: "", Version: "v1", Resource: "pods"}:                                    "PodList",
	}
	client := fake.NewSimpleDynamicClientWithCustomListKinds(scheme, listKinds)
	// Patch the client to return error for v1alpha1
	gvrV1 := schema.GroupVersionResource{Group: "ome.oracle.com", Version: "v1alpha1", Resource: "dedicatedaiclusters"}
	client.PrependReactor("list", "dedicatedaiclusters", func(action cgotesting.Action) (handled bool, ret runtime.Object, err error) {
		if action.GetResource() == gvrV1 {
			return true, nil, errors.New("v1 error")
		}
		return false, nil, nil
	})

	ctx := context.Background()
	_, err := ListDedicatedAIClusters(ctx, client)
	assert.Error(t, err)
}

func TestListDedicatedAIClusters_ErrorV2(t *testing.T) {
	t.Parallel()
	scheme := runtime.NewScheme()
	listKinds := map[schema.GroupVersionResource]string{
		{Group: "ome.oracle.com", Version: "v1alpha1", Resource: "dedicatedaiclusters"}: "DedicatedAIClusterList",
		{Group: "ome.io", Version: "v1beta1", Resource: "dedicatedaiclusters"}:          "DedicatedAIClusterList",
		{Group: "", Version: "v1", Resource: "pods"}:                                    "PodList",
	}
	client := fake.NewSimpleDynamicClientWithCustomListKinds(scheme, listKinds)
	// Patch the client to return error for v1beta1
	gvrV2 := schema.GroupVersionResource{Group: "ome.io", Version: "v1beta1", Resource: "dedicatedaiclusters"}
	client.PrependReactor("list", "dedicatedaiclusters", func(action cgotesting.Action) (handled bool, ret runtime.Object, err error) {
		if action.GetResource() == gvrV2 {
			return true, nil, errors.New("v2 error")
		}
		return false, nil, nil
	})

	ctx := context.Background()
	_, err := ListDedicatedAIClusters(ctx, client)
	assert.Error(t, err)
}

func TestLoadDedicatedAIClusters_HappyPath(t *testing.T) {
	t.Parallel()
	scheme := runtime.NewScheme()
	objs := []runtime.Object{
		makeUnstructuredDACV1("dac1", "GPU", 2, "ready", "tid1"),
		makeUnstructuredDACV2("dac2", "profileA", 3, "active", "tid2"),
	}
	listKinds := map[schema.GroupVersionResource]string{
		{Group: "ome.oracle.com", Version: "v1alpha1", Resource: "dedicatedaiclusters"}: "DedicatedAIClusterList",
		{Group: "ome.io", Version: "v1beta1", Resource: "dedicatedaiclusters"}:          "DedicatedAIClusterList",
		{Group: "", Version: "v1", Resource: "pods"}:                                    "PodList",
	}
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
	listKinds := map[schema.GroupVersionResource]string{
		{Group: "ome.oracle.com", Version: "v1alpha1", Resource: "dedicatedaiclusters"}: "DedicatedAIClusterList",
		{Group: "ome.io", Version: "v1beta1", Resource: "dedicatedaiclusters"}:          "DedicatedAIClusterList",
		{Group: "", Version: "v1", Resource: "pods"}:                                    "PodList",
	}
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
	listKinds := map[schema.GroupVersionResource]string{
		{Group: "ome.oracle.com", Version: "v1alpha1", Resource: "dedicatedaiclusters"}: "DedicatedAIClusterList",
		{Group: "ome.io", Version: "v1beta1", Resource: "dedicatedaiclusters"}:          "DedicatedAIClusterList",
		{Group: "", Version: "v1", Resource: "pods"}:                                    "PodList",
	}
	client := fake.NewSimpleDynamicClientWithCustomListKinds(scheme, listKinds, obj)

	ctx := context.Background()
	clusters, err := ListDedicatedAIClusters(ctx, client)
	require.NoError(t, err)
	assert.Len(t, clusters, 1) // Should still return, with defaults
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
	listKinds := map[schema.GroupVersionResource]string{
		{Group: "ome.oracle.com", Version: "v1alpha1", Resource: "dedicatedaiclusters"}: "DedicatedAIClusterList",
		{Group: "ome.io", Version: "v1beta1", Resource: "dedicatedaiclusters"}:          "DedicatedAIClusterList",
	}
	client := fake.NewSimpleDynamicClientWithCustomListKinds(scheme, listKinds, obj)

	ctx := context.Background()
	clusters, err := listDedicatedAIClustersV2(ctx, client, PodCache{byNS: map[string][]*unstructured.Unstructured{}})
	require.NoError(t, err)
	assert.Len(t, clusters, 1)
}
