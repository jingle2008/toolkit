package k8s

import (
	"context"

	"github.com/jingle2008/toolkit/pkg/infra/logging"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic"
)

// PodStats holds pod statistics for a namespace.
type PodStats struct {
	TotalPods int
	IdlePods  int
	ModelName string
	Type      string
}

// PodCache maps namespace name to pods.
type PodCache struct {
	byNS map[string][]*unstructured.Unstructured
}

func (c PodCache) getPodStats(ctx context.Context, namespace string) PodStats {
	pods := c.byNS[namespace]
	idlePods, totalPods := 0, len(pods)
	componentMap := make(map[string]struct{})
	modelNameMap := make(map[string]struct{})

	logger := logging.FromContext(ctx)
	for _, item := range pods {
		labels := getLabels(item)
		app := labels[appLabel]
		if app == idleTagV1 || app == idleTagV2 {
			idlePods++
			continue
		}

		if modelName, ok := labels[baseModelLabel]; ok {
			modelNameMap[modelName] = struct{}{}
		} else {
			logger.Errorw("workload pod without base model label",
				"pod", item.GetName(), "namespace", item.GetNamespace())
		}

		if component, ok := labels[componentLabel]; ok {
			componentMap[component] = struct{}{}
		} else {
			logger.Errorw("workload pod without componment label",
				"pod", item.GetName(), "namespace", item.GetNamespace())
		}
	}

	component := getUniqeKey(logger, componentMap, componentLabel, namespace)
	workloadType := component
	if component == predictorTag {
		workloadType = "Hosting"
	}

	return PodStats{
		IdlePods:  idlePods,
		TotalPods: totalPods,
		ModelName: getUniqeKey(logger, modelNameMap, baseModelLabel, namespace),
		Type:      workloadType,
	}
}

func getUniqeKey(logger logging.Logger, m map[string]struct{}, label, namespace string) string {
	if len(m) == 0 {
		return "n/a"
	}

	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}

	if len(keys) > 1 {
		logger.Errorw("multiple label values found on workload pods in namespace",
			"label", label, "values", keys, "namesapce", namespace)
		return "n/a"
	}

	return keys[0]
}

// getLabels safely extracts labels from an unstructured pod.
func getLabels(item *unstructured.Unstructured) map[string]string {
	labels := make(map[string]string)
	raw, found, _ := unstructured.NestedMap(item.Object, "metadata", "labels")
	if !found {
		return labels
	}
	for k, v := range raw {
		if s, ok := v.(string); ok {
			labels[k] = s
		}
	}
	return labels
}

func buildPodCache(ctx context.Context, client dynamic.Interface) (PodCache, error) {
	cache := PodCache{byNS: make(map[string][]*unstructured.Unstructured)}

	err := processPodQueries(ctx, client, gpuPodSelectors, runningPodSelector,
		listPodsWithSelectors,
		func(ns string, pods []*unstructured.Unstructured) {
			cache.byNS[ns] = append(cache.byNS[ns], pods...)
		})
	if err != nil {
		return cache, err
	}

	return cache, nil
}
