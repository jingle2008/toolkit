package utils

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
)

type mockKubernetesClient struct {
	nodes []corev1.Node
	pods  []corev1.Pod
}

func (m *mockKubernetesClient) CoreV1NodesList(ctx context.Context, opts v1.ListOptions) ([]corev1.Node, error) {
	return m.nodes, nil
}

func (m *mockKubernetesClient) CoreV1PodsList(ctx context.Context, namespace string, opts v1.ListOptions) ([]corev1.Pod, error) {
	return m.pods, nil
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
