package k8s

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	models "github.com/jingle2008/toolkit/pkg/models"
)

// podGPULimits sums nvidia.com/gpu across the pod's containers' limits.
func podGPULimits(pod *corev1.Pod) int {
	var total int64
	for _, c := range pod.Spec.Containers {
		if q, ok := c.Resources.Limits[gpuProperty]; ok {
			total += q.Value()
		}
	}
	return int(total)
}

// LoadGPUWorkloadsByNode lists running pods that consume GPU and groups
// them by spec.nodeName (== GPUNode.Name). A pod qualifies when it limits
// nvidia.com/gpu > 0 and is scheduled to a node.
func LoadGPUWorkloadsByNode(ctx context.Context, clientset kubernetes.Interface) (map[string][]models.GPUWorkload, error) {
	pods, err := clientset.CoreV1().Pods("").List(ctx, v1.ListOptions{
		FieldSelector: runningPodSelector,
	})
	if err != nil {
		return nil, err
	}

	result := make(map[string][]models.GPUWorkload)
	for i := range pods.Items {
		pod := &pods.Items[i]
		gpus := podGPULimits(pod)
		if gpus <= 0 || pod.Spec.NodeName == "" {
			continue
		}
		labels := pod.Labels
		annos := pod.Annotations
		w := models.GPUWorkload{
			Name:      pod.Name,
			Node:      pod.Spec.NodeName,
			TenantID:  labels["tenancy-id"],
			Namespace: pod.Namespace,
			Model:     labels["base-model-name"],
			Runtime:   labels["serving-runtime"],
			GPUs:      gpus,
			Mode:      annos["ome.io/deploymentMode"],
		}
		result[pod.Spec.NodeName] = append(result[pod.Spec.NodeName], w)
	}
	return result, nil
}
