//nolint:paralleltest
package actions

import (
	"context"
	"errors"
	"testing"

	"github.com/jingle2008/toolkit/pkg/models"
	"github.com/oracle/oci-go-sdk/v65/generativeai"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeGenAI struct {
	GetDedicatedAiClusterResp generativeai.GetDedicatedAiClusterResponse
	ListEndpointsResp         generativeai.ListEndpointsResponse
	DeleteEndpointResp        generativeai.DeleteEndpointResponse
	DeleteDedicatedResp       generativeai.DeleteDedicatedAiClusterResponse
	GetWorkRequestResp        generativeai.GetWorkRequestResponse
	Err                       error
}

func TestDeleteDedicatedAICluster_Success(t *testing.T) {
	origGetGenAIClient := getGenAIClient
	defer func() { getGenAIClient = origGetGenAIClient }()

	// Setup fake responses
	clusterID := "ocid1.dedicatedaicluster.oc1..example"
	endpointID := "ocid1.endpoint.oc1..endpoint"
	workReqID := "ocid1.workrequest.oc1..workreq"
	opcReqID := "req-123"

	fakeClient := &fakeGenAI{
		GetDedicatedAiClusterResp: generativeai.GetDedicatedAiClusterResponse{
			DedicatedAiCluster: generativeai.DedicatedAiCluster{
				Id:            &clusterID,
				CompartmentId: func() *string { s := "compartment"; return &s }(),
				DisplayName:   func() *string { s := "test-dac"; return &s }(),
			},
		},
		ListEndpointsResp: generativeai.ListEndpointsResponse{
			EndpointCollection: generativeai.EndpointCollection{
				Items: []generativeai.EndpointSummary{
					{
						Id:                   &endpointID,
						DisplayName:          func() *string { s := "ep1"; return &s }(),
						DedicatedAiClusterId: &clusterID,
					},
				},
			},
		},
		DeleteEndpointResp: generativeai.DeleteEndpointResponse{
			OpcWorkRequestId: &workReqID,
			OpcRequestId:     &opcReqID,
		},
		DeleteDedicatedResp: generativeai.DeleteDedicatedAiClusterResponse{
			OpcWorkRequestId: &workReqID,
			OpcRequestId:     &opcReqID,
		},
		GetWorkRequestResp: generativeai.GetWorkRequestResponse{
			WorkRequest: generativeai.WorkRequest{
				Status: generativeai.OperationStatusSucceeded,
			},
		},
	}

	getGenAIClient = func(_ models.Environment) (genAI, error) {
		return fakeClient, nil
	}

	dac := &models.DedicatedAICluster{}
	env := models.Environment{Type: "prod", Region: "us-phoenix-1", Realm: "oc1"}
	logger := &fakeLogger{}
	err := DeleteDedicatedAICluster(context.Background(), dac, env, logger)
	require.NoError(t, err)
}

func (f *fakeGenAI) GetDedicatedAiCluster(_ context.Context, _ generativeai.GetDedicatedAiClusterRequest) (generativeai.GetDedicatedAiClusterResponse, error) {
	return f.GetDedicatedAiClusterResp, f.Err
}

func (f *fakeGenAI) DeleteDedicatedAiCluster(_ context.Context, _ generativeai.DeleteDedicatedAiClusterRequest) (generativeai.DeleteDedicatedAiClusterResponse, error) {
	return f.DeleteDedicatedResp, f.Err
}

func (f *fakeGenAI) ListEndpoints(_ context.Context, _ generativeai.ListEndpointsRequest) (generativeai.ListEndpointsResponse, error) {
	return f.ListEndpointsResp, f.Err
}

func (f *fakeGenAI) DeleteEndpoint(_ context.Context, _ generativeai.DeleteEndpointRequest) (generativeai.DeleteEndpointResponse, error) {
	return f.DeleteEndpointResp, f.Err
}

func (f *fakeGenAI) GetWorkRequest(_ context.Context, _ generativeai.GetWorkRequestRequest) (generativeai.GetWorkRequestResponse, error) {
	return f.GetWorkRequestResp, f.Err
}

func TestDeleteDedicatedAICluster_ClientError(t *testing.T) {
	t.Parallel()
	origGetGenAIClient := getGenAIClient
	defer func() { getGenAIClient = origGetGenAIClient }()

	// Patch getGenAIClient to simulate client creation error
	getGenAIClient = func(_ models.Environment) (genAI, error) {
		return nil, errors.New("client creation failed")
	}

	dac := &models.DedicatedAICluster{}
	env := models.Environment{Type: "prod", Region: "us-phoenix-1", Realm: "oc1"}
	logger := &fakeLogger{}
	err := DeleteDedicatedAICluster(context.Background(), dac, env, logger)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create GenerativeAI client")
}
