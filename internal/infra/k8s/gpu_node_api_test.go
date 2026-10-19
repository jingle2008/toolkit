package k8s

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	cgotesting "k8s.io/client-go/testing"
)

func makeNode(name string, labels map[string]string, allocatable int64, unschedulable bool, conditions []corev1.NodeCondition) *corev1.Node {
	return &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: labels,
		},
		Spec: corev1.NodeSpec{
			Unschedulable: unschedulable,
		},
		Status: corev1.NodeStatus{
			Allocatable: corev1.ResourceList{
				gpuProperty: *resource.NewQuantity(allocatable, resource.DecimalSI),
			},
			Conditions: conditions,
		},
	}
}

func makePod(name, nodeName string, gpuReq int64, label string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: map[string]string{label: "dummy"},
		},
		Spec: corev1.PodSpec{
			NodeName: nodeName,
			Containers: []corev1.Container{
				{
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							gpuProperty: *resource.NewQuantity(gpuReq, resource.DecimalSI),
						},
					},
				},
			},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
		},
	}
}

func TestListGPUNodes_HappyPath(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	node := makeNode("n1", map[string]string{
		"nvidia.com/gpu.present":           "true",
		"beta.kubernetes.io/instance-type": "g4dn.xlarge",
		"instance-pool.name":               "pool1",
	}, 4, false, []corev1.NodeCondition{
		{Type: corev1.NodeReady, Status: corev1.ConditionTrue},
	})

	pod := makePod("p1", "n1", 2, servingLabelV1)
	client := fake.NewSimpleClientset(node, pod)

	nodes, err := ListGPUNodes(ctx, client, -1)
	require.NoError(t, err)
	assert.Len(t, nodes, 1)
	assert.Equal(t, "n1", nodes[0].Name)
	assert.Equal(t, 4, nodes[0].Allocatable)
	assert.Equal(t, 2, nodes[0].Allocated)
	assert.True(t, nodes[0].IsHealthy())
	assert.True(t, nodes[0].IsReady)
	assert.False(t, nodes[0].IsSchedulingDisabled)
}

// A pod bound to a node but still Pending (pulling its image) already
// holds its GPUs, so the allocation query must not be limited to Running
// pods, and a pod not yet scheduled must not be charged to any node.
func TestListGPUNodes_CountsScheduledPendingPods(t *testing.T) {
	t.Parallel()
	node := makeNode("n1", map[string]string{"nvidia.com/gpu.present": "true"}, 8, false,
		[]corev1.NodeCondition{{Type: corev1.NodeReady, Status: corev1.ConditionTrue}})
	running := makePod("running", "n1", 4, servingLabelV2)
	pending := makePod("pending", "n1", 4, servingLabelV2)
	pending.Status.Phase = corev1.PodPending
	unscheduled := makePod("unscheduled", "", 2, servingLabelV2)
	unscheduled.Status.Phase = corev1.PodPending
	client := fake.NewSimpleClientset(node, running, pending, unscheduled)

	var allocSelectors []string
	client.PrependReactor("list", "pods", func(a cgotesting.Action) (bool, runtime.Object, error) {
		if r := a.(cgotesting.ListAction).GetListRestrictions(); !r.Labels.Empty() {
			allocSelectors = append(allocSelectors, r.Fields.String())
		}
		return false, nil, nil
	})

	nodes, err := ListGPUNodes(context.Background(), client, -1)
	require.NoError(t, err)
	require.Len(t, nodes, 1)
	assert.Equal(t, 8, nodes[0].Allocated, "the Pending pod's GPUs count; the unscheduled one's don't")
	require.NotEmpty(t, allocSelectors)
	for _, sel := range allocSelectors {
		assert.Equal(t, allocatedPodSelector, sel)
	}
}

// An MI300X node advertises amd.com/gpu and its pods request it: both
// count, or the node reads "Missing GPUs" with every GPU free.
func TestListGPUNodes_AMDGPUs(t *testing.T) {
	t.Parallel()
	const amd corev1.ResourceName = "amd.com/gpu"
	node := makeNode("mi300", map[string]string{
		"nvidia.com/gpu.present":           "true",
		"beta.kubernetes.io/instance-type": "BM.GPU.MI300X.8",
	}, 0, false, []corev1.NodeCondition{{Type: corev1.NodeReady, Status: corev1.ConditionTrue}})
	node.Status.Allocatable = corev1.ResourceList{amd: *resource.NewQuantity(8, resource.DecimalSI)}
	pod := makePod("p", "mi300", 0, servingLabelV2)
	pod.Spec.Containers[0].Resources.Requests = corev1.ResourceList{amd: *resource.NewQuantity(2, resource.DecimalSI)}
	client := fake.NewSimpleClientset(node, pod)

	nodes, err := ListGPUNodes(context.Background(), client, -1)
	require.NoError(t, err)
	require.Len(t, nodes, 1)
	assert.Equal(t, 8, nodes[0].Allocatable)
	assert.Equal(t, 2, nodes[0].Allocated)
	assert.Equal(t, "OK", nodes[0].GetStatus())
}

func TestListGPUNodes_NoGPUNodes(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	client := fake.NewSimpleClientset()
	nodes, err := ListGPUNodes(ctx, client, -1)
	require.NoError(t, err)
	assert.Empty(t, nodes)
}

func TestListGPUNodes_Unschedulable(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	node := makeNode("n2", map[string]string{
		"nvidia.com/gpu.present": "true",
		"instance-pool.name":     "pool2",
	}, 2, true, nil)
	client := fake.NewSimpleClientset(node)
	nodes, err := ListGPUNodes(ctx, client, -1)
	require.NoError(t, err)
	assert.Len(t, nodes, 1)
	assert.True(t, nodes[0].IsSchedulingDisabled)
}

func TestListGPUNodes_ErrorFromNodesList(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	client := fake.NewSimpleClientset()
	// Simulate error by clearing reactors and adding one that returns error
	client.ReactionChain = nil
	client.PrependReactor("list", "nodes", func(_ cgotesting.Action) (bool, runtime.Object, error) {
		return true, nil, assert.AnError
	})
	_, err := ListGPUNodes(ctx, client, -1)
	assert.Error(t, err)
}

func TestListGPUNodes_ErrorFromPodsList(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	node := makeNode("n3", map[string]string{
		"nvidia.com/gpu.present": "true",
		"instance-pool.name":     "pool3",
	}, 1, false, nil)
	client := fake.NewSimpleClientset(node)
	// Simulate error from Pods().List
	client.PrependReactor("list", "pods", func(_ cgotesting.Action) (bool, runtime.Object, error) {
		return true, nil, assert.AnError
	})
	nodes, err := ListGPUNodes(ctx, client, -1)
	require.Error(t, err)
	assert.Nil(t, nodes)
}

func TestLoadGPUNodes_HappyPath(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	node := makeNode("n4", map[string]string{
		"nvidia.com/gpu.present": "true",
		"instance-pool.name":     "pool4",
	}, 8, false, nil)
	client := fake.NewSimpleClientset(node)
	result, err := LoadGPUNodesByPool(ctx, client)
	require.NoError(t, err)
	assert.Contains(t, result, "pool4")
	assert.Equal(t, "n4", result["pool4"][0].Name)
}

func TestLoadGPUNodes_Empty(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	client := fake.NewSimpleClientset()
	result, err := LoadGPUNodesByPool(ctx, client)
	require.NoError(t, err)
	assert.Empty(t, result)
}
