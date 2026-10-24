//nolint:paralleltest
package oci

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/oracle/oci-go-sdk/v65/generativeai"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jingle2008/toolkit/pkg/infra/logging"
	"github.com/jingle2008/toolkit/pkg/models"
)

type fakeGenAI struct {
	GetDedicatedAiClusterResp generativeai.GetDedicatedAiClusterResponse
	ListEndpointsResp         generativeai.ListEndpointsResponse
	DeleteEndpointResp        generativeai.DeleteEndpointResponse
	DeleteDedicatedResp       generativeai.DeleteDedicatedAiClusterResponse
	GetWorkRequestResp        generativeai.GetWorkRequestResponse
	Err                       error
	// DeleteDedicatedErr, when set, fails only the DeleteDedicatedAiCluster
	// call (Err fails every call), letting tests exercise the delete path
	// after Get/ListEndpoints succeed.
	DeleteDedicatedErr error
	// ListEndpointsFunc and DeleteEndpointFunc, when set, replace the canned
	// responses so tests can serve multiple pages and record deletions.
	ListEndpointsFunc  func(req generativeai.ListEndpointsRequest) (generativeai.ListEndpointsResponse, error)
	DeleteEndpointFunc func(req generativeai.DeleteEndpointRequest) (generativeai.DeleteEndpointResponse, error)
	// GetWorkRequestFunc, when set, replaces the canned work-request response.
	GetWorkRequestFunc func() (generativeai.GetWorkRequestResponse, error)
}

func TestDeleteDedicatedAICluster_Success(t *testing.T) {
	origNewGenAIClient := newGenAIClient
	defer func() { newGenAIClient = origNewGenAIClient }()

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

	newGenAIClient = func(_ models.Environment) (genAI, error) {
		return fakeClient, nil
	}

	dac := &models.DedicatedAICluster{}
	env := models.Environment{Type: "prod", Region: "us-phoenix-1", Realm: "oc1"}
	logger := &testLogger{}
	err := DeleteDedicatedAICluster(context.Background(), dac, env, logger)
	require.NoError(t, err)
}

func (f *fakeGenAI) GetDedicatedAiCluster(_ context.Context, _ generativeai.GetDedicatedAiClusterRequest) (generativeai.GetDedicatedAiClusterResponse, error) {
	return f.GetDedicatedAiClusterResp, f.Err
}

func (f *fakeGenAI) DeleteDedicatedAiCluster(_ context.Context, _ generativeai.DeleteDedicatedAiClusterRequest) (generativeai.DeleteDedicatedAiClusterResponse, error) {
	if f.DeleteDedicatedErr != nil {
		return f.DeleteDedicatedResp, f.DeleteDedicatedErr
	}
	return f.DeleteDedicatedResp, f.Err
}

func (f *fakeGenAI) ListEndpoints(_ context.Context, req generativeai.ListEndpointsRequest) (generativeai.ListEndpointsResponse, error) {
	if f.ListEndpointsFunc != nil {
		return f.ListEndpointsFunc(req)
	}
	return f.ListEndpointsResp, f.Err
}

func (f *fakeGenAI) DeleteEndpoint(_ context.Context, req generativeai.DeleteEndpointRequest) (generativeai.DeleteEndpointResponse, error) {
	if f.DeleteEndpointFunc != nil {
		return f.DeleteEndpointFunc(req)
	}
	return f.DeleteEndpointResp, f.Err
}

func (f *fakeGenAI) GetWorkRequest(_ context.Context, _ generativeai.GetWorkRequestRequest) (generativeai.GetWorkRequestResponse, error) {
	if f.GetWorkRequestFunc != nil {
		return f.GetWorkRequestFunc()
	}
	return f.GetWorkRequestResp, f.Err
}

func TestDeleteDedicatedAICluster_ClientError(t *testing.T) {
	t.Parallel()
	origNewGenAIClient := newGenAIClient
	defer func() { newGenAIClient = origNewGenAIClient }()

	// Patch newGenAIClient to simulate client creation error
	newGenAIClient = func(_ models.Environment) (genAI, error) {
		return nil, errors.New("client creation failed")
	}

	dac := &models.DedicatedAICluster{}
	env := models.Environment{Type: "prod", Region: "us-phoenix-1", Realm: "oc1"}
	logger := &testLogger{}
	err := DeleteDedicatedAICluster(context.Background(), dac, env, logger)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create GenerativeAI client")
}

// validClusterFake returns a fake whose Get/ListEndpoints succeed, so tests can
// drive the delete path. ListEndpoints is empty by default (no endpoint work).
func validClusterFake() *fakeGenAI {
	clusterID := "ocid1.dedicatedaicluster.oc1..example"
	return &fakeGenAI{
		GetDedicatedAiClusterResp: generativeai.GetDedicatedAiClusterResponse{
			DedicatedAiCluster: generativeai.DedicatedAiCluster{
				Id:            &clusterID,
				CompartmentId: func() *string { s := "compartment"; return &s }(),
				DisplayName:   func() *string { s := "test-dac"; return &s }(),
			},
		},
	}
}

// Finding #2: a delete failure with a partial (nil OpcRequestId) response must
// surface a controlled error, not panic dereferencing the nil pointer.
func TestDeleteDedicatedAICluster_DeleteError_NilOpcRequestId(t *testing.T) {
	origNewGenAIClient := newGenAIClient
	defer func() { newGenAIClient = origNewGenAIClient }()

	fakeClient := validClusterFake()
	fakeClient.DeleteDedicatedErr = errors.New("boom")
	// DeleteDedicatedResp left zero-valued → OpcRequestId is nil.
	newGenAIClient = func(_ models.Environment) (genAI, error) { return fakeClient, nil }

	dac := &models.DedicatedAICluster{}
	env := models.Environment{Type: "prod", Region: "us-phoenix-1", Realm: "oc1"}
	err := DeleteDedicatedAICluster(context.Background(), dac, env, &testLogger{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to delete DedicatedAICluster")
	assert.Contains(t, err.Error(), "boom")
}

// Finding #2: a delete response missing OpcWorkRequestId must yield a controlled
// error immediately, not deref nil in waitForWorkRequest nor poll until timeout.
func TestDeleteDedicatedAICluster_NilWorkRequestID(t *testing.T) {
	origNewGenAIClient := newGenAIClient
	defer func() { newGenAIClient = origNewGenAIClient }()

	fakeClient := validClusterFake()
	opcReqID := "req-123"
	fakeClient.DeleteDedicatedResp = generativeai.DeleteDedicatedAiClusterResponse{
		OpcRequestId: &opcReqID, // OpcWorkRequestId intentionally nil
	}
	newGenAIClient = func(_ models.Environment) (genAI, error) { return fakeClient, nil }

	// Bound the call so a regression (polling a nil work request forever) fails
	// fast as a timeout rather than hanging the suite.
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	dac := &models.DedicatedAICluster{}
	env := models.Environment{Type: "prod", Region: "us-phoenix-1", Realm: "oc1"}
	err := DeleteDedicatedAICluster(ctx, dac, env, &testLogger{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "work request id is nil")
}

// Finding #3: an endpoint summary with a nil DedicatedAiClusterId must be
// skipped, not panic the whole deletion before the cluster delete runs.
func TestDeleteDedicatedAICluster_NilEndpointClusterID_Skipped(t *testing.T) {
	origNewGenAIClient := newGenAIClient
	defer func() { newGenAIClient = origNewGenAIClient }()

	fakeClient := validClusterFake()
	epID := "ocid1.endpoint.oc1..ep"
	fakeClient.ListEndpointsResp = generativeai.ListEndpointsResponse{
		EndpointCollection: generativeai.EndpointCollection{
			Items: []generativeai.EndpointSummary{
				{Id: &epID, DedicatedAiClusterId: nil}, // malformed: must be skipped
			},
		},
	}
	// Stop at the cluster-delete call to prove we got past the endpoint loop.
	opcReqID := "req-123"
	fakeClient.DeleteDedicatedResp = generativeai.DeleteDedicatedAiClusterResponse{OpcRequestId: &opcReqID}
	fakeClient.DeleteDedicatedErr = errors.New("reached-delete")
	newGenAIClient = func(_ models.Environment) (genAI, error) { return fakeClient, nil }

	dac := &models.DedicatedAICluster{}
	env := models.Environment{Type: "prod", Region: "us-phoenix-1", Realm: "oc1"}
	err := DeleteDedicatedAICluster(context.Background(), dac, env, &testLogger{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "reached-delete")
}

// Endpoints past the first ListEndpoints page must be deleted too: every page
// is requested with the previous response's OpcNextPage token.
func TestDeleteEndpointsInDAC_FollowsPages(t *testing.T) {
	clusterID := "ocid1.dedicatedaicluster.oc1..example"
	otherID := "ocid1.dedicatedaicluster.oc1..other"
	workReqID := "ocid1.workrequest.oc1..workreq"
	ep := func(id, cluster string) generativeai.EndpointSummary {
		return generativeai.EndpointSummary{Id: strPtr(id), DedicatedAiClusterId: strPtr(cluster)}
	}
	pages := map[string]generativeai.ListEndpointsResponse{
		"": {
			EndpointCollection: generativeai.EndpointCollection{Items: []generativeai.EndpointSummary{ep("ep1", clusterID), ep("x1", otherID)}},
			OpcNextPage:        strPtr("p2"),
		},
		"p2": {
			EndpointCollection: generativeai.EndpointCollection{Items: []generativeai.EndpointSummary{ep("ep2", clusterID)}},
			OpcNextPage:        strPtr("p3"),
		},
		"p3": {
			EndpointCollection: generativeai.EndpointCollection{Items: []generativeai.EndpointSummary{ep("ep3", clusterID)}},
		},
	}

	var mu sync.Mutex
	var requested, deleted []string
	fakeClient := &fakeGenAI{
		GetWorkRequestResp: generativeai.GetWorkRequestResponse{
			WorkRequest: generativeai.WorkRequest{Status: generativeai.OperationStatusSucceeded},
		},
		ListEndpointsFunc: func(req generativeai.ListEndpointsRequest) (generativeai.ListEndpointsResponse, error) {
			assert.Equal(t, "compartment", derefOr(req.CompartmentId, ""))
			page := derefOr(req.Page, "")
			requested = append(requested, page)
			return pages[page], nil
		},
		DeleteEndpointFunc: func(req generativeai.DeleteEndpointRequest) (generativeai.DeleteEndpointResponse, error) {
			mu.Lock()
			defer mu.Unlock()
			deleted = append(deleted, *req.EndpointId)
			return generativeai.DeleteEndpointResponse{OpcWorkRequestId: &workReqID}, nil
		},
	}

	// testLogger is not goroutine-safe and endpoints are deleted concurrently.
	dac := &generativeai.DedicatedAiCluster{Id: &clusterID, CompartmentId: strPtr("compartment")}
	err := deleteEndpointsInDAC(context.Background(), fakeClient, dac, logging.NewNoOpLogger())
	require.NoError(t, err)
	assert.Equal(t, []string{"", "p2", "p3"}, requested)
	assert.ElementsMatch(t, []string{"ep1", "ep2", "ep3"}, deleted)
}

// A failure on a later page aborts before any endpoint is deleted.
func TestDeleteEndpointsInDAC_PageError(t *testing.T) {
	fakeClient := &fakeGenAI{
		ListEndpointsFunc: func(req generativeai.ListEndpointsRequest) (generativeai.ListEndpointsResponse, error) {
			if req.Page == nil {
				return generativeai.ListEndpointsResponse{OpcNextPage: strPtr("p2")}, nil
			}
			return generativeai.ListEndpointsResponse{}, errors.New("page2 boom")
		},
		DeleteEndpointFunc: func(generativeai.DeleteEndpointRequest) (generativeai.DeleteEndpointResponse, error) {
			t.Error("no endpoint should be deleted")
			return generativeai.DeleteEndpointResponse{}, nil
		},
	}
	dac := &generativeai.DedicatedAiCluster{Id: strPtr("dac"), CompartmentId: strPtr("compartment")}
	err := deleteEndpointsInDAC(context.Background(), fakeClient, dac, &testLogger{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to list endpoints")
	assert.Contains(t, err.Error(), "page2 boom")
}
