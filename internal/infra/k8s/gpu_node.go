package k8s

import (
	"cmp"
	"context"
	"slices"

	models "github.com/jingle2008/toolkit/pkg/models"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// gpuProperty is the Kubernetes resource name for GPU.
const gpuProperty corev1.ResourceName = "nvidia.com/gpu"

// nodeCondGpuUnhealthy is the condition type for unhealthy GPU nodes.
const nodeCondGpuUnhealthy corev1.NodeConditionType = "GpuUnhealthy"

// ListGpuNodes returns a list of GpuNode objects from the given kubernetesClient.
// If no selectors are provided, DefaultGPUSelectors is used.
func ListGpuNodes(ctx context.Context, clientset kubernetes.Interface, selectors ...string) ([]models.GpuNode, error) {
	if len(selectors) == 0 {
		selectors = gpuPodSelectors
	}

	nodes, err := clientset.CoreV1().Nodes().List(ctx, v1.ListOptions{
		LabelSelector: "nvidia.com/gpu.present=true",
	})
	if err != nil {
		return nil, err
	}

	gpuAllocationMap := make(map[string]int64)
	for _, node := range nodes.Items {
		gpuAllocationMap[node.Name] = 0
	}

	err = processPodQueries(ctx, clientset, selectors, runningPodSelector,
		getGpuAllocations,
		func(node string, usage int64) {
			gpuAllocationMap[node] += usage
		})
	if err != nil {
		return nil, err
	}

	gpuNodes := make([]models.GpuNode, 0, len(nodes.Items))
	for _, node := range nodes.Items {
		allocQty := node.Status.Allocatable[gpuProperty]
		allocatable, _ := allocQty.AsInt64()
		gpuNodes = append(gpuNodes, models.GpuNode{
			Name:                 node.Name,
			InstanceType:         node.Labels["beta.kubernetes.io/instance-type"],
			NodePool:             node.Labels["instance-pool.name"],
			Allocatable:          int(allocatable),
			Allocated:            int(gpuAllocationMap[node.Name]),
			IsHealthy:            isNodeHealthy(node.Status.Conditions),
			IsReady:              isNodeReady(node.Status.Conditions),
			IsSchedulingDisabled: node.Spec.Unschedulable,
		})
	}

	return gpuNodes, nil
}

func isNodeHealthy(conditions []corev1.NodeCondition) bool {
	for _, condition := range conditions {
		if condition.Type == nodeCondGpuUnhealthy {
			return condition.Status == corev1.ConditionFalse
		}
	}
	return false
}

func isNodeReady(conditions []corev1.NodeCondition) bool {
	for _, condition := range conditions {
		if condition.Type == corev1.NodeReady {
			return condition.Status == corev1.ConditionTrue
		}
	}
	return false
}

/*
LoadGpuNodes returns a map of node pool names to slices of GpuNode, sorted by free GPUs descending.
It fetches all GPU nodes and groups them by their node pool label.
*/
func LoadGpuNodes(ctx context.Context, clientset kubernetes.Interface) (map[string][]models.GpuNode, error) {
	nodes, err := ListGpuNodes(ctx, clientset)
	if err != nil {
		return nil, err
	}

	result := make(map[string][]models.GpuNode)
	for _, node := range nodes {
		result[node.NodePool] = append(result[node.NodePool], node)
	}

	// sort by free GPUs
	for _, v := range result {
		slices.SortFunc(v, func(a, b models.GpuNode) int {
			ai := a.Allocatable - a.Allocated
			bi := b.Allocatable - b.Allocated
			return cmp.Compare(bi, ai)
		})
	}

	return result, nil
}
