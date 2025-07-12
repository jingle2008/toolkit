package k8s

import (
	"context"
	"slices"
	"strings"
	"time"

	models "github.com/jingle2008/toolkit/pkg/models"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

// ListDedicatedAIClusters returns all DedicatedAICluster resources from both v1alpha1 and v1beta1 CRDs.
func ListDedicatedAIClusters(ctx context.Context, client dynamic.Interface) ([]models.DedicatedAICluster, error) {
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

// listDedicatedAIClustersGeneric fetches DedicatedAIClusters using a GVR and extractor.
func listDedicatedAIClustersGeneric(
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

// listDedicatedAIClustersV1 fetches DedicatedAIClusters from v1alpha1 CRD
func listDedicatedAIClustersV1(ctx context.Context, client dynamic.Interface, cache PodCache) ([]models.DedicatedAICluster, error) {
	gvr := schema.GroupVersionResource{
		Group:    "ome.oracle.com",
		Version:  "v1alpha1",
		Resource: "dedicatedaiclusters",
	}
	return listDedicatedAIClustersGeneric(ctx, client, gvr, func(item unstructured.Unstructured) models.DedicatedAICluster {
		name, _, _ := unstructured.NestedString(item.Object, "metadata", "name")
		spec, _, _ := unstructured.NestedMap(item.Object, "spec")
		status, _, _ := unstructured.NestedMap(item.Object, "status")
		labels, hasLabels, _ := unstructured.NestedMap(item.Object, "metadata", "labels")
		creationTimestampStr, _, _ := unstructured.NestedString(item.Object, "metadata", "creationTimestamp")
		var age string
		if t, err := time.Parse(time.RFC3339, creationTimestampStr); err == nil {
			age = FormatAge(time.Since(t))
		}

		dacType, _ := spec["type"].(string)
		unitShape, _ := spec["unitShape"].(string)
		size, _ := spec["size"].(int64)

		tenantID := "missing"
		if hasLabels {
			tenantID = tenantIDFromLabels(labels)
		}

		statusStr, _ := status["status"].(string)
		if statusStr == "" {
			statusStr = "pending"
		}

		stats := cache.getPodStats(ctx, name)
		return models.DedicatedAICluster{
			Name:          name,
			Type:          dacType,
			UnitShape:     unitShape,
			Size:          int(size),
			Status:        statusStr,
			TenantID:      tenantID,
			ModelName:     stats.ModelName,
			IdleReplicas:  stats.IdlePods,
			TotalReplicas: stats.TotalPods,
			Age:           age,
		}
	})
}

// listDedicatedAIClustersV2 fetches DedicatedAIClusters from v1beta1 CRD
func listDedicatedAIClustersV2(ctx context.Context, client dynamic.Interface, cache PodCache) ([]models.DedicatedAICluster, error) {
	gvr := schema.GroupVersionResource{
		Group:    "ome.io",
		Version:  "v1beta1",
		Resource: "dedicatedaiclusters",
	}
	return listDedicatedAIClustersGeneric(ctx, client, gvr, func(item unstructured.Unstructured) models.DedicatedAICluster {
		name, _, _ := unstructured.NestedString(item.Object, "metadata", "name")
		spec, _, _ := unstructured.NestedMap(item.Object, "spec")
		status, _, _ := unstructured.NestedMap(item.Object, "status")
		labels, hasLabels, _ := unstructured.NestedMap(item.Object, "metadata", "labels")
		creationTimestampStr, _, _ := unstructured.NestedString(item.Object, "metadata", "creationTimestamp")
		var age string
		if t, err := time.Parse(time.RFC3339, creationTimestampStr); err == nil {
			age = FormatAge(time.Since(t))
		}

		profile, _ := spec["profile"].(string)
		count, _ := spec["count"].(int64)

		tenantID := "missing"
		if hasLabels {
			tenantID = tenantIDFromLabels(labels)
		}

		dacLifecycleState, _ := status["dacLifecycleState"].(string)
		statusStr := dacLifecycleState
		if statusStr == "" {
			statusStr = "pending"
		}

		stats := cache.getPodStats(ctx, name)
		return models.DedicatedAICluster{
			Name:          name,
			Profile:       profile,
			Size:          int(count),
			Status:        statusStr,
			TenantID:      tenantID,
			ModelName:     stats.ModelName,
			IdleReplicas:  stats.IdlePods,
			TotalReplicas: stats.TotalPods,
			Type:          stats.Type,
			Age:           age,
		}
	})
}

/*
LoadDedicatedAIClusters loads DedicatedAICluster information using the provided DedicatedAIClusterLister.
*/
func LoadDedicatedAIClusters(ctx context.Context, client dynamic.Interface) (map[string][]models.DedicatedAICluster, error) {
	dacs, err := ListDedicatedAIClusters(ctx, client)
	if err != nil {
		return nil, err
	}

	result := make(map[string][]models.DedicatedAICluster)
	for _, dac := range dacs {
		result[dac.TenantID] = append(result[dac.TenantID], dac)
	}

	for _, v := range result {
		slices.SortFunc(v, func(a, b models.DedicatedAICluster) int {
			return strings.Compare(a.GetKey(), b.GetKey())
		})
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
