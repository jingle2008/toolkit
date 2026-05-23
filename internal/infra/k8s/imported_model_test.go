package k8s

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"

	"github.com/jingle2008/toolkit/pkg/models"
)

// newNamespacedBM builds an ome.io/v1beta1 BaseModel (namespaced)
// unstructured fixture. Mirrors newCBM (which builds ClusterBaseModel)
// but sets a namespace.
func newNamespacedBM(namespace, name string, spec, status map[string]any, labels, ann map[string]string) *unstructured.Unstructured {
	labelsAny := map[string]any{}
	for k, v := range labels {
		labelsAny[k] = v
	}
	annAny := map[string]any{}
	for k, v := range ann {
		annAny[k] = v
	}
	return &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "ome.io/v1beta1",
			"kind":       "BaseModel",
			"metadata": map[string]any{
				"name":        name,
				"namespace":   namespace,
				"labels":      labelsAny,
				"annotations": annAny,
			},
			"spec":   spec,
			"status": status,
		},
	}
}

// TestLoadImportedModels_BothSources verifies the loader merges the
// two scripted sources correctly:
//  1. namespaced BaseModel CRs across all namespaces
//  2. cluster-scoped ClusterBaseModel CRs WITH `tenancy-id` label
//
// CBMs WITHOUT a tenancy-id label must be skipped (they're the shared
// catalog, surfaced by LoadBaseModels).
func TestLoadImportedModels_BothSources(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	// Source 1: two namespaced BaseModels in different namespaces.
	ns1 := newNamespacedBM("team-a", "import-a",
		map[string]any{"vendor": "acme", "version": "v1"},
		map[string]any{"state": "Ready"},
		nil, nil)
	// Namespaced CR with tenancy-id label (allowed; tenantId comes through).
	ns2 := newNamespacedBM("team-b", "import-b",
		map[string]any{"vendor": "acme", "version": "v2"},
		map[string]any{"state": "Ready"},
		map[string]string{"tenancy-id": "ocid1.tenancy.b"}, nil)

	// Source 2: one ClusterBaseModel WITH tenancy-id (imported), one
	// WITHOUT (shared catalog — must be skipped here).
	cbTenantScoped := newCBM("cluster-tenant",
		map[string]any{"vendor": "acme", "version": "v1"},
		map[string]any{"state": "Ready"},
		map[string]string{"tenancy-id": "ocid1.tenancy.c"}, nil)
	cbShared := newCBM("cluster-shared",
		map[string]any{"vendor": "acme", "version": "v1"},
		map[string]any{"state": "Ready"},
		nil, nil)

	// dynamicfake needs the GVR-kind map registered for the namespaced
	// resource (which is a different Kind than ClusterBaseModel).
	scheme := runtime.NewScheme()
	gvrToKind := map[schema.GroupVersionResource]string{
		{Group: "ome.io", Version: "v1beta1", Resource: "basemodels"}:        "BaseModelList",
		{Group: "ome.io", Version: "v1beta1", Resource: "clusterbasemodels"}: "ClusterBaseModelList",
	}
	client := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, gvrToKind, ns1, ns2, cbTenantScoped, cbShared)

	out, err := LoadImportedModels(ctx, client)
	require.NoError(t, err)
	require.Len(t, out, 3, "expected ns1 + ns2 + cbTenantScoped; cbShared must be filtered")

	byName := map[string]models.ImportedModel{}
	for _, m := range out {
		byName[m.Name] = m
	}

	a, ok := byName["import-a"]
	require.True(t, ok)
	assert.Equal(t, "team-a", a.Namespace, "non-empty Namespace ⇒ namespaced BaseModel CR")
	assert.Empty(t, a.TenantID, "namespaced CR without tenancy-id label should have empty TenantID")

	b, ok := byName["import-b"]
	require.True(t, ok)
	assert.Equal(t, "team-b", b.Namespace)
	assert.Equal(t, "ocid1.tenancy.b", b.TenantID, "namespaced CR with tenancy-id label propagates it")

	c, ok := byName["cluster-tenant"]
	require.True(t, ok)
	assert.Empty(t, c.Namespace, "empty Namespace ⇒ cluster-scoped ClusterBaseModel CR")
	assert.Equal(t, "ocid1.tenancy.c", c.TenantID)

	_, sharedLeaked := byName["cluster-shared"]
	assert.False(t, sharedLeaked, "ClusterBaseModel without tenancy-id label must not appear in imported models")
}
