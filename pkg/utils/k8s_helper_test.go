package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
)

func TestNewK8sHelper_NilConfig(t *testing.T) {
	helper, err := NewK8sHelper("", "")
	assert.NoError(t, err)
	assert.NotNil(t, helper)
}

func TestNewK8sHelperWithClients(t *testing.T) {
	helper, err := NewK8sHelperWithClients("", "", nil, nil)
	assert.NoError(t, err)
	assert.NotNil(t, helper)
}

func TestListGpuNodesWithSelectors_Error(t *testing.T) {
	helper := &K8sHelper{
		clientsetFunc: func(_ *rest.Config) (KubernetesClient, error) { return &errorKubernetesClient{}, nil },
	}
	helper.config = &rest.Config{}
	_, err := helper.ListGpuNodesWithSelectors("app=fail")
	assert.Error(t, err)
}

func TestUpdateGpuAllocations(t *testing.T) {
	client := &mockK8sClientAlloc{
		pods: []corev1.Pod{
			{
				Spec: corev1.PodSpec{
					NodeName: "node1",
					Containers: []corev1.Container{
						{Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								GPUProperty: *resource.NewQuantity(2, resource.DecimalSI),
							},
						}},
					},
				},
			},
		},
	}
	allocMap := map[string]int64{"node1": 0}
	err := updateGpuAllocations(client, allocMap, "app=test")
	assert.NoError(t, err)
	assert.Equal(t, int64(2), allocMap["node1"])
}

func TestIsNodeHealthy(t *testing.T) {
	conds := []corev1.NodeCondition{
		{Type: corev1.NodeConditionType("GpuUnhealthy"), Status: corev1.ConditionFalse},
	}
	assert.True(t, isNodeHealthy(conds))
	conds[0].Status = corev1.ConditionTrue
	assert.False(t, isNodeHealthy(conds))
}

func TestIsNodeReady(t *testing.T) {
	conds := []corev1.NodeCondition{
		{Type: corev1.NodeReady, Status: corev1.ConditionTrue},
	}
	assert.True(t, isNodeReady(conds))
	conds[0].Status = corev1.ConditionFalse
	assert.False(t, isNodeReady(conds))
}

func TestCalculatePodGPUs(t *testing.T) {
	pod := corev1.Pod{
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						GPUProperty: *resource.NewQuantity(3, resource.DecimalSI),
					},
				}},
			},
		},
	}
	assert.Equal(t, int64(3), calculatePodGPUs(&pod))
}

func TestListGpuNodes_Mock(t *testing.T) {
	node := corev1.Node{
		ObjectMeta: v1.ObjectMeta{
			Name:   "node1",
			Labels: map[string]string{"beta.kubernetes.io/instance-type": "n1", "instance-pool.name": "pool1"},
		},
		Status: corev1.NodeStatus{
			Allocatable: corev1.ResourceList{
				GPUProperty: *resource.NewQuantity(4, resource.DecimalSI),
			},
			Conditions: []corev1.NodeCondition{
				{Type: corev1.NodeConditionType("GpuUnhealthy"), Status: corev1.ConditionFalse},
				{Type: corev1.NodeReady, Status: corev1.ConditionTrue},
			},
		},
	}
	pod := corev1.Pod{
		Spec: corev1.PodSpec{
			NodeName: "node1",
			Containers: []corev1.Container{
				{Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						GPUProperty: *resource.NewQuantity(2, resource.DecimalSI),
					},
				}},
			},
		},
	}
	mockClient := &mockKubernetesClient{
		nodes: []corev1.Node{node},
		pods:  []corev1.Pod{pod},
	}
	helper := &K8sHelper{
		clientsetFunc: func(_ *rest.Config) (KubernetesClient, error) { return mockClient, nil },
		dynamicFunc:   nil,
	}
	helper.config = &rest.Config{} // not used by mock, but must be non-nil

	nodes, err := helper.ListGpuNodesWithSelectors("app=dummy")
	assert.NoError(t, err)
	assert.Len(t, nodes, 1)
	assert.Equal(t, "node1", nodes[0].Name)
	assert.Equal(t, 4, nodes[0].Allocatable)
	assert.Equal(t, 2, nodes[0].Allocated)
	assert.True(t, nodes[0].IsHealthy)
	assert.True(t, nodes[0].IsReady)
	assert.Equal(t, "n1", nodes[0].InstanceType)
	assert.Equal(t, "pool1", nodes[0].NodePool)
}
