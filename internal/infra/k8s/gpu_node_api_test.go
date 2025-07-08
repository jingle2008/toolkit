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

func TestListGpuNodes_HappyPath(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	node := makeNode("n1", map[string]string{
		"nvidia.com/gpu.present":           "true",
		"beta.kubernetes.io/instance-type": "g4dn.xlarge",
		"instance-pool.name":               "pool1",
	}, 4, false, []corev1.NodeCondition{
		{Type: nodeCondGpuUnhealthy, Status: corev1.ConditionFalse},
		{Type: corev1.NodeReady, Status: corev1.ConditionTrue},
	})

	pod := makePod("p1", "n1", 2, "app")
	client := fake.NewSimpleClientset(node, pod)

	nodes, err := ListGpuNodes(ctx, client)
	require.NoError(t, err)
	assert.Len(t, nodes, 1)
	assert.Equal(t, "n1", nodes[0].Name)
	assert.Equal(t, 4, nodes[0].Allocatable)
	assert.Equal(t, 2, nodes[0].Allocated)
	assert.True(t, nodes[0].IsHealthy)
	assert.True(t, nodes[0].IsReady)
	assert.False(t, nodes[0].IsSchedulingDisabled)
}

func TestListGpuNodes_NoGPUNodes(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	client := fake.NewSimpleClientset()
	nodes, err := ListGpuNodes(ctx, client)
	require.NoError(t, err)
	assert.Empty(t, nodes)
}

func TestListGpuNodes_Unschedulable(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	node := makeNode("n2", map[string]string{
		"nvidia.com/gpu.present": "true",
		"instance-pool.name":     "pool2",
	}, 2, true, nil)
	client := fake.NewSimpleClientset(node)
	nodes, err := ListGpuNodes(ctx, client)
	require.NoError(t, err)
	assert.Len(t, nodes, 1)
	assert.True(t, nodes[0].IsSchedulingDisabled)
}

func TestListGpuNodes_ErrorFromNodesList(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	client := fake.NewSimpleClientset()
	// Simulate error by clearing reactors and adding one that returns error
	client.ReactionChain = nil
	client.PrependReactor("list", "nodes", func(_ cgotesting.Action) (bool, runtime.Object, error) {
		return true, nil, assert.AnError
	})
	_, err := ListGpuNodes(ctx, client)
	assert.Error(t, err)
}

func TestListGpuNodes_ErrorFromPodsList(t *testing.T) {
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
	nodes, err := ListGpuNodes(ctx, client)
	require.Error(t, err)
	assert.Nil(t, nodes)
}

func TestLoadGpuNodes_HappyPath(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	node := makeNode("n4", map[string]string{
		"nvidia.com/gpu.present": "true",
		"instance-pool.name":     "pool4",
	}, 8, false, nil)
	client := fake.NewSimpleClientset(node)
	result, err := LoadGpuNodes(ctx, client)
	require.NoError(t, err)
	assert.Contains(t, result, "pool4")
	assert.Equal(t, "n4", result["pool4"][0].Name)
}

func TestLoadGpuNodes_Empty(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	client := fake.NewSimpleClientset()
	result, err := LoadGpuNodes(ctx, client)
	require.NoError(t, err)
	assert.Empty(t, result)
}
