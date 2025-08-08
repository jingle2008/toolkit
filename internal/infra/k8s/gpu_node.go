package k8s

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	models "github.com/jingle2008/toolkit/pkg/models"
)

// gpuProperty is the Kubernetes resource name for GPU.
const gpuProperty corev1.ResourceName = "nvidia.com/gpu"

const (
	nodeCondGpuBus   corev1.NodeConditionType = "GpuBus"
	nodeCondGpuCount corev1.NodeConditionType = "GpuCount"
)

// ListGpuNodes lists a list of gpu nodes up to the limit.
func ListGpuNodes(ctx context.Context, clientset kubernetes.Interface, limit int) ([]models.GpuNode, error) {
	nodes, err := clientset.CoreV1().Nodes().List(ctx, v1.ListOptions{
		LabelSelector: "nvidia.com/gpu.present=true",
		Limit:         int64(limit),
	})
	if err != nil {
		return nil, err
	}

	// 1. GPU allocation: as before, using processPodQueries (4 label selectors)
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

	// 2. Pod issues: one extra query for all pods not Running/Succeeded
	badPhaseSelector := "status.phase!=Running,status.phase!=Succeeded"
	badPods, err := clientset.CoreV1().Pods("").List(ctx, v1.ListOptions{
		FieldSelector: badPhaseSelector,
	})
	if err != nil {
		return nil, err
	}
	podIssueMap := make(map[string][]string)
	for _, p := range badPods.Items {
		if p.Spec.NodeName == "" {
			continue
		}
		// Defensive: fake clientset does not filter by phase, so skip healthy pods here.
		if p.Status.Phase == corev1.PodRunning || p.Status.Phase == corev1.PodSucceeded {
			continue
		}
		podIssueMap[p.Spec.NodeName] = append(
			podIssueMap[p.Spec.NodeName],
			fmt.Sprintf("pod %s: %s", p.Name, getPodReason(&p)))
	}

	gpuNodes := make([]models.GpuNode, 0, len(nodes.Items))
	for _, node := range nodes.Items {
		allocQty := node.Status.Allocatable[gpuProperty]
		allocatable, _ := allocQty.AsInt64()
		age := FormatAge(time.Since(node.CreationTimestamp.Time))
		issues := getNodeIssues(node.Status.Conditions)
		// Add pod issues for this node
		issues = append(issues, podIssueMap[node.Name]...)
		gpuNodes = append(gpuNodes, models.GpuNode{
			Name:                 node.Name,
			InstanceType:         node.Labels["beta.kubernetes.io/instance-type"],
			NodePool:             node.Labels["instance-pool.name"],
			CompartmentID:        node.Annotations["oci.oraclecloud.com/compartment-id"],
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

func getPodReason(p *corev1.Pod) string {
	if p.Status.Reason != "" {
		return p.Status.Reason
	}
	for _, cs := range p.Status.ContainerStatuses {
		if cs.State.Waiting != nil {
			return cs.State.Waiting.Reason
		}
		if cs.State.Terminated != nil {
			return cs.State.Terminated.Reason
		}
	}
	return "unknown"
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
	nodes, err := ListGpuNodes(ctx, clientset, 0)
	if err != nil {
		return nil, err
	}

	result := make(map[string][]models.GpuNode)
	for _, node := range nodes {
		result[node.NodePool] = append(result[node.NodePool], node)
	}
	return result, nil
}
