package k8s

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/jingle2008/toolkit/internal/testutil"
	models "github.com/jingle2008/toolkit/pkg/models"
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
	t.Parallel()
	helper, err := NewHelper("", "")
	require.NoError(t, err)
	assert.NotNil(t, helper)
}

func TestNewHelperWithClients(t *testing.T) {
	t.Parallel()
	helper, err := NewHelper("", "")
	require.NoError(t, err)
	assert.NotNil(t, helper)
}

func TestNewHelper_ChangeContextError(t *testing.T) {
	t.Parallel()
	// Create a temp file that is not a valid kubeconfig
	tmp := ""
	f, err := os.CreateTemp("", "badkubeconfig")
	require.NoError(t, err)
	tmp = f.Name()
	_ = f.Close()
	defer func() { _ = os.Remove(tmp) }()

	helper, err := NewHelper(tmp, "nonexistent-context")
	require.Error(t, err)
	assert.Nil(t, helper)
}

func TestListGpuNodesWithSelectors_Error(t *testing.T) {
	t.Parallel()
	cs := testutil.NewFakeClient()
	cs.PrependReactor("*", "*", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("boom")
	})
	helper := &Helper{
		clientsetFunc: func(_ *rest.Config) (kubernetesClient, error) {
			fakeAdapter := testutil.NewFakeKubernetesClientAdapter(cs)
			return fakeAdapter, nil
		},
	}
	helper.config = &rest.Config{}
	_, err := helper.ListGpuNodesWithSelectors(context.Background(), "app=fail")
	assert.Error(t, err)
}

func TestUpdateGpuAllocations(t *testing.T) {
	t.Parallel()
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
						corev1.ResourceName(GPUProperty): *resource.NewQuantity(2, resource.DecimalSI),
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
	fakeAdapter := testutil.NewFakeKubernetesClientAdapter(fakeClient)
	err := updateGpuAllocations(context.Background(), fakeAdapter, allocMap, "app=test")
	require.NoError(t, err)
	assert.Equal(t, int64(2), allocMap["node1"])
}

func TestIsNodeHealthy(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		conds []corev1.NodeCondition
		want  bool
	}{
		{
			name: "healthy GPU",
			conds: []corev1.NodeCondition{
				{Type: NodeCondGpuUnhealthy, Status: corev1.ConditionFalse},
			},
			want: true,
		},
		{
			name: "unhealthy GPU",
			conds: []corev1.NodeCondition{
				{Type: NodeCondGpuUnhealthy, Status: corev1.ConditionTrue},
			},
			want: false,
		},
		{
			name: "no GPU condition",
			conds: []corev1.NodeCondition{
				{Type: corev1.NodeReady, Status: corev1.ConditionTrue},
			},
			want: false,
		},
		{
			name:  "empty conditions",
			conds: nil,
			want:  false,
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, isNodeHealthy(tt.conds))
		})
	}
}

func TestIsNodeReady(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		conds []corev1.NodeCondition
		want  bool
	}{
		{
			name: "ready node",
			conds: []corev1.NodeCondition{
				{Type: corev1.NodeReady, Status: corev1.ConditionTrue},
			},
			want: true,
		},
		{
			name: "not ready node",
			conds: []corev1.NodeCondition{
				{Type: corev1.NodeReady, Status: corev1.ConditionFalse},
			},
			want: false,
		},
		{
			name: "no ready condition",
			conds: []corev1.NodeCondition{
				{Type: NodeCondGpuUnhealthy, Status: corev1.ConditionFalse},
			},
			want: false,
		},
		{
			name:  "empty conditions",
			conds: nil,
			want:  false,
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, isNodeReady(tt.conds))
		})
	}
}

func TestCalculatePodGPUs(t *testing.T) {
	t.Parallel()
	pod := corev1.Pod{
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceName(GPUProperty): *resource.NewQuantity(3, resource.DecimalSI),
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
				corev1.ResourceName(GPUProperty): *resource.NewQuantity(8, resource.DecimalSI),
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
						corev1.ResourceName(GPUProperty): *resource.NewQuantity(6, resource.DecimalSI),
					},
				}},
			},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
		},
	}
	fakeClientset := testutil.NewFakeClient(node, pod)
	helper := &Helper{
		clientsetFunc: func(_ *rest.Config) (kubernetesClient, error) {
			fakeAdapter := testutil.NewFakeKubernetesClientAdapter(fakeClientset)
			return fakeAdapter, nil
		},
		dynamicFunc: nil,
	}
	helper.config = &rest.Config{}

	nodes, err := helper.ListGpuNodesWithSelectors(context.Background(), "app=dummy")
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
	t.Parallel()
	helper := &Helper{
		dynamicFunc: func(_ *rest.Config) (dynamicClient, error) {
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
	clusters, err := helper.ListDedicatedAIClusters(context.Background())
	require.NoError(t, err)
	assert.NotNil(t, clusters)
	assert.GreaterOrEqual(t, len(clusters), 2)
}

func TestTenantIDFromLabels(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		labels map[string]interface{}
		want   string
	}{
		{
			name:   "string tenancy-id",
			labels: map[string]interface{}{"tenancy-id": "tid"},
			want:   "tid",
		},
		{
			name:   "missing tenancy-id",
			labels: map[string]interface{}{},
			want:   "UNKNOWN_TENANCY",
		},
		{
			name:   "non-string tenancy-id",
			labels: map[string]interface{}{"tenancy-id": 123},
			want:   "UNKNOWN_TENANCY",
		},
		{
			name:   "nil labels",
			labels: nil,
			want:   "UNKNOWN_TENANCY",
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, TenantIDFromLabels(tt.labels))
		})
	}
}

// ---- merged from internal/utils/k8s_test.go ----

type mockK8sHelper struct {
	nodes []models.GpuNode
	dacs  []models.DedicatedAICluster
	err   error
}

func (m *mockK8sHelper) ListGpuNodes(_ context.Context) ([]models.GpuNode, error) {
	return m.nodes, m.err
}

func (m *mockK8sHelper) ListDedicatedAIClusters(_ context.Context) ([]models.DedicatedAICluster, error) {
	return m.dacs, m.err
}

func TestLoadGpuNodes_Success(t *testing.T) {
	t.Parallel()
	mock := &mockK8sHelper{
		nodes: []models.GpuNode{
			{Name: "n1", NodePool: "pool1", Allocatable: 4, Allocated: 2},
			{Name: "n2", NodePool: "pool1", Allocatable: 2, Allocated: 0},
		},
	}
	result, err := LoadGpuNodes(context.Background(), mock)
	require.NoError(t, err)
	assert.Contains(t, result, "pool1")
	assert.Len(t, result["pool1"], 2)
}

func TestLoadGpuNodes_Error(t *testing.T) {
	t.Parallel()
	mock := &mockK8sHelper{err: errors.New("fail")}
	_, err := LoadGpuNodes(context.Background(), mock)
	assert.Error(t, err)
}

func TestLoadDedicatedAIClusters_Success(t *testing.T) {
	t.Parallel()
	mock := &mockK8sHelper{
		dacs: []models.DedicatedAICluster{
			{Name: "dac1", TenantID: "tid1"},
			{Name: "dac2", TenantID: "tid1"},
		},
	}
	result, err := LoadDedicatedAIClusters(context.Background(), mock)
	require.NoError(t, err)
	assert.Contains(t, result, "tid1")
	assert.Len(t, result["tid1"], 2)
}

func TestLoadDedicatedAIClusters_Error(t *testing.T) {
	t.Parallel()
	mock := &mockK8sHelper{err: errors.New("fail")}
	_, err := LoadDedicatedAIClusters(context.Background(), mock)
	assert.Error(t, err)
}
