package k8s

import (
	"context"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic"

	"github.com/jingle2008/toolkit/pkg/infra/logging"
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
		if labels[appLabel] == reservationLabel {
			idlePods++
			continue
		}

		annos := getAnnotations(item)
		if modelName, ok := annos[baseModelLabelV2]; ok {
			modelNameMap[modelName] = struct{}{}
		} else if modelName, ok := labels[baseModelLabelV1]; ok {
			modelNameMap[modelName] = struct{}{}
		} else {
			logger.Errorw("workload pod without base model annotation/label",
				"pod", item.GetName(), "namespace", item.GetNamespace())
		}

		if _, ok := labels[servingLabelV1]; ok {
			componentMap[servingLabelV1] = struct{}{}
		} else if _, ok := labels[servingLabelV2]; ok {
			componentMap[servingLabelV2] = struct{}{}
		} else if _, ok := labels[trainingLabelV2]; ok {
			componentMap[trainingLabelV2] = struct{}{}
		} else {
			logger.Errorw("workload pod without serving/training label",
				"pod", item.GetName(), "namespace", item.GetNamespace())
		}
	}

	component := getUniqeKey(logger, componentMap, namespace)
	workloadType := component
	switch component {
	case servingLabelV1, servingLabelV2:
		workloadType = "Hosting"
	case trainingLabelV2:
		workloadType = "Fine-tuning"
	}

	return PodStats{
		IdlePods:  idlePods,
		TotalPods: totalPods,
		ModelName: getUniqeKey(logger, modelNameMap, namespace),
		Type:      workloadType,
	}
}

func getUniqeKey(logger logging.Logger, m map[string]struct{}, namespace string) string {
	if len(m) == 0 {
		return ""
	}

	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}

	if len(keys) > 1 {
		logger.Errorw("multiple configs found on workload pods in namespace",
			"values", keys, "namespace", namespace)
		return ""
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

// getAnnotations safely extracts annotations from an unstructured pod.
func getAnnotations(item *unstructured.Unstructured) map[string]string {
	annos := make(map[string]string)
	raw, found, _ := unstructured.NestedMap(item.Object, "metadata", "annotations")
	if !found {
		return annos
	}
	for k, v := range raw {
		if s, ok := v.(string); ok {
			annos[k] = s
		}
	}
	return annos
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
