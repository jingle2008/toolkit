package utils

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/jingle2008/toolkit/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/rest"
	k8stesting "k8s.io/client-go/testing"
)

func TestNewK8sHelper_NilConfig(t *testing.T) {
	helper, err := NewK8sHelper("", "")
	require.NoError(t, err)
	assert.NotNil(t, helper)
}

func TestNewK8sHelperWithClients(t *testing.T) {
	helper, err := NewK8sHelper("", "")
	require.NoError(t, err)
	assert.NotNil(t, helper)
}

func TestNewK8sHelper_ChangeContextError(t *testing.T) {
	// Create a temp file that is not a valid kubeconfig
	tmp := ""
	f, err := os.CreateTemp("", "badkubeconfig")
	require.NoError(t, err)
	tmp = f.Name()
	_ = f.Close()
	defer func() { _ = os.Remove(tmp) }()

	helper, err := NewK8sHelper(tmp, "nonexistent-context")
	require.Error(t, err)
	assert.Nil(t, helper)
}

func TestListGpuNodesWithSelectors_Error(t *testing.T) {
	cs := testutil.NewFakeClient()
	cs.PrependReactor("*", "*", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("boom")
	})
	helper := &K8sHelper{
		clientsetFunc: func(_ *rest.Config) (KubernetesClient, error) {
			return &testutil.FakeKubernetesClientAdapter{Clientset: cs}, nil
		},
	}
	helper.config = &rest.Config{}
	_, err := helper.ListGpuNodesWithSelectors("app=fail")
	assert.Error(t, err)
}

func TestUpdateGpuAllocations(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: v1.ObjectMeta{
			Name:      "pod1",
			Namespace: "default",
			Labels: map[string]string{
				"app": "test",
			},
		},
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
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
		},
	}
	fakeClient := testutil.NewFakeClient(pod)
	allocMap := map[string]int64{"node1": 0}
	err := updateGpuAllocations(&testutil.FakeKubernetesClientAdapter{Clientset: fakeClient}, allocMap, "app=test")
	require.NoError(t, err)
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

func TestListGpuNodes_FakeClient(t *testing.T) {
	t.Parallel()
	node := &corev1.Node{
		ObjectMeta: v1.ObjectMeta{
			Name: "node2",
			Labels: map[string]string{
				"beta.kubernetes.io/instance-type": "n2",
				"instance-pool.name":               "pool2",
				"nvidia.com/gpu.present":           "true",
			},
		},
		Status: corev1.NodeStatus{
			Allocatable: corev1.ResourceList{
				GPUProperty: *resource.NewQuantity(8, resource.DecimalSI),
			},
			Conditions: []corev1.NodeCondition{
				{Type: corev1.NodeConditionType("GpuUnhealthy"), Status: corev1.ConditionFalse},
				{Type: corev1.NodeReady, Status: corev1.ConditionTrue},
			},
		},
	}
	pod := &corev1.Pod{
		ObjectMeta: v1.ObjectMeta{
			Name:      "pod2",
			Namespace: "default",
			Labels: map[string]string{
				"app": "dummy",
			},
		},
		Spec: corev1.PodSpec{
			NodeName: "node2",
			Containers: []corev1.Container{
				{Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						GPUProperty: *resource.NewQuantity(6, resource.DecimalSI),
					},
				}},
			},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
		},
	}
	fakeClientset := testutil.NewFakeClient(node, pod)
	helper := &K8sHelper{
		clientsetFunc: func(_ *rest.Config) (KubernetesClient, error) {
			return &testutil.FakeKubernetesClientAdapter{Clientset: fakeClientset}, nil
		},
		dynamicFunc: nil,
	}
	helper.config = &rest.Config{}

	nodes, err := helper.ListGpuNodesWithSelectors("app=dummy")
	require.NoError(t, err)
	assert.Len(t, nodes, 1)
	assert.Equal(t, "node2", nodes[0].Name)
	assert.Equal(t, 8, nodes[0].Allocatable)
	assert.Equal(t, 6, nodes[0].Allocated)
	assert.True(t, nodes[0].IsHealthy)
	assert.True(t, nodes[0].IsReady)
	assert.Equal(t, "n2", nodes[0].InstanceType)
	assert.Equal(t, "pool2", nodes[0].NodePool)
}

// ---- merged from k8s_helper_dynamic_test.go ----

type mockDynamicClient struct {
	lists map[string]*unstructured.UnstructuredList
}

func (m *mockDynamicClient) ResourceList(_ context.Context, gvr schema.GroupVersionResource, _ v1.ListOptions) (*unstructured.UnstructuredList, error) {
	key := gvr.Group + "/" + gvr.Version
	return m.lists[key], nil
}

func TestListDedicatedAIClusters(t *testing.T) {
	helper := &K8sHelper{
		dynamicFunc: func(_ *rest.Config) (DynamicClient, error) {
			return &mockDynamicClient{
				lists: map[string]*unstructured.UnstructuredList{
					"ome.oracle.com/v1alpha1": {
						Items: []unstructured.Unstructured{
							{
								Object: map[string]interface{}{
									"metadata": map[string]interface{}{
										"name": "dac1",
										"labels": map[string]interface{}{
											"tenancy-id": "tid1",
										},
									},
									"spec": map[string]interface{}{
										"type":      "t1",
										"unitShape": "shape1",
										"size":      int64(2),
									},
									"status": map[string]interface{}{
										"status": "active",
									},
								},
							},
						},
					},
					"ome.io/v1beta1": {
						Items: []unstructured.Unstructured{
							{
								Object: map[string]interface{}{
									"metadata": map[string]interface{}{
										"name": "dac2",
										"labels": map[string]interface{}{
											"tenancy-id": "tid2",
										},
									},
									"spec": map[string]interface{}{
										"profile": "p1",
										"count":   int64(3),
									},
									"status": map[string]interface{}{
										"dacLifecycleState": "ready",
									},
								},
							},
						},
					},
				},
			}, nil
		},
	}
	helper.config = nil
	clusters, err := helper.ListDedicatedAIClusters()
	require.NoError(t, err)
	assert.NotNil(t, clusters)
	assert.GreaterOrEqual(t, len(clusters), 2)
}

func TestTenantIDFromLabels(t *testing.T) {
	assert.Equal(t, "tid", tenantIDFromLabels(map[string]interface{}{"tenancy-id": "tid"}))
	assert.Equal(t, "UNKNOWN_TENANCY", tenantIDFromLabels(map[string]interface{}{}))
	assert.Equal(t, "UNKNOWN_TENANCY", tenantIDFromLabels(map[string]interface{}{"tenancy-id": 123}))
}
