package oci

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/core"

	"github.com/jingle2008/toolkit/pkg/models"
)

var (
	newComputeClient func(models.Environment) (computeClient, error) = func(env models.Environment) (computeClient, error) {
		return NewComputeClient(env)
	}
	newComputeMgmtClient func(models.Environment) (computeMgmtClient, error) = func(env models.Environment) (computeMgmtClient, error) {
		return NewComputeManagementClient(env)
	}
)

type computeClient interface {
	InstanceAction(ctx context.Context, req core.InstanceActionRequest) (core.InstanceActionResponse, error)
	TerminateInstance(ctx context.Context, req core.TerminateInstanceRequest) (core.TerminateInstanceResponse, error)
}

type computeMgmtClient interface {
	GetInstancePool(ctx context.Context, req core.GetInstancePoolRequest) (core.GetInstancePoolResponse, error)
	UpdateInstancePool(ctx context.Context, req core.UpdateInstancePoolRequest) (core.UpdateInstancePoolResponse, error)
	ListInstancePools(ctx context.Context, req core.ListInstancePoolsRequest) (core.ListInstancePoolsResponse, error)
}

// SoftResetInstance performs a soft reset (restart) of the given instance.
func SoftResetInstance(
	ctx context.Context,
	node *models.GPUNode,
	env models.Environment,
	logger *slog.Logger,
) error {
	client, err := newComputeClient(env)
	if err != nil {
		return fmt.Errorf("failed to create compute client: %w", err)
	}

	logger.Info("restarting node", "id", node.ID, "name", node.Name)
	resp, err := client.InstanceAction(ctx, core.InstanceActionRequest{
		InstanceId: common.String(node.ID),
		Action:     core.InstanceActionActionSoftreset,
	})
	if err != nil {
		reqID := ""
		if resp.OpcRequestId != nil {
			reqID = *resp.OpcRequestId
		}
		return fmt.Errorf("failed to soft reset instance: %w, request id: %s", err, reqID)
	}

	logger.Info("restart request is submitted successfully", "id",
		node.ID, "opc-request-id", resp.OpcRequestId)
	return nil
}

// ErrNoInstancePool is returned for a pool OCI has no instance pool for
// (status NONEXIST): there is nothing to scale.
var ErrNoInstancePool = errors.New("no OCI instance pool; nothing to scale")

// PoolScale is a scale-up's outcome: the size the caller last saw, the
// size OCI reported just before the update, and the size requested.
type PoolScale struct {
	Shown  int `json:"shown"`
	Before int `json:"before"`
	After  int `json:"after"`
}

// String renders s as "4 → 6 instances", noting when OCI's size had moved
// from the one the caller saw.
func (s PoolScale) String() string {
	out := fmt.Sprintf("%d → %d instances", s.Before, s.After)
	if s.Before != s.Shown {
		out += fmt.Sprintf(" (was %d, not %d)", s.Before, s.Shown)
	}
	return out
}

// IncreasePoolSize grows the given GPU pool's instance pool by add
// instances, which may take it past its Terraform-declared size. add is
// counted from the size OCI reports just before the update, not from
// pool.ActualSize, which may be stale.
func IncreasePoolSize(
	ctx context.Context,
	pool *models.GPUPool,
	add int,
	env models.Environment,
	logger *slog.Logger,
) (PoolScale, error) {
	scale := PoolScale{Shown: pool.ActualSize}
	if pool.ID == "" {
		return scale, fmt.Errorf("pool %s: %w", pool.Name, ErrNoInstancePool)
	}
	if add < 1 {
		return scale, fmt.Errorf("pool %s: scale-up must add at least 1 instance, got %d", pool.Name, add)
	}
	mgmtClient, err := newComputeMgmtClient(env)
	if err != nil {
		return scale, fmt.Errorf("failed to create compute management client: %w", err)
	}

	current, err := mgmtClient.GetInstancePool(ctx, core.GetInstancePoolRequest{InstancePoolId: common.String(pool.ID)})
	if err != nil {
		return scale, fmt.Errorf("failed to get instance pool: %w, request id: %s", err, derefOr(current.OpcRequestId, ""))
	}
	if current.Size == nil {
		return scale, fmt.Errorf("instance pool %s reported no size", pool.ID)
	}
	scale.Before = *current.Size
	scale.After = scale.Before + add
	newSize := scale.After
	logger.Info("scaling up instance pool", "id", pool.ID, "name", pool.Name, "from", scale.Before, "newSize", newSize)
	resp, err := mgmtClient.UpdateInstancePool(ctx, core.UpdateInstancePoolRequest{
		InstancePoolId: common.String(pool.ID),
		UpdateInstancePoolDetails: core.UpdateInstancePoolDetails{
			Size: common.Int(newSize),
		},
	})
	if err != nil {
		reqID := ""
		if resp.OpcRequestId != nil {
			reqID = *resp.OpcRequestId
		}
		return scale, fmt.Errorf("failed to update instance pool: %w, request id: %s", err, reqID)
	}
	logger.Info("scaling request is submitted successfully", "id",
		pool.ID, "opc-request-id", resp.OpcRequestId)
	return scale, nil
}

// TerminateInstance terminates the given instance, optionally preserving the boot volume.
func TerminateInstance(
	ctx context.Context,
	node *models.GPUNode,
	env models.Environment,
	logger *slog.Logger,
) error {
	client, err := newComputeClient(env)
	if err != nil {
		return fmt.Errorf("failed to create compute client: %w", err)
	}

	logger.Info("deleting node", "id", node.ID, "name", node.Name)
	resp, err := client.TerminateInstance(ctx, core.TerminateInstanceRequest{
		InstanceId:         common.String(node.ID),
		PreserveBootVolume: common.Bool(false),
	})
	if err != nil {
		reqID := ""
		if resp.OpcRequestId != nil {
			reqID = *resp.OpcRequestId
		}
		return fmt.Errorf("failed to terminate instance: %w, request id: %s", err, reqID)
	}
	logger.Info("deletion request is submitted successfully", "id",
		node.ID, "opc-request-id", resp.OpcRequestId)
	return nil
}

// PopulateGPUPools populates ActualSize and Status for a GPUPool using OCI instance pool API.
// compartmentID is required to look up the pool.
func PopulateGPUPools(
	ctx context.Context,
	gpuPools []models.GPUPool,
	env models.Environment,
	compartmentID string,
) error {
	if len(gpuPools) == 0 || compartmentID == "" {
		return nil // nothing to do
	}

	mgmtClient, err := newComputeMgmtClient(env)
	if err != nil {
		return err
	}

	summaries, err := listAllPages(func(page *string) ([]core.InstancePoolSummary, *string, error) {
		resp, err := mgmtClient.ListInstancePools(ctx, core.ListInstancePoolsRequest{
			CompartmentId: &compartmentID,
			Page:          page,
		})
		if err != nil {
			return nil, nil, fmt.Errorf("failed to list instance pools: %w, request id: %s",
				err, derefOr(resp.OpcRequestId, ""))
		}
		return resp.Items, resp.OpcNextPage, nil
	})
	if err != nil {
		return err
	}

	gpuPoolMap := make(map[string]*models.GPUPool)
	for i := range gpuPools {
		gpuPoolMap[gpuPools[i].Name] = &gpuPools[i]
		gpuPools[i].Status = models.GPUPoolStatusNonexistent
	}

	applyInstancePoolSummaries(gpuPoolMap, summaries)
	return nil
}

// applyInstancePoolSummaries enriches matching pools from instance-pool
// summaries, skipping any summary missing a field it reads so a partial OCI
// response can never panic enrichment.
func applyInstancePoolSummaries(gpuPoolMap map[string]*models.GPUPool, summaries []core.InstancePoolSummary) {
	for _, summary := range summaries {
		if summary.DisplayName == nil || summary.Id == nil || summary.Size == nil {
			continue
		}
		if pool, ok := gpuPoolMap[*summary.DisplayName]; ok {
			pool.ID = *summary.Id
			pool.ActualSize = *summary.Size
			pool.Status = string(summary.LifecycleState)
		}
	}
}
