// Package actions: delete_dac.go
// Implements synchronous deletion of DedicatedAICluster via OCI Generative AI SDK.
package actions

import (
	"context"
	"fmt"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/jingle2008/toolkit/internal/infra/oci"
	"github.com/jingle2008/toolkit/pkg/infra/logging"
	"github.com/jingle2008/toolkit/pkg/models"
	"github.com/oracle/oci-go-sdk/v65/generativeai"
)

var getGenAIClient = oci.GetGenAIClient

/*
DeleteDedicatedAICluster deletes a DedicatedAICluster using the OCI Generative AI SDK.
*/
func DeleteDedicatedAICluster(ctx context.Context, dac *models.DedicatedAICluster, env models.Environment, logger logging.Logger) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	client, err := getGenAIClient(env, logger)
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
	delResp, err := client.DeleteDedicatedAiCluster(ctx, delReq)
	if err != nil {
		return fmt.Errorf("failed to delete DedicatedAICluster: %w, request id: %s",
			err, *delResp.OpcRequestId)
	}

	// Poll work request status before returning
	if err := waitForWorkRequest(ctx, client, delResp.OpcWorkRequestId, logger); err != nil {
		return fmt.Errorf("DedicatedAICluster deletion did not complete successfully: %w", err)
	}

	logger.Infow("DedicatedAICluster is deleted successfully", "id",
		dacID, "opc-request-id", delResp.OpcRequestId)

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

	logger.Infow("endpoints found", "count", len(listResp.Items))

	var count int32
	const maxConcurrent = 5
	sem := make(chan struct{}, maxConcurrent)
	g, gctx := errgroup.WithContext(ctx)

	for _, item := range listResp.Items {
		if *item.DedicatedAiClusterId != *dac.Id {
			continue
		}

		sem <- struct{}{}
		count++
		ep := item // capture loop variable
		g.Go(func() error {
			defer func() { <-sem }()
			return deleteEndpoint(gctx, client, &ep, logger)
		})
	}

	if err := g.Wait(); err != nil {
		return err
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

	// Poll work request status before returning
	if err := waitForWorkRequest(ctx, client, delResp.OpcWorkRequestId, logger); err != nil {
		return fmt.Errorf("endpoint deletion did not complete successfully: %w", err)
	}

	logger.Infow("endpoint is deleted successfully", "id",
		endpoint.Id, "opc-request-id", delResp.OpcRequestId)

	return nil
}

func waitForWorkRequest(
	ctx context.Context,
	client *generativeai.GenerativeAiClient,
	workRequestID *string,
	logger logging.Logger,
) error {
	const interval = 5 * time.Second
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			wrReq := generativeai.GetWorkRequestRequest{WorkRequestId: workRequestID}
			wrResp, err := client.GetWorkRequest(ctx, wrReq)
			if err != nil {
				logger.Infow("failed to get work request status", "level", "warn", "workRequestId", *workRequestID, "err", err)
				continue
			}
			status := string(wrResp.Status)
			if status == "SUCCEEDED" {
				return nil
			}
			if status == "FAILED" {
				return fmt.Errorf("work request %s failed", *workRequestID)
			}
			// Otherwise, keep polling for Accepted/InProgress
		}
	}
}
