package k8s

import (
	"context"
	"fmt"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"

	"github.com/jingle2008/toolkit/pkg/models"
)

// LoadImportedModels returns the tenant-imported model catalog: every
// namespaced ome.io BaseModel CR (across all namespaces) plus every
// cluster-scoped ClusterBaseModel CR that carries a `tenancy-id`
// label. ClusterBaseModels WITHOUT a `tenancy-id` label are the
// shared catalog and are surfaced by LoadBaseModels instead.
//
// Both sources reuse parseBaseModel for the shared spec/status
// fields, then wrap with the source-specific identity (namespace for
// namespaced CRs; tenancy-id label value for cluster-scoped CRs).
func LoadImportedModels(ctx context.Context, client dynamic.Interface) ([]models.ImportedModel, error) {
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

	result := make([]models.ImportedModel, 0, len(nsList.Items)+len(cbList.Items))
	for _, item := range nsList.Items {
		labels := getLabels(&item)
		result = append(result, models.ImportedModel{
			BaseModel: parseBaseModel(&item),
			Namespace: item.GetNamespace(),
			TenantID:  labels["tenancy-id"], // may be empty for namespaced CRs
		})
	}
	for _, item := range cbList.Items {
		tenantID, ok := getLabels(&item)["tenancy-id"]
		if !ok {
			// Shared catalog (no tenancy-id label) — surfaced by
			// LoadBaseModels instead. Cluster-scoped CBMs in this
			// loader leave Namespace empty by construction.
			continue
		}
		result = append(result, models.ImportedModel{
			BaseModel: parseBaseModel(&item),
			TenantID:  tenantID,
		})
	}
	return result, nil
}
