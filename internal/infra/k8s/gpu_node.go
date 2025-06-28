package k8s

import (
	"context"
	"fmt"
	"sort"

	"github.com/jingle2008/toolkit/internal/infra/logging"
	models "github.com/jingle2008/toolkit/pkg/models"
	"golang.org/x/sync/errgroup"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// gpuProperty is the Kubernetes resource name for GPU.
const gpuProperty corev1.ResourceName = "nvidia.com/gpu"

// nodeCondGpuUnhealthy is the condition type for unhealthy GPU nodes.
const nodeCondGpuUnhealthy corev1.NodeConditionType = "GpuUnhealthy"

// defaultGPUSelectors is the default set of label selectors used to sum GPU allocations.
var defaultGPUSelectors = []string{
	"app=dummy",
	"component=predictor",
	"ome.oracle.com/trainingjob",
}

// ListGpuNodes returns a list of GpuNode objects from the given kubernetesClient.
// If no selectors are provided, DefaultGPUSelectors is used.
func ListGpuNodes(ctx context.Context, clientset kubernetes.Clientset, selectors ...string) ([]models.GpuNode, error) {
	if len(selectors) == 0 {
		selectors = defaultGPUSelectors
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

	eg, egCtx := errgroup.WithContext(ctx)
	for _, sel := range selectors {
		selCopy := sel
		eg.Go(func() error {
			err := updateGpuAllocations(egCtx, clientset, gpuAllocationMap, selCopy)
			if err != nil {
				logging.FromContext(egCtx).Errorw("updateGpuAllocations failed", "selector", selCopy, "err", err)
			}
			return nil // always return nil so all selectors run, errors are logged
		})
	}
	_ = eg.Wait()

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

func updateGpuAllocations(ctx context.Context, clientset kubernetes.Clientset,
	gpuAllocationMap map[string]int64, label string,
) error {
	pods, err := clientset.CoreV1().Pods("").List(ctx, v1.ListOptions{
		LabelSelector: label,
		FieldSelector: "status.phase=Running",
	})
	if err != nil {
		return fmt.Errorf("failed to list pods for selector %q in updateGpuAllocations: %w", label, err)
	}

	for _, pod := range pods.Items {
		if _, ok := gpuAllocationMap[pod.Spec.NodeName]; ok {
			gpuAllocationMap[pod.Spec.NodeName] += calculatePodGPUs(&pod)
		}
	}

	return nil
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

func LoadGpuNodes(ctx context.Context, clientset *kubernetes.Clientset) (map[string][]models.GpuNode, error) {
	nodes, err := ListGpuNodes(ctx, *clientset)
	if err != nil {
		return nil, err
	}

	result := make(map[string][]models.GpuNode)
	for _, node := range nodes {
		result[node.NodePool] = append(result[node.NodePool], node)
	}

	// sort by free GPUs
	for _, v := range result {
		sort.Slice(v, func(i, j int) bool {
			vi := v[i].Allocatable - v[i].Allocated
			vj := v[j].Allocatable - v[j].Allocated
			return vi > vj
		})
	}

	return result, nil
}
