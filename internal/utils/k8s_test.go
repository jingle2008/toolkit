package utils

import (
	"context"
	"errors"
	"testing"

	"github.com/jingle2008/toolkit/pkg/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
	orig := helperFactory
	defer func() { helperFactory = orig }()
	helperFactory = func(_ string, _ string) (gpuHelper, error) {
		return &mockK8sHelper{
			nodes: []models.GpuNode{
				{Name: "n1", NodePool: "pool1", Allocatable: 4, Allocated: 2},
				{Name: "n2", NodePool: "pool1", Allocatable: 2, Allocated: 0},
			},
		}, nil
	}
	env := models.Environment{Realm: "test", Type: "dev", Region: "us-test-1"}
	result, err := LoadGpuNodes(context.Background(), "dummy", env)
	require.NoError(t, err)
	assert.Contains(t, result, "pool1")
	assert.Len(t, result["pool1"], 2)
}

func TestLoadGpuNodes_Error(t *testing.T) {
	t.Parallel()
	orig := helperFactory
	defer func() { helperFactory = orig }()
	helperFactory = func(_ string, _ string) (gpuHelper, error) {
		return &mockK8sHelper{err: errors.New("fail")}, nil
	}
	env := models.Environment{Realm: "test", Type: "dev", Region: "us-test-1"}
	_, err := LoadGpuNodes(context.Background(), "dummy", env)
	assert.Error(t, err)
}

func TestLoadDedicatedAIClusters_Success(t *testing.T) {
	t.Parallel()
	orig := helperFactory
	defer func() { helperFactory = orig }()
	helperFactory = func(_ string, _ string) (gpuHelper, error) {
		return &mockK8sHelper{
			dacs: []models.DedicatedAICluster{
				{Name: "dac1", TenantID: "tid1"},
				{Name: "dac2", TenantID: "tid1"},
			},
		}, nil
	}
	env := models.Environment{Realm: "test", Type: "dev", Region: "us-test-1"}
	result, err := LoadDedicatedAIClusters(context.Background(), "dummy", env)
	require.NoError(t, err)
	assert.Contains(t, result, "tid1")
	assert.Len(t, result["tid1"], 2)
}

func TestLoadDedicatedAIClusters_Error(t *testing.T) {
	t.Parallel()
	orig := helperFactory
	defer func() { helperFactory = orig }()
	helperFactory = func(_ string, _ string) (gpuHelper, error) {
		return &mockK8sHelper{err: errors.New("fail")}, nil
	}
	env := models.Environment{Realm: "test", Type: "dev", Region: "us-test-1"}
	_, err := LoadDedicatedAIClusters(context.Background(), "dummy", env)
	assert.Error(t, err)
}
