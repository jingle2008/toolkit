package actions

import (
	"context"
	"fmt"

	"github.com/jingle2008/toolkit/internal/infra/oci"
	"github.com/jingle2008/toolkit/pkg/infra/logging"
	"github.com/jingle2008/toolkit/pkg/models"
	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/core"
)

var (
	newComputeClient func(models.Environment) (computeClient, error) = func(env models.Environment) (computeClient, error) {
		return oci.GetComputeClient(env)
	}
	newComputeMgmtClient func(models.Environment) (computeMgmtClient, error) = func(env models.Environment) (computeMgmtClient, error) {
		return oci.GetComputeManagementClient(env)
	}
)

type computeClient interface {
	InstanceAction(ctx context.Context, req core.InstanceActionRequest) (core.InstanceActionResponse, error)
	TerminateInstance(ctx context.Context, req core.TerminateInstanceRequest) (core.TerminateInstanceResponse, error)
}

type computeMgmtClient interface {
	UpdateInstancePool(ctx context.Context, req core.UpdateInstancePoolRequest) (core.UpdateInstancePoolResponse, error)
	ListInstancePools(ctx context.Context, req core.ListInstancePoolsRequest) (core.ListInstancePoolsResponse, error)
}

// SoftResetInstance performs a soft reset (reboot) of the given instance.
func SoftResetInstance(
	ctx context.Context,
	node *models.GpuNode,
	env models.Environment,
	logger logging.Logger,
) error {
	client, err := newComputeClient(env)
	if err != nil {
		return fmt.Errorf("failed to create compute client: %w", err)
	}

	logger.Infow("rebooting node", "id", node.ID, "name", node.Name)
	resp, err := client.InstanceAction(ctx, core.InstanceActionRequest{
		InstanceId: common.String(node.ID),
		Action:     core.InstanceActionActionSoftreset,
	})
	if err != nil {
		return fmt.Errorf("failed to soft reset instance: %w, request id: %s",
			err, *resp.OpcRequestId)
	}

	logger.Infow("reboot request is submitted successfully", "id",
		node.ID, "opc-request-id", resp.OpcRequestId)
	return nil
}

// IncreasePoolSize scales up the given GPU pool by 1 and returns the work request ID.
func IncreasePoolSize(
	ctx context.Context,
	pool *models.GpuPool,
	env models.Environment,
	logger logging.Logger,
) error {
	mgmtClient, err := newComputeMgmtClient(env)
	if err != nil {
		return fmt.Errorf("failed to create compute management client: %w", err)
	}

	newSize := pool.ActualSize + 1
	logger.Infow("scaling up instance pool", "id", pool.ID, "name", pool.Name, "newSize", newSize)
	resp, err := mgmtClient.UpdateInstancePool(ctx, core.UpdateInstancePoolRequest{
		InstancePoolId: common.String(pool.ID),
		UpdateInstancePoolDetails: core.UpdateInstancePoolDetails{
			Size: common.Int(newSize),
		},
	})
	if err != nil {
		return fmt.Errorf("failed to update instance pool: %w, request id: %s",
			err, *resp.OpcRequestId)
	}
	logger.Infow("scaling request is submitted successfully", "id",
		pool.ID, "opc-request-id", resp.OpcRequestId)
	return nil
}

// TerminateInstance terminates the given instance, optionally preserving the boot volume.
func TerminateInstance(
	ctx context.Context,
	node *models.GpuNode,
	env models.Environment,
	logger logging.Logger,
) error {
	client, err := newComputeClient(env)
	if err != nil {
		return fmt.Errorf("failed to create compute client: %w", err)
	}

	logger.Infow("deleting node", "id", node.ID, "name", node.Name)
	resp, err := client.TerminateInstance(ctx, core.TerminateInstanceRequest{
		InstanceId:         common.String(node.ID),
		PreserveBootVolume: common.Bool(false),
	})
	if err != nil {
		return fmt.Errorf("failed to terminate instance: %w, request id: %s",
			err, *resp.OpcRequestId)
	}
	logger.Infow("deletion request is submitted successfully", "id",
		node.ID, "opc-request-id", resp.OpcRequestId)
	return nil
}

// PopulateGpuPools populates ActualSize and Status for a GpuPool using OCI instance pool API.
// compartmentID is required to look up the pool.
func PopulateGpuPools(
	ctx context.Context,
	gpuPools []models.GpuPool,
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

	req := core.ListInstancePoolsRequest{
		CompartmentId: &compartmentID,
	}
	resp, err := mgmtClient.ListInstancePools(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to list instance pools: %w, request id: %s",
			err, *resp.OpcRequestId)
	}

	gpuPoolMap := make(map[string]*models.GpuPool)
	for i := range gpuPools {
		gpuPoolMap[gpuPools[i].Name] = &gpuPools[i]
		gpuPools[i].Status = "NONEXIST"
	}

	for _, summary := range resp.Items {
		if pool, ok := gpuPoolMap[*summary.DisplayName]; ok {
			pool.ID = *summary.Id
			pool.ActualSize = *summary.Size
			pool.Status = string(summary.LifecycleState)
		}
	}
	return nil
}
