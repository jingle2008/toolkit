package utils

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/rest"
)

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
