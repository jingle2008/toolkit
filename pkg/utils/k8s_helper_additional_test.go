package utils

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
)

type errorKubernetesClient struct{}

func (e *errorKubernetesClient) CoreV1NodesList(ctx context.Context, opts v1.ListOptions) ([]corev1.Node, error) {
	return nil, errors.New("fail nodes")
}

func (e *errorKubernetesClient) CoreV1PodsList(ctx context.Context, namespace string, opts v1.ListOptions) ([]corev1.Pod, error) {
	return nil, errors.New("fail pods")
}

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

type mockK8sClientAlloc struct {
	pods []corev1.Pod
}

func (m *mockK8sClientAlloc) CoreV1NodesList(ctx context.Context, opts v1.ListOptions) ([]corev1.Node, error) {
	return nil, nil
}

func (m *mockK8sClientAlloc) CoreV1PodsList(ctx context.Context, namespace string, opts v1.ListOptions) ([]corev1.Pod, error) {
	return m.pods, nil
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
