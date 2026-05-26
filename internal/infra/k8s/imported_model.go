package k8s

import (
	"context"
	"fmt"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"

	"github.com/jingle2008/toolkit/pkg/models"
)

// LoadImportedModels returns the tenant-imported model catalog
// grouped by TenantID, mirroring LoadDedicatedAIClusters. Two
// sources are merged:
//
//  1. Namespaced ome.io BaseModel CRs across all namespaces.
//  2. Cluster-scoped ClusterBaseModel CRs carrying a `tenancy-id`
//     label. ClusterBaseModels WITHOUT the label are the shared
//     catalog (surfaced by LoadBaseModels instead).
//
// The label value is the authoritative tenant key. A namespaced
// CR without a `tenancy-id` label is a config error (every
// imported model should declare its tenancy); we bucket those under
// `"UNKNOWN_TENANCY"` so they're visible-as-orphan rather than
// silently dropped — same convention as LoadDedicatedAIClusters.
//
// Both sources reuse parseBaseModel for the shared spec/status
// fields. Atomicity: if either list call fails, the whole load
// fails — we don't return a partial result. The two API calls hit
// different GVRs (namespaced `basemodels` vs cluster-scoped
// `clusterbasemodels`) that often have asymmetric RBAC; a caller
// with namespaced-only access will see a hard failure rather than
// half the catalog. This is a deliberate choice over the
// LoadGPUPools-style PartialLoadError idiom because the two
// sources are conceptually one catalog — a missing half is more
// confusing than an explicit error.
func LoadImportedModels(ctx context.Context, client dynamic.Interface) (map[string][]models.ImportedModel, error) {
	namespacedGVR := schema.GroupVersionResource{
		Group:    "ome.io",
		Version:  "v1beta1",
		Resource: "basemodels",
	}
	nsList, err := client.Resource(namespacedGVR).List(ctx, v1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list BaseModel: %w", err)
	}

	clusterGVR := schema.GroupVersionResource{
		Group:    "ome.io",
		Version:  "v1beta1",
		Resource: "clusterbasemodels",
	}
	cbList, err := client.Resource(clusterGVR).List(ctx, v1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list ClusterBaseModel: %w", err)
	}

	result := make(map[string][]models.ImportedModel)
	for _, item := range nsList.Items {
		labels, _, _ := unstructured.NestedMap(item.Object, "metadata", "labels")
		tenantID := tenantIDFromLabels(labels) // nil labels → UNKNOWN_TENANCY (orphan bucket)
		result[tenantID] = append(result[tenantID], models.ImportedModel{
			BaseModel: parseBaseModel(&item),
			Namespace: item.GetNamespace(),
			TenantID:  tenantID,
		})
	}
	for _, item := range cbList.Items {
		labels, hasLabels, _ := unstructured.NestedMap(item.Object, "metadata", "labels")
		if !hasLabels {
			continue // shared catalog
		}
		if _, ok := labels["tenancy-id"]; !ok {
			// Shared catalog (no tenancy-id label) — surfaced by
			// LoadBaseModels instead. Cluster-scoped CBMs in this
			// loader leave Namespace empty by construction.
			continue
		}
		tenantID := tenantIDFromLabels(labels)
		result[tenantID] = append(result[tenantID], models.ImportedModel{
			BaseModel: parseBaseModel(&item),
			TenantID:  tenantID,
		})
	}
	return result, nil
}
