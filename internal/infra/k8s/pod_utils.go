package k8s

import (
	"context"
	"fmt"

	"github.com/jingle2008/toolkit/pkg/infra/logging"
	"golang.org/x/sync/errgroup"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
)

const (
	runningPodSelector = "status.phase=Running"
	idleTagV1          = "dummy"
	idleTagV2          = "reservation"
	appLabel           = "app"
	componentLabel     = "component"
	predictorTag       = "predictor"
	baseModelLabel     = "base-model-name"
)

var gpuPodSelectors = []string{
	fmt.Sprintf("%s in (%s,%s)", appLabel, idleTagV1, idleTagV2),
	fmt.Sprintf("%s=%s", componentLabel, predictorTag),
	"ome.oracle.com/trainingjob",
}

func getGpuAllocations(
	ctx context.Context,
	clientset kubernetes.Interface,
	labelSelector string,
	fieldSelector string,
) (map[string]int64, error) {
	pods, err := clientset.CoreV1().Pods("").List(ctx, v1.ListOptions{
		LabelSelector: labelSelector,
		FieldSelector: fieldSelector,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list pods for selector %q in getGpuAllocations: %w", labelSelector, err)
	}

	usageMap := make(map[string]int64)
	for _, pod := range pods.Items {
		usageMap[pod.Spec.NodeName] += calculatePodGPUs(&pod)
	}

	return usageMap, nil
}

func calculatePodGPUs(pod *corev1.Pod) int64 {
	var total int64
	for _, container := range pod.Spec.Containers {
		if val, ok := container.Resources.Requests[gpuProperty]; ok {
			total += val.Value()
		}
	}
	return total
}

// listPodsWithSelectors lists pods (as unstructured) using label and field selectors with a dynamic client.
func listPodsWithSelectors(
	ctx context.Context,
	client dynamic.Interface,
	labelSelector string,
	fieldSelector string,
) (map[string][]*unstructured.Unstructured, error) {
	gvr := schema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "pods",
	}
	list, err := client.Resource(gvr).List(ctx, v1.ListOptions{
		LabelSelector: labelSelector,
		FieldSelector: fieldSelector,
	})
	if err != nil {
		return nil, err
	}

	podsMap := make(map[string][]*unstructured.Unstructured)
	for i, pod := range list.Items {
		ns := pod.GetNamespace()
		podsMap[ns] = append(podsMap[ns], &list.Items[i])
	}

	return podsMap, nil
}

func processPodQueries[C any, V any](
	ctx context.Context,
	client C,
	labelSelectors []string,
	fieldSelector string,
	podsMapper func(context.Context, C, string, string) (map[string]V, error),
	valueReducer func(string, V),
) {
	eg, egCtx := errgroup.WithContext(ctx)
	type result struct {
		valueMap map[string]V
		selector string
		err      error
	}

	resCh := make(chan result, len(labelSelectors))
	for _, selector := range labelSelectors {
		selCopy := selector
		eg.Go(func() error {
			m, err := podsMapper(egCtx, client, selCopy, fieldSelector)
			if err != nil {
				resCh <- result{err: err, selector: selCopy}
				return nil
			}
			resCh <- result{valueMap: m}
			return nil
		})
	}
	_ = eg.Wait()
	close(resCh)

	logger := logging.FromContext(ctx)
	for r := range resCh {
		if r.err != nil {
			logger.Errorw("failed to load pods", "selector", r.selector, "error", r.err)
			continue
		}

		for key, value := range r.valueMap {
			valueReducer(key, value)
		}
	}
}
