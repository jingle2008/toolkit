package k8s

import (
	"context"
	"time"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	models "github.com/jingle2008/toolkit/pkg/models"
)

// gpuWorkloadPageSize bounds each pods List call so the broad
// (label-less) detection filter never pulls the cluster's entire
// running-pod set into one response. Pages are followed via the
// Continue token in LoadGPUWorkloadsByNode.
const gpuWorkloadPageSize = 500

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
			// calculatePodGPUs (requests) is the same accounting the
			// GPUNode "Allocated" sum uses. For nvidia.com/gpu requests
			// equal limits (Kubernetes forbids overcommitting extended
			// resources), so this matches the spec's "limits" intent while
			// keeping a single source of truth. Only Spec.Containers count
			// (not init/sidecars) — GPU workloads request the device on
			// their main container.
			gpus := int(calculatePodGPUs(pod))
			if gpus <= 0 || pod.Spec.NodeName == "" {
				continue
			}
			labels := pod.Labels
			annos := pod.Annotations
			// Only ContainerStatuses: init containers run once at startup,
			// so their restarts don't signal an ongoing serving failure.
			var restarts int
			for _, cs := range pod.Status.ContainerStatuses {
				restarts += int(cs.RestartCount)
			}
			age := ""
			if ts := pod.CreationTimestamp; !ts.IsZero() {
				age = FormatAge(time.Since(ts.Time))
			}
			w := models.GPUWorkload{
				Name:      pod.Name,
				Node:      pod.Spec.NodeName,
				TenantID:  labels["tenancy-id"],
				Namespace: pod.Namespace,
				Model:     labels["base-model-name"],
				Runtime:   labels["serving-runtime"],
				GPUs:      gpus,
				Restarts:  restarts,
				Age:       age,
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
