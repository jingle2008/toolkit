package k8s

import (
	"context"
	"time"

	models "github.com/jingle2008/toolkit/pkg/models"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// gpuProperty is the Kubernetes resource name for GPU.
const gpuProperty corev1.ResourceName = "nvidia.com/gpu"

const (
	nodeCondGpuBus   corev1.NodeConditionType = "GpuBus"
	nodeCondGpuCount corev1.NodeConditionType = "GpuCount"
)

func listGpuNodes(ctx context.Context, clientset kubernetes.Interface) ([]models.GpuNode, error) {
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

	err = processPodQueries(ctx, clientset, gpuPodSelectors, runningPodSelector,
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
		age := FormatAge(time.Since(node.CreationTimestamp.Time))
		issues := getNodeIssues(node.Status.Conditions)
		gpuNodes = append(gpuNodes, models.GpuNode{
			Name:                 node.Name,
			InstanceType:         node.Labels["beta.kubernetes.io/instance-type"],
			NodePool:             node.Labels["instance-pool.name"],
			CompartmentId:        node.Annotations["oci.oraclecloud.com/compartment-id"],
			ID:                   node.Spec.ProviderID,
			Allocatable:          int(allocatable),
			Allocated:            int(gpuAllocationMap[node.Name]),
			IsReady:              isNodeReady(node.Status.Conditions),
			IsSchedulingDisabled: node.Spec.Unschedulable,
			Age:                  age,
			Issues:               issues,
		})
	}

	return gpuNodes, nil
}

// getNodeIssues returns a list of messages for alarming node conditions.
func getNodeIssues(conditions []corev1.NodeCondition) []string {
	issues := make([]string, 0)
	for _, c := range conditions {
		switch c.Type {
		case corev1.NodeMemoryPressure,
			corev1.NodeDiskPressure,
			corev1.NodePIDPressure,
			corev1.NodeNetworkUnavailable,
			nodeCondGpuBus,
			nodeCondGpuCount:
			if c.Status == corev1.ConditionTrue {
				issues = append(issues, c.Message)
			}
		case corev1.NodeReady:
			// exhaustive check
		}
	}
	return issues
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
LoadGpuNodes returns a map of node pool names to slices of GpuNode.
It fetches all GPU nodes and groups them by their node pool label.
*/
func LoadGpuNodes(ctx context.Context, clientset kubernetes.Interface) (map[string][]models.GpuNode, error) {
	nodes, err := listGpuNodes(ctx, clientset)
	if err != nil {
		return nil, err
	}

	result := make(map[string][]models.GpuNode)
	for _, node := range nodes {
		result[node.NodePool] = append(result[node.NodePool], node)
	}
	return result, nil
}
