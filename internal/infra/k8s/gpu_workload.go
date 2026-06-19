package k8s

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	models "github.com/jingle2008/toolkit/pkg/models"
)

// gpuWorkloadPageSize bounds each pods List call so the broad
// (label-less) detection filter never pulls the cluster's entire
// running-pod set into one response. Pages are followed via the
// Continue token in LoadGPUWorkloadsByNode.
const gpuWorkloadPageSize = 500

// podGPULimits sums nvidia.com/gpu across the pod's containers' limits.
// Only Spec.Containers are considered (not InitContainers / sidecars),
// matching calculatePodGPUs: GPU serving/training/reservation workloads
// request the device on their main container. Limits (not requests) per
// spec; for nvidia.com/gpu the two are equal — Kubernetes forbids
// overcommitting extended resources — so this agrees with the
// requests-based node "Allocated" sum.
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
//
// Detection is intentionally broad (any GPU-consuming pod, not just
// serving workloads), so it cannot pre-narrow with a label selector the
// way the GPUNode allocation path does. To keep that broad scan bounded
// on large clusters, pods are listed in pages of gpuWorkloadPageSize and
// accumulated across the Continue token rather than in a single response.
func LoadGPUWorkloadsByNode(ctx context.Context, clientset kubernetes.Interface) (map[string][]models.GPUWorkload, error) {
	result := make(map[string][]models.GPUWorkload)
	cont := ""
	for {
		page, err := clientset.CoreV1().Pods("").List(ctx, v1.ListOptions{
			FieldSelector: runningPodSelector,
			Limit:         gpuWorkloadPageSize,
			Continue:      cont,
		})
		if err != nil {
			return nil, err
		}
		for i := range page.Items {
			pod := &page.Items[i]
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
		cont = page.Continue
		if cont == "" {
			break
		}
	}
	return result, nil
}
