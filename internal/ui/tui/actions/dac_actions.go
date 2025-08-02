// Package actions: delete_dac.go
// Implements synchronous deletion of DedicatedAICluster via OCI Generative AI SDK.
package actions

import (
	"context"
	"fmt"
	"time"

	"github.com/jingle2008/toolkit/internal/infra/oci"
	"github.com/jingle2008/toolkit/pkg/infra/logging"
	"github.com/jingle2008/toolkit/pkg/models"
	"github.com/oracle/oci-go-sdk/v65/generativeai"
)

/*
DeleteDedicatedAICluster deletes a DedicatedAICluster using the OCI Generative AI SDK.
*/
func DeleteDedicatedAICluster(ctx context.Context, dac *models.DedicatedAICluster, env models.Environment, logger logging.Logger) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	client, err := oci.GetGenAIClient(env, logger)
	if err != nil {
		return fmt.Errorf("failed to create GenerativeAI client: %w", err)
	}

	dacID := dac.GetID(env.Realm, env.Region)
	getReq := generativeai.GetDedicatedAiClusterRequest{
		DedicatedAiClusterId: &dacID,
	}
	getResp, err := client.GetDedicatedAiCluster(ctx, getReq)
	if err != nil {
		return fmt.Errorf("failed to get DedicatedAiCluster: %w", err)
	}

	logger.Infow("deleting DedicatedAiCluster", "id", getResp.Id, "name", getResp.DisplayName)
	if err := deleteEndpointsInDAC(ctx, client, &getResp.DedicatedAiCluster, logger); err != nil {
		return err
	}

	delReq := generativeai.DeleteDedicatedAiClusterRequest{
		DedicatedAiClusterId: &dacID,
	}
	if delResp, err := client.DeleteDedicatedAiCluster(ctx, delReq); err != nil {
		return fmt.Errorf("failed to delete DedicatedAICluster: %w, request id: %s",
			err, *delResp.OpcRequestId)
	}

	return nil
}

func deleteEndpointsInDAC(
	ctx context.Context,
	client *generativeai.GenerativeAiClient,
	dac *generativeai.DedicatedAiCluster,
	logger logging.Logger,
) error {
	compartmentId := dac.CompartmentId
	listReq := generativeai.ListEndpointsRequest{
		CompartmentId: compartmentId,
	}
	listResp, err := client.ListEndpoints(ctx, listReq)
	if err != nil {
		return fmt.Errorf("failed to list endpoints: %w", err)
	}

	count := 0
	logger.Infow("endpoints found", "count", len(listResp.Items))
	for _, item := range listResp.Items {
		if *item.DedicatedAiClusterId != *dac.Id {
			continue
		}

		count++
		if err := deleteEndpoint(ctx, client, &item, logger); err != nil {
			return err
		}
	}

	logger.Infow("endpoints deleted", "count", count)
	return nil
}

func deleteEndpoint(
	ctx context.Context,
	client *generativeai.GenerativeAiClient,
	endpoint *generativeai.EndpointSummary,
	logger logging.Logger,
) error {
	logger.Infow("deleting endpoint", "id", endpoint.Id, "name", endpoint.DisplayName)
	delReq := generativeai.DeleteEndpointRequest{
		EndpointId: endpoint.Id,
	}
	delResp, err := client.DeleteEndpoint(ctx, delReq)
	if err != nil {
		return fmt.Errorf("failed to delete endpoint: %w", err)
	}

	logger.Infow("endpoint is deleted successfully", "id",
		endpoint.Id, "opc-request-id", delResp.OpcRequestId)

	return nil
}
