//nolint:paralleltest // tests use global state
package oci

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"testing"

	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jingle2008/toolkit/pkg/models"
)

// testLogger is a slog.Handler capturing info and error messages; log
// through slog.New(logger). It is safe for concurrent use.
type testLogger struct {
	mu     sync.Mutex
	Infos  []string
	Errors []string
}

func (f *testLogger) Enabled(context.Context, slog.Level) bool { return true }

func (f *testLogger) Handle(_ context.Context, r slog.Record) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	switch r.Level {
	case slog.LevelInfo:
		f.Infos = append(f.Infos, r.Message)
	case slog.LevelError:
		f.Errors = append(f.Errors, r.Message)
	}
	return nil
}

func (f *testLogger) WithAttrs([]slog.Attr) slog.Handler { return f }
func (f *testLogger) WithGroup(string) slog.Handler      { return f }

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
	GetInstancePoolFunc    func(ctx context.Context, req core.GetInstancePoolRequest) (core.GetInstancePoolResponse, error)
	UpdateInstancePoolFunc func(ctx context.Context, req core.UpdateInstancePoolRequest) (core.UpdateInstancePoolResponse, error)
	ListInstancePoolsFunc  func(ctx context.Context, req core.ListInstancePoolsRequest) (core.ListInstancePoolsResponse, error)
}

func (f *fakeMgmtClient) GetInstancePool(ctx context.Context, req core.GetInstancePoolRequest) (core.GetInstancePoolResponse, error) {
	return f.GetInstancePoolFunc(ctx, req)
}

func (f *fakeMgmtClient) UpdateInstancePool(ctx context.Context, req core.UpdateInstancePoolRequest) (core.UpdateInstancePoolResponse, error) {
	return f.UpdateInstancePoolFunc(ctx, req)
}

func (f *fakeMgmtClient) ListInstancePools(ctx context.Context, req core.ListInstancePoolsRequest) (core.ListInstancePoolsResponse, error) {
	return f.ListInstancePoolsFunc(ctx, req)
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
	node := &models.GPUNode{ID: "id1", Name: "n1"}
	logger := &testLogger{}
	err := SoftResetInstance(context.Background(), node, makeEnv(), slog.New(logger))
	require.NoError(t, err)
	require.Contains(t, logger.Infos[0], "restarting node")
	require.Contains(t, logger.Infos[1], "restart request is submitted successfully")
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
	node := &models.GPUNode{ID: "id1", Name: "n1"}
	logger := &testLogger{}
	err := SoftResetInstance(context.Background(), node, makeEnv(), slog.New(logger))
	require.Error(t, err)
	require.Contains(t, err.Error(), "fail")
	require.Contains(t, err.Error(), "reqid")
}

// fakePoolMgmt fakes an instance pool OCI reports at size, recording the
// size an update requests.
func fakePoolMgmt(t *testing.T, size int, updateErr error) *int {
	t.Helper()
	orig := newComputeMgmtClient
	t.Cleanup(func() { newComputeMgmtClient = orig })
	gotSize := new(int)
	newComputeMgmtClient = func(_ models.Environment) (computeMgmtClient, error) {
		return &fakeMgmtClient{
			GetInstancePoolFunc: func(_ context.Context, req core.GetInstancePoolRequest) (core.GetInstancePoolResponse, error) {
				assert.Equal(t, "pid", *req.InstancePoolId)
				return core.GetInstancePoolResponse{InstancePool: core.InstancePool{Size: common.Int(size)}}, nil
			},
			UpdateInstancePoolFunc: func(_ context.Context, req core.UpdateInstancePoolRequest) (core.UpdateInstancePoolResponse, error) {
				*gotSize = *req.Size
				return core.UpdateInstancePoolResponse{OpcRequestId: strPtr("reqid")}, updateErr
			},
		}, nil
	}
	return gotSize
}

// add counts from the size OCI reports, past the declared size if need be.
func TestIncreasePoolSize_AddsToOCISize(t *testing.T) {
	gotSize := fakePoolMgmt(t, 3, nil)
	pool := &models.GPUPool{ID: "pid", Name: "pname", ActualSize: 3, Size: 3}
	logger := &testLogger{}
	scale, err := IncreasePoolSize(context.Background(), pool, 2, makeEnv(), slog.New(logger))
	require.NoError(t, err)
	assert.Equal(t, 5, *gotSize, "two past the declared size")
	assert.Equal(t, PoolScale{Shown: 3, Before: 3, After: 5}, scale)
	assert.Equal(t, "3 → 5 instances", scale.String())
	require.Contains(t, logger.Infos[0], "scaling up instance pool")
	require.Contains(t, logger.Infos[1], "scaling request is submitted successfully")
}

// A size that moved since the caller looked is the one added to, and
// the result says so.
func TestIncreasePoolSize_StaleActualSize(t *testing.T) {
	gotSize := fakePoolMgmt(t, 5, nil)
	pool := &models.GPUPool{ID: "pid", Name: "pname", ActualSize: 4, Size: 4}
	scale, err := IncreasePoolSize(context.Background(), pool, 2, makeEnv(), slog.New(&testLogger{}))
	require.NoError(t, err)
	assert.Equal(t, 7, *gotSize)
	assert.Equal(t, "5 → 7 instances (was 5, not 4)", scale.String())
}

// FillPool fills to the declared size from the size OCI reports, not
// from a stale ActualSize: it never goes past the declared size.
func TestIncreasePoolSize_Fill(t *testing.T) {
	gotSize := fakePoolMgmt(t, 5, nil)
	pool := &models.GPUPool{ID: "pid", Name: "pname", ActualSize: 4, Size: 6}
	scale, err := IncreasePoolSize(context.Background(), pool, FillPool, makeEnv(), slog.New(&testLogger{}))
	require.NoError(t, err)
	assert.Equal(t, 6, *gotSize, "one short of 6 now, not two")
	assert.Equal(t, "5 → 6 instances (was 5, not 4)", scale.String())
}

// A fill that finds OCI already at (or past) the declared size updates
// nothing.
func TestIncreasePoolSize_FillAtSize(t *testing.T) {
	gotSize := fakePoolMgmt(t, 6, nil)
	pool := &models.GPUPool{ID: "pid", Name: "pname", ActualSize: 4, Size: 6}
	scale, err := IncreasePoolSize(context.Background(), pool, FillPool, makeEnv(), slog.New(&testLogger{}))
	require.NoError(t, err)
	assert.Equal(t, 0, *gotSize, "no update")
	assert.Equal(t, PoolScale{Shown: 4, Before: 6, After: 6}, scale)
	assert.Equal(t, "already 6 instances; nothing to scale (was 6, not 4)", scale.String())
}

// Nothing reaches OCI for a pool it doesn't have or a negative count.
func TestIncreasePoolSize_Refusals(t *testing.T) {
	orig := newComputeMgmtClient
	defer func() { newComputeMgmtClient = orig }()
	newComputeMgmtClient = func(models.Environment) (computeMgmtClient, error) {
		t.Fatal("must not reach OCI")
		return nil, errors.New("unreachable")
	}
	_, err := IncreasePoolSize(context.Background(), &models.GPUPool{Name: "gone", Status: models.GPUPoolStatusNonexistent}, 1, makeEnv(), slog.New(&testLogger{}))
	require.ErrorIs(t, err, ErrNoInstancePool)
	_, err = IncreasePoolSize(context.Background(), &models.GPUPool{ID: "pid", Name: "p"}, -1, makeEnv(), slog.New(&testLogger{}))
	require.ErrorContains(t, err, "at least 1")
}

func TestIncreasePoolSize_Error(t *testing.T) {
	fakePoolMgmt(t, 2, errors.New("fail"))
	pool := &models.GPUPool{ID: "pid", Name: "pname", ActualSize: 2, Size: 3}
	_, err := IncreasePoolSize(context.Background(), pool, 1, makeEnv(), slog.New(&testLogger{}))
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
	node := &models.GPUNode{ID: "id1", Name: "n1"}
	logger := &testLogger{}
	err := TerminateInstance(context.Background(), node, makeEnv(), slog.New(logger))
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
	node := &models.GPUNode{ID: "id1", Name: "n1"}
	logger := &testLogger{}
	err := TerminateInstance(context.Background(), node, makeEnv(), slog.New(logger))
	require.Error(t, err)
	require.Contains(t, err.Error(), "fail")
	require.Contains(t, err.Error(), "reqid")
}

func TestPopulateGPUPools(t *testing.T) {
	orig := newComputeMgmtClient
	defer func() { newComputeMgmtClient = orig }()
	// Case: empty slice or empty compartmentID
	err := PopulateGPUPools(context.Background(), nil, makeEnv(), "comp")
	require.NoError(t, err)
	err = PopulateGPUPools(context.Background(), []models.GPUPool{{}}, makeEnv(), "")
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
	pools := []models.GPUPool{{Name: "p1"}}
	err = PopulateGPUPools(context.Background(), pools, makeEnv(), "comp")
	require.NoError(t, err)
	require.Equal(t, "id1", pools[0].ID)
	require.Equal(t, 3, pools[0].ActualSize)
	require.Equal(t, "RUNNING", pools[0].Status)

	// Case: pool not found
	pools = []models.GPUPool{{Name: "p2"}}
	newComputeMgmtClient = func(_ models.Environment) (computeMgmtClient, error) {
		return &fakeMgmtClient{
			ListInstancePoolsFunc: func(_ context.Context, _ core.ListInstancePoolsRequest) (core.ListInstancePoolsResponse, error) {
				return core.ListInstancePoolsResponse{
					Items: []core.InstancePoolSummary{},
				}, nil
			},
		}, nil
	}
	err = PopulateGPUPools(context.Background(), pools, makeEnv(), "comp")
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
	pools = []models.GPUPool{{Name: "p3"}}
	err = PopulateGPUPools(context.Background(), pools, makeEnv(), "comp")
	require.Error(t, err)
	require.Contains(t, err.Error(), "fail")
	require.Contains(t, err.Error(), "reqid")
}

// Finding #5: instance-pool summaries with nil DisplayName/Id/Size must be
// skipped rather than panic during enrichment; valid summaries still apply.
func TestPopulateGPUPools_NilSummaryFields(t *testing.T) {
	orig := newComputeMgmtClient
	defer func() { newComputeMgmtClient = orig }()
	newComputeMgmtClient = func(_ models.Environment) (computeMgmtClient, error) {
		return &fakeMgmtClient{
			ListInstancePoolsFunc: func(_ context.Context, _ core.ListInstancePoolsRequest) (core.ListInstancePoolsResponse, error) {
				return core.ListInstancePoolsResponse{
					Items: []core.InstancePoolSummary{
						{DisplayName: nil, Id: strPtr("idnil"), Size: intPtr(1), LifecycleState: "RUNNING"},
						{DisplayName: strPtr("p1"), Id: nil, Size: nil, LifecycleState: "RUNNING"},
						{DisplayName: strPtr("p2"), Id: strPtr("id2"), Size: intPtr(4), LifecycleState: "RUNNING"},
					},
				}, nil
			},
		}, nil
	}

	pools := []models.GPUPool{{Name: "p1"}, {Name: "p2"}}
	err := PopulateGPUPools(context.Background(), pools, makeEnv(), "comp")
	require.NoError(t, err)

	// p1's summary had nil Id/Size → skipped, stays NONEXIST.
	require.Equal(t, "NONEXIST", pools[0].Status)
	// p2's summary was complete → enriched.
	require.Equal(t, "id2", pools[1].ID)
	require.Equal(t, 4, pools[1].ActualSize)
	require.Equal(t, "RUNNING", pools[1].Status)
}

// Pools listed past the first ListInstancePools page must be enriched, not
// left NONEXIST: every page is requested with the previous OpcNextPage.
func TestPopulateGPUPools_FollowsPages(t *testing.T) {
	orig := newComputeMgmtClient
	defer func() { newComputeMgmtClient = orig }()

	summary := func(name, id string, size int) core.InstancePoolSummary {
		return core.InstancePoolSummary{DisplayName: strPtr(name), Id: strPtr(id), Size: intPtr(size), LifecycleState: "RUNNING"}
	}
	pages := map[string]core.ListInstancePoolsResponse{
		"":   {Items: []core.InstancePoolSummary{summary("p1", "id1", 1)}, OpcNextPage: strPtr("t2")},
		"t2": {Items: []core.InstancePoolSummary{summary("p2", "id2", 2)}, OpcNextPage: strPtr("t3")},
		"t3": {Items: []core.InstancePoolSummary{summary("p3", "id3", 3)}},
	}
	var requested []string
	newComputeMgmtClient = func(_ models.Environment) (computeMgmtClient, error) {
		return &fakeMgmtClient{
			ListInstancePoolsFunc: func(_ context.Context, req core.ListInstancePoolsRequest) (core.ListInstancePoolsResponse, error) {
				require.Equal(t, "comp", derefOr(req.CompartmentId, ""))
				page := derefOr(req.Page, "")
				requested = append(requested, page)
				return pages[page], nil
			},
		}, nil
	}

	pools := []models.GPUPool{{Name: "p3"}, {Name: "p1"}, {Name: "p2"}, {Name: "gone"}}
	err := PopulateGPUPools(context.Background(), pools, makeEnv(), "comp")
	require.NoError(t, err)
	require.Equal(t, []string{"", "t2", "t3"}, requested)
	require.Equal(t, "id3", pools[0].ID)
	require.Equal(t, 3, pools[0].ActualSize)
	require.Equal(t, "id1", pools[1].ID)
	require.Equal(t, "id2", pools[2].ID)
	require.Equal(t, "RUNNING", pools[2].Status)
	require.Equal(t, "NONEXIST", pools[3].Status)
}

// An error on a later page fails the whole populate with that page's request id.
func TestPopulateGPUPools_PageError(t *testing.T) {
	orig := newComputeMgmtClient
	defer func() { newComputeMgmtClient = orig }()
	newComputeMgmtClient = func(_ models.Environment) (computeMgmtClient, error) {
		return &fakeMgmtClient{
			ListInstancePoolsFunc: func(_ context.Context, req core.ListInstancePoolsRequest) (core.ListInstancePoolsResponse, error) {
				if req.Page == nil {
					return core.ListInstancePoolsResponse{OpcNextPage: strPtr("t2")}, nil
				}
				return core.ListInstancePoolsResponse{OpcRequestId: strPtr("reqid2")}, errors.New("page2 fail")
			},
		}, nil
	}
	err := PopulateGPUPools(context.Background(), []models.GPUPool{{Name: "p1"}}, makeEnv(), "comp")
	require.Error(t, err)
	require.Contains(t, err.Error(), "page2 fail")
	require.Contains(t, err.Error(), "reqid2")
}

func strPtr(s string) *string { return &s }
func intPtr(i int) *int       { return &i }
