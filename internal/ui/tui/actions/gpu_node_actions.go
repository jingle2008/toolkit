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

// SoftResetInstance performs a soft reset (reboot) of the given instance.
func SoftResetInstance(
	ctx context.Context,
	node *models.GpuNode,
	env models.Environment,
	logger logging.Logger,
) error {
	client, err := oci.GetComputeClient(env)
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

// TerminateInstance terminates the given instance, optionally preserving the boot volume.
func TerminateInstance(
	ctx context.Context,
	node *models.GpuNode,
	env models.Environment,
	logger logging.Logger,
) error {
	client, err := oci.GetComputeClient(env)
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
