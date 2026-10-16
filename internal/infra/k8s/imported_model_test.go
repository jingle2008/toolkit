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
// two sources correctly, grouped by TenantID:
//
//  1. Namespaced BaseModel CRs across all namespaces (key from label
//     if present, else "UNKNOWN_TENANCY" for orphans).
//  2. Cluster-scoped ClusterBaseModel CRs WITH `tenancy-id` label
//     (key from label). CBMs WITHOUT a tenancy-id label are the
//     shared catalog (surfaced by LoadBaseModels) and must be
//     filtered out here.
func TestLoadImportedModels_BothSources(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	// Source 1a: namespaced BM WITHOUT a label — orphan bucket.
	ns1 := newNamespacedBM("team-a", "import-a",
		map[string]any{"vendor": "acme", "version": "v1"},
		map[string]any{"state": "Ready"},
		nil, nil)
	// Source 1b: namespaced BM WITH a label — tenant bucket.
	ns2 := newNamespacedBM("team-b", "import-b",
		map[string]any{"vendor": "acme", "version": "v2"},
		map[string]any{"state": "Ready"},
		map[string]string{"tenancy-id": "ocid1.tenancy.b"}, nil)

	// Source 2a: cluster-scoped CBM WITH label — tenant bucket.
	cbTenantScoped := newCBM("cluster-tenant",
		map[string]any{"vendor": "acme", "version": "v1"},
		map[string]any{"state": "Ready"},
		map[string]string{"tenancy-id": "ocid1.tenancy.c"}, nil)
	// Source 2b: cluster-scoped CBM WITHOUT label — shared catalog, must be filtered.
	cbShared := newCBM("cluster-shared",
		map[string]any{"vendor": "acme", "version": "v1"},
		map[string]any{"state": "Ready"},
		nil, nil)

	scheme := runtime.NewScheme()
	gvrToKind := map[schema.GroupVersionResource]string{
		{Group: "ome.io", Version: "v1beta1", Resource: "basemodels"}:        "BaseModelList",
		{Group: "ome.io", Version: "v1beta1", Resource: "clusterbasemodels"}: "ClusterBaseModelList",
	}
	client := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, gvrToKind, ns1, ns2, cbTenantScoped, cbShared)

	out, err := LoadImportedModels(ctx, client)
	require.NoError(t, err)
	require.Len(t, out, 3, "expected three tenant buckets: UNKNOWN_TENANCY + ocid1.tenancy.b + ocid1.tenancy.c")

	// Bucket 1: orphan namespaced CR
	orphan := out["UNKNOWN_TENANCY"]
	require.Len(t, orphan, 1)
	assert.Equal(t, "import-a", orphan[0].Name)
	assert.Equal(t, "team-a", orphan[0].Namespace, "namespace preserved even when TenantID is the orphan sentinel")
	assert.Equal(t, "UNKNOWN_TENANCY", orphan[0].TenantID, "TenantID on the value matches the bucket key")

	// Bucket 2: namespaced CR with tenancy-id label
	b := out["ocid1.tenancy.b"]
	require.Len(t, b, 1)
	assert.Equal(t, "import-b", b[0].Name)
	assert.Equal(t, "team-b", b[0].Namespace)
	assert.Equal(t, "ocid1.tenancy.b", b[0].TenantID)

	// Bucket 3: cluster-scoped CBM with label
	c := out["ocid1.tenancy.c"]
	require.Len(t, c, 1)
	assert.Equal(t, "cluster-tenant", c[0].Name)
	assert.Empty(t, c[0].Namespace, "cluster-scoped CR has no namespace")
	assert.Equal(t, "ocid1.tenancy.c", c[0].TenantID)

	// Shared CBM must be absent from every bucket.
	for k, bucket := range out {
		for _, item := range bucket {
			assert.NotEqual(t, "cluster-shared", item.Name,
				"shared CBM leaked into bucket %q", k)
		}
	}
	_ = models.ImportedModel{} // model package referenced for clarity
}

// Copies of one model in several namespaces — the tenant's `default`
// library copy plus the copies made into DAC namespaces for serving — are
// one ImportedModel: `default` is the primary, the rest are Copies.
func TestLoadImportedModels_MergesCopies(t *testing.T) {
	t.Parallel()
	tenancy := map[string]string{"tenancy-id": "ocid1.tenancy.t"}
	spec := map[string]any{"vendor": "acme", "version": "v1"}
	ready := map[string]any{"state": "Ready"}
	objs := []runtime.Object{
		// DAC-namespace copies carry the source's HF annotations (as on dp-prod-ord).
		newNamespacedBM("amaaaaaa-dac2", "amaaaaaa-m1", spec, ready, tenancy, map[string]string{"hf-model-id": "org/model"}),
		newNamespacedBM("default", "amaaaaaa-m1", spec, ready, tenancy, nil),
		newNamespacedBM("amaaaaaa-dac1", "amaaaaaa-m1", spec, ready, tenancy, nil),
		newNamespacedBM("default", "amaaaaaa-m2", spec, ready, tenancy, nil),
		// No `default` copy: the first namespace in sort order is primary.
		newNamespacedBM("amaaaaaa-dac9", "amaaaaaa-m3", spec, ready, tenancy, nil),
		newNamespacedBM("amaaaaaa-dac3", "amaaaaaa-m3", spec, ready, tenancy, nil),
	}
	gvrToKind := map[schema.GroupVersionResource]string{
		{Group: "ome.io", Version: "v1beta1", Resource: "basemodels"}:        "BaseModelList",
		{Group: "ome.io", Version: "v1beta1", Resource: "clusterbasemodels"}: "ClusterBaseModelList",
	}
	client := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), gvrToKind, objs...)

	out, err := LoadImportedModels(context.Background(), client)
	require.NoError(t, err)
	byName := map[string]models.ImportedModel{}
	for _, m := range out["ocid1.tenancy.t"] {
		byName[m.Name] = m
	}
	require.Len(t, byName, 3, "one row per model, not per copy")
	assert.Equal(t, "default", byName["amaaaaaa-m1"].Namespace)
	assert.Equal(t, []string{"amaaaaaa-dac1", "amaaaaaa-dac2"}, byName["amaaaaaa-m1"].Copies)
	assert.Equal(t, []string{"amaaaaaa-dac1", "amaaaaaa-dac2"}, byName["amaaaaaa-m1"].DACNamespaces(), "served by two DACs")
	assert.Empty(t, byName["amaaaaaa-m2"].Copies)
	assert.Empty(t, byName["amaaaaaa-m2"].DACNamespaces(), "only in the library: not on a DAC")
	assert.Equal(t, "amaaaaaa-dac3", byName["amaaaaaa-m3"].Namespace)
	assert.Equal(t, []string{"amaaaaaa-dac9"}, byName["amaaaaaa-m3"].Copies)
}
