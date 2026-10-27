package k8s

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/jingle2008/toolkit/internal/logging"
	models "github.com/jingle2008/toolkit/pkg/models"
)

// gpuProperty is the Kubernetes resource name for an NVIDIA GPU.
const gpuProperty corev1.ResourceName = "nvidia.com/gpu"

// gpuResources are the extended resources a GPU is advertised and
// requested as: NVIDIA's, and AMD's on the MI300X shapes. A node or pod
// uses one vendor's, so summing them gives its GPU count.
var gpuResources = []corev1.ResourceName{gpuProperty, "amd.com/gpu"}

// gpuCount sums the GPU resources in list.
func gpuCount(list corev1.ResourceList) int64 {
	var total int64
	for _, name := range gpuResources {
		if q, ok := list[name]; ok {
			total += q.Value()
		}
	}
	return total
}

// ListGPUNodes lists a list of gpu nodes up to the limit.
func ListGPUNodes(ctx context.Context, clientset kubernetes.Interface, limit int) ([]models.GPUNode, error) {
	nodes, err := clientset.CoreV1().Nodes().List(ctx, v1.ListOptions{
		LabelSelector: "nvidia.com/gpu.present=true",
		Limit:         int64(limit),
	})
	if err != nil {
		return nil, err
	}

	// 1. GPU allocation, using processPodQueries (4 label selectors). It
	// counts scheduled Pending pods too, as the scheduler does: counting
	// only Running ones showed a node free while a pod starting on it
	// already held its GPUs.
	gpuAllocationMap := make(map[string]int64)
	for _, node := range nodes.Items {
		gpuAllocationMap[node.Name] = 0
	}
	err = processPodQueries(ctx, clientset, gpuPodSelectors, allocatedPodSelector,
		getGPUAllocations,
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
			fmt.Sprintf("pod %s: %s", p.Name, getPodReason(&p)),
		)
	}

	gpuNodes := make([]models.GPUNode, 0, len(nodes.Items))
	for _, node := range nodes.Items {
		allocatable := gpuCount(node.Status.Allocatable)
		issues := getNodeIssues(node.Status.Conditions)
		// Add pod issues for this node
		issues = append(issues, podIssueMap[node.Name]...)
		gpuNodes = append(gpuNodes, models.GPUNode{
			Name:                 node.Name,
			InstanceType:         node.Labels["beta.kubernetes.io/instance-type"],
			NodePool:             node.Labels["instance-pool.name"],
			CompartmentID:        node.Annotations["oci.oraclecloud.com/compartment-id"],
			ID:                   node.Spec.ProviderID,
			Allocatable:          int(allocatable),
			Allocated:            int(gpuAllocationMap[node.Name]),
			IsReady:              isNodeReady(node.Status.Conditions),
			IsSchedulingDisabled: node.Spec.Unschedulable,
			Age:                  models.AgeOf(node.CreationTimestamp.Time),
			CreatedAt:            node.CreationTimestamp.Time,
			Issues:               issues,
		})
	}
	return gpuNodes, nil
}

/*
getNodeIssues returns a message for each alarming node condition.

Every condition but Ready follows the node-problem-detector convention —
True means the problem is present — and that holds for the kubelet's own
pressure conditions as well as the OCI health checks (GpuBus, GpuCount,
GpuEcc, GpuRowRemap, RdmaLink, KernelDeadlock, ReadonlyFilesystem, ...).
So any True condition other than Ready is an issue, rather than a fixed
list of types: a list misses checks added later, and once matched on
"GPUBus" while nodes report "GpuBus", so GPU faults never showed.
Unknown (e.g. a check that doesn't run on this shape) is not an issue.
*/
func getNodeIssues(conditions []corev1.NodeCondition) []string {
	issues := make([]string, 0)
	for _, c := range conditions {
		if c.Type == corev1.NodeReady || c.Status != corev1.ConditionTrue {
			continue
		}
		msg := c.Message
		if msg == "" {
			msg = string(c.Type)
		}
		issues = append(issues, msg)
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
LoadGPUNodesByPool returns a map of node pool names to slices of GPUNode.
It fetches all GPU nodes and groups them by their node pool label.
*/
func LoadGPUNodesByPool(ctx context.Context, clientset kubernetes.Interface) (map[string][]models.GPUNode, error) {
	nodes, err := ListGPUNodes(ctx, clientset, 0)
	if err != nil {
		return nil, err
	}

	result := make(map[string][]models.GPUNode)
	for _, node := range nodes {
		result[node.NodePool] = append(result[node.NodePool], node)
	}
	logging.FromContext(ctx).Debug("loaded gpu nodes",
		"nodes", len(nodes), "pools", len(result))
	return result, nil
}
