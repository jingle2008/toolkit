//nolint:paralleltest // tests use global state
package actions

import (
	"context"
	"errors"
	"testing"

	"github.com/oracle/oci-go-sdk/v65/core"
	"github.com/stretchr/testify/require"

	"github.com/jingle2008/toolkit/pkg/infra/logging"
	"github.com/jingle2008/toolkit/pkg/models"
)

type testLogger struct {
	Infos  []string
	Errors []string
}

func (f *testLogger) Infow(msg string, _ ...any)       { f.Infos = append(f.Infos, msg) }
func (f *testLogger) Errorw(msg string, _ ...any)      { f.Errors = append(f.Errors, msg) }
func (f *testLogger) Debugw(string, ...any)            {}
func (f *testLogger) Sync() error                      { return nil }
func (f *testLogger) WithFields(...any) logging.Logger { return f }
func (f *testLogger) DebugEnabled() bool               { return false }

type fakeComputeClient struct {
	core.ComputeClient
	InstanceActionFunc    func(ctx context.Context, req core.InstanceActionRequest) (core.InstanceActionResponse, error)
	TerminateInstanceFunc func(ctx context.Context, req core.TerminateInstanceRequest) (core.TerminateInstanceResponse, error)
}

func (f *fakeComputeClient) InstanceAction(ctx context.Context, req core.InstanceActionRequest) (core.InstanceActionResponse, error) {
	return f.InstanceActionFunc(ctx, req)
}

func (f *fakeComputeClient) TerminateInstance(ctx context.Context, req core.TerminateInstanceRequest) (core.TerminateInstanceResponse, error) {
	return f.TerminateInstanceFunc(ctx, req)
}

type fakeMgmtClient struct {
	core.ComputeManagementClient
	UpdateInstancePoolFunc func(ctx context.Context, req core.UpdateInstancePoolRequest) (core.UpdateInstancePoolResponse, error)
	ListInstancePoolsFunc  func(ctx context.Context, req core.ListInstancePoolsRequest) (core.ListInstancePoolsResponse, error)
}

func (f *fakeMgmtClient) UpdateInstancePool(ctx context.Context, req core.UpdateInstancePoolRequest) (core.UpdateInstancePoolResponse, error) {
	return f.UpdateInstancePoolFunc(ctx, req)
}

func (f *fakeMgmtClient) ListInstancePools(ctx context.Context, req core.ListInstancePoolsRequest) (core.ListInstancePoolsResponse, error) {
	return f.ListInstancePoolsFunc(ctx, req)
}

func makeEnv() models.Environment {
	return models.Environment{Realm: "oc1", Region: "us-phoenix-1"}
}

func TestSoftResetInstance_Success(t *testing.T) {
	orig := newComputeClient
	defer func() { newComputeClient = orig }()
	newComputeClient = func(_ models.Environment) (computeClient, error) {
		return &fakeComputeClient{
			InstanceActionFunc: func(_ context.Context, _ core.InstanceActionRequest) (core.InstanceActionResponse, error) {
				return core.InstanceActionResponse{OpcRequestId: strPtr("reqid")}, nil
			},
		}, nil
	}
	node := &models.GpuNode{ID: "id1", Name: "n1"}
	logger := &testLogger{}
	err := SoftResetInstance(context.Background(), node, makeEnv(), logger)
	require.NoError(t, err)
	require.Contains(t, logger.Infos[0], "rebooting node")
	require.Contains(t, logger.Infos[1], "reboot request is submitted successfully")
}

func TestSoftResetInstance_Error(t *testing.T) {
	orig := newComputeClient
	defer func() { newComputeClient = orig }()
	newComputeClient = func(_ models.Environment) (computeClient, error) {
		return &fakeComputeClient{
			InstanceActionFunc: func(_ context.Context, _ core.InstanceActionRequest) (core.InstanceActionResponse, error) {
				return core.InstanceActionResponse{OpcRequestId: strPtr("reqid")}, errors.New("fail")
			},
		}, nil
	}
	node := &models.GpuNode{ID: "id1", Name: "n1"}
	logger := &testLogger{}
	err := SoftResetInstance(context.Background(), node, makeEnv(), logger)
	require.Error(t, err)
	require.Contains(t, err.Error(), "fail")
	require.Contains(t, err.Error(), "reqid")
}

func TestIncreasePoolSize_Success(t *testing.T) {
	orig := newComputeMgmtClient
	defer func() { newComputeMgmtClient = orig }()
	newComputeMgmtClient = func(_ models.Environment) (computeMgmtClient, error) {
		return &fakeMgmtClient{
			UpdateInstancePoolFunc: func(_ context.Context, _ core.UpdateInstancePoolRequest) (core.UpdateInstancePoolResponse, error) {
				return core.UpdateInstancePoolResponse{OpcRequestId: strPtr("reqid")}, nil
			},
		}, nil
	}
	pool := &models.GpuPool{ID: "pid", Name: "pname", ActualSize: 2}
	logger := &testLogger{}
	err := IncreasePoolSize(context.Background(), pool, makeEnv(), logger)
	require.NoError(t, err)
	require.Contains(t, logger.Infos[0], "scaling up instance pool")
	require.Contains(t, logger.Infos[1], "scaling request is submitted successfully")
}

func TestIncreasePoolSize_Error(t *testing.T) {
	orig := newComputeMgmtClient
	defer func() { newComputeMgmtClient = orig }()
	newComputeMgmtClient = func(_ models.Environment) (computeMgmtClient, error) {
		return &fakeMgmtClient{
			UpdateInstancePoolFunc: func(_ context.Context, _ core.UpdateInstancePoolRequest) (core.UpdateInstancePoolResponse, error) {
				return core.UpdateInstancePoolResponse{OpcRequestId: strPtr("reqid")}, errors.New("fail")
			},
		}, nil
	}
	pool := &models.GpuPool{ID: "pid", Name: "pname", ActualSize: 2}
	logger := &testLogger{}
	err := IncreasePoolSize(context.Background(), pool, makeEnv(), logger)
	require.Error(t, err)
	require.Contains(t, err.Error(), "fail")
	require.Contains(t, err.Error(), "reqid")
}

func TestTerminateInstance_Success(t *testing.T) {
	orig := newComputeClient
	defer func() { newComputeClient = orig }()
	newComputeClient = func(_ models.Environment) (computeClient, error) {
		return &fakeComputeClient{
			TerminateInstanceFunc: func(_ context.Context, _ core.TerminateInstanceRequest) (core.TerminateInstanceResponse, error) {
				return core.TerminateInstanceResponse{OpcRequestId: strPtr("reqid")}, nil
			},
		}, nil
	}
	node := &models.GpuNode{ID: "id1", Name: "n1"}
	logger := &testLogger{}
	err := TerminateInstance(context.Background(), node, makeEnv(), logger)
	require.NoError(t, err)
	require.Contains(t, logger.Infos[0], "deleting node")
	require.Contains(t, logger.Infos[1], "deletion request is submitted successfully")
}

func TestTerminateInstance_Error(t *testing.T) {
	orig := newComputeClient
	defer func() { newComputeClient = orig }()
	newComputeClient = func(_ models.Environment) (computeClient, error) {
		return &fakeComputeClient{
			TerminateInstanceFunc: func(_ context.Context, _ core.TerminateInstanceRequest) (core.TerminateInstanceResponse, error) {
				return core.TerminateInstanceResponse{OpcRequestId: strPtr("reqid")}, errors.New("fail")
			},
		}, nil
	}
	node := &models.GpuNode{ID: "id1", Name: "n1"}
	logger := &testLogger{}
	err := TerminateInstance(context.Background(), node, makeEnv(), logger)
	require.Error(t, err)
	require.Contains(t, err.Error(), "fail")
	require.Contains(t, err.Error(), "reqid")
}

func TestPopulateGpuPools(t *testing.T) {
	orig := newComputeMgmtClient
	defer func() { newComputeMgmtClient = orig }()
	// Case: empty slice or empty compartmentID
	err := PopulateGpuPools(context.Background(), nil, makeEnv(), "comp")
	require.NoError(t, err)
	err = PopulateGpuPools(context.Background(), []models.GpuPool{{}}, makeEnv(), "")
	require.NoError(t, err)

	// Case: pool found
	newComputeMgmtClient = func(_ models.Environment) (computeMgmtClient, error) {
		return &fakeMgmtClient{
			ListInstancePoolsFunc: func(_ context.Context, _ core.ListInstancePoolsRequest) (core.ListInstancePoolsResponse, error) {
				return core.ListInstancePoolsResponse{
					Items: []core.InstancePoolSummary{
						{
							DisplayName:    strPtr("p1"),
							Id:             strPtr("id1"),
							Size:           intPtr(3),
							LifecycleState: "RUNNING",
						},
					},
				}, nil
			},
		}, nil
	}
	pools := []models.GpuPool{{Name: "p1"}}
	err = PopulateGpuPools(context.Background(), pools, makeEnv(), "comp")
	require.NoError(t, err)
	require.Equal(t, "id1", pools[0].ID)
	require.Equal(t, 3, pools[0].ActualSize)
	require.Equal(t, "RUNNING", pools[0].Status)

	// Case: pool not found
	pools = []models.GpuPool{{Name: "p2"}}
	newComputeMgmtClient = func(_ models.Environment) (computeMgmtClient, error) {
		return &fakeMgmtClient{
			ListInstancePoolsFunc: func(_ context.Context, _ core.ListInstancePoolsRequest) (core.ListInstancePoolsResponse, error) {
				return core.ListInstancePoolsResponse{
					Items: []core.InstancePoolSummary{},
				}, nil
			},
		}, nil
	}
	err = PopulateGpuPools(context.Background(), pools, makeEnv(), "comp")
	require.NoError(t, err)
	require.Equal(t, "NONEXIST", pools[0].Status)

	// Case: error from ListInstancePools
	newComputeMgmtClient = func(_ models.Environment) (computeMgmtClient, error) {
		return &fakeMgmtClient{
			ListInstancePoolsFunc: func(_ context.Context, _ core.ListInstancePoolsRequest) (core.ListInstancePoolsResponse, error) {
				return core.ListInstancePoolsResponse{OpcRequestId: strPtr("reqid")}, errors.New("fail")
			},
		}, nil
	}
	pools = []models.GpuPool{{Name: "p3"}}
	err = PopulateGpuPools(context.Background(), pools, makeEnv(), "comp")
	require.Error(t, err)
	require.Contains(t, err.Error(), "fail")
	require.Contains(t, err.Error(), "reqid")
}

func strPtr(s string) *string { return &s }
func intPtr(i int) *int       { return &i }
