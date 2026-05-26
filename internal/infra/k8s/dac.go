package k8s

import (
	"context"
	"time"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"

	models "github.com/jingle2008/toolkit/pkg/models"
)

// listDedicatedAIClusters returns all DedicatedAICluster resources from both v1alpha1 and v1beta1 CRDs.
func listDedicatedAIClusters(ctx context.Context, client dynamic.Interface) ([]models.DedicatedAICluster, error) {
	cache, err := buildPodCache(ctx, client)
	if err != nil {
		return nil, err
	}
	v1Clusters, err := listDedicatedAIClustersV1(ctx, client, cache)
	if err != nil {
		return nil, err
	}
	v2Clusters, err := listDedicatedAIClustersV2(ctx, client, cache)
	if err != nil {
		return nil, err
	}
	return append(v1Clusters, v2Clusters...), nil
}

// listDACsWithOverlay fetches DedicatedAIClusters using a GVR and extractor.
func listDACsWithOverlay(
	ctx context.Context,
	client dynamic.Interface,
	gvr schema.GroupVersionResource,
	extract func(item unstructured.Unstructured) models.DedicatedAICluster,
) ([]models.DedicatedAICluster, error) {
	list, err := client.Resource(gvr).List(ctx, v1.ListOptions{})
	if err != nil {
		return nil, err
	}
	dacs := make([]models.DedicatedAICluster, 0, len(list.Items))
	for _, item := range list.Items {
		dacs = append(dacs, extract(item))
	}
	return dacs, nil
}

// dacOverlay applies version-specific spec fields on top of the
// shared DedicatedAICluster shell produced by extractDAC. stats is
// passed in so v1beta1 can populate Type from pod-derived metadata.
type dacOverlay func(spec map[string]any, stats PodStats, dac *models.DedicatedAICluster)

// extractDAC builds the per-item closure listDACsWithOverlay
// expects. statusField names the version-specific status string
// (v1alpha1: "status"; v1beta1: "dacLifecycleState"); overlay applies
// the remaining version-specific spec/stats overlays.
func extractDAC(ctx context.Context, cache PodCache, statusField string, overlay dacOverlay) func(unstructured.Unstructured) models.DedicatedAICluster {
	return func(item unstructured.Unstructured) models.DedicatedAICluster {
		name, _, _ := unstructured.NestedString(item.Object, "metadata", "name")
		spec, _, _ := unstructured.NestedMap(item.Object, "spec")
		status, _, _ := unstructured.NestedMap(item.Object, "status")
		labels, _, _ := unstructured.NestedMap(item.Object, "metadata", "labels")
		creationTimestampStr, _, _ := unstructured.NestedString(item.Object, "metadata", "creationTimestamp")

		var age string
		if t, err := time.Parse(time.RFC3339, creationTimestampStr); err == nil {
			age = FormatAge(time.Since(t))
		}

		// labels==nil (NestedMap returns nil when the field is absent
		// or non-map) is handled by tenantIDFromLabels: it returns
		// UNKNOWN_TENANCY, the same bucket used when labels are
		// present but lack `tenancy-id`. Keeps the orphan key
		// consistent across all "missing tenant" scenarios (and
		// across DAC + ImportedModel).
		tenantID := tenantIDFromLabels(labels)

		statusStr, _ := status[statusField].(string)
		if statusStr == "" {
			statusStr = "pending"
		}

		stats := cache.getPodStats(ctx, name)
		dac := models.DedicatedAICluster{
			Name:          name,
			Status:        statusStr,
			TenantID:      tenantID,
			ModelName:     stats.ModelName,
			IdleReplicas:  stats.IdlePods,
			TotalReplicas: stats.TotalPods,
			Age:           age,
		}
		overlay(spec, stats, &dac)
		return dac
	}
}

// listDedicatedAIClustersV1 fetches DedicatedAIClusters from v1alpha1 CRD
func listDedicatedAIClustersV1(ctx context.Context, client dynamic.Interface, cache PodCache) ([]models.DedicatedAICluster, error) {
	gvr := schema.GroupVersionResource{
		Group:    "ome.oracle.com",
		Version:  "v1alpha1",
		Resource: "dedicatedaiclusters",
	}
	return listDACsWithOverlay(ctx, client, gvr, extractDAC(ctx, cache, "status",
		func(spec map[string]any, _ PodStats, dac *models.DedicatedAICluster) {
			dac.Type, _ = spec["type"].(string)
			dac.UnitShape, _ = spec["unitShape"].(string)
			size, _ := spec["size"].(int64)
			dac.Size = int(size)
		}))
}

// listDedicatedAIClustersV2 fetches DedicatedAIClusters from v1beta1 CRD
func listDedicatedAIClustersV2(ctx context.Context, client dynamic.Interface, cache PodCache) ([]models.DedicatedAICluster, error) {
	gvr := schema.GroupVersionResource{
		Group:    "ome.io",
		Version:  "v1beta1",
		Resource: "dedicatedaiclusters",
	}
	return listDACsWithOverlay(ctx, client, gvr, extractDAC(ctx, cache, "dacLifecycleState",
		func(spec map[string]any, stats PodStats, dac *models.DedicatedAICluster) {
			dac.Profile, _ = spec["profile"].(string)
			count, _ := spec["count"].(int64)
			dac.Size = int(count)
			dac.Type = stats.Type
		}))
}

/*
LoadDedicatedAIClusters loads DedicatedAICluster information using the provided DedicatedAIClusterLister.
*/
func LoadDedicatedAIClusters(ctx context.Context, client dynamic.Interface) (map[string][]models.DedicatedAICluster, error) {
	dacs, err := listDedicatedAIClusters(ctx, client)
	if err != nil {
		return nil, err
	}

	result := make(map[string][]models.DedicatedAICluster)
	for _, dac := range dacs {
		result[dac.TenantID] = append(result[dac.TenantID], dac)
	}
	return result, nil
}

func tenantIDFromLabels(labels map[string]any) string {
	value := labels["tenancy-id"]
	if value != nil {
		if str, ok := value.(string); ok {
			return str
		}
	}
	return "UNKNOWN_TENANCY"
}
