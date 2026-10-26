//nolint:paralleltest
package oci

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/oracle/oci-go-sdk/v65/generativeai"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jingle2008/toolkit/internal/logging"
)

// fakeServiceError implements common.ServiceError with a fixed status.
type fakeServiceError struct{ status int }

func (e fakeServiceError) Error() string           { return fmt.Sprintf("service error %d", e.status) }
func (e fakeServiceError) GetHTTPStatusCode() int  { return e.status }
func (e fakeServiceError) GetMessage() string      { return "msg" }
func (e fakeServiceError) GetCode() string         { return "Code" }
func (e fakeServiceError) GetOpcRequestID() string { return "opc" }

func TestIsRetryableWorkRequestError(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"401 expired token", fakeServiceError{http.StatusUnauthorized}, false},
		{"403", fakeServiceError{http.StatusForbidden}, false},
		{"404", fakeServiceError{http.StatusNotFound}, false},
		{"400", fakeServiceError{http.StatusBadRequest}, false},
		{"wrapped 401", fmt.Errorf("wrap: %w", fakeServiceError{http.StatusUnauthorized}), false},
		{"429", fakeServiceError{http.StatusTooManyRequests}, true},
		{"500", fakeServiceError{http.StatusInternalServerError}, true},
		{"503", fakeServiceError{http.StatusServiceUnavailable}, true},
		{"network", errors.New("connection reset"), true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, isRetryableWorkRequestError(tc.err))
		})
	}
}

// A 401 (expired session token) aborts on the first poll instead of
// retrying until the 10-minute deletion timeout.
func TestWaitForWorkRequest_NonRetryableFailsFast(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		calls := 0
		client := &fakeGenAI{GetWorkRequestFunc: func() (generativeai.GetWorkRequestResponse, error) {
			calls++
			return generativeai.GetWorkRequestResponse{}, fakeServiceError{http.StatusUnauthorized}
		}}
		start := time.Now()
		err := waitForWorkRequest(t.Context(), client, strPtr("wr"), slog.New(&testLogger{}))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "service error 401")
		assert.Equal(t, 1, calls)
		assert.Equal(t, workRequestPollInterval, time.Since(start))
	})
}

// Transient failures (429, 5xx, network) keep polling until success.
func TestWaitForWorkRequest_RetriesTransient(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		errs := []error{
			fakeServiceError{http.StatusTooManyRequests},
			fakeServiceError{http.StatusServiceUnavailable},
			errors.New("connection reset"),
		}
		calls := 0
		client := &fakeGenAI{GetWorkRequestFunc: func() (generativeai.GetWorkRequestResponse, error) {
			calls++
			if calls <= len(errs) {
				return generativeai.GetWorkRequestResponse{}, errs[calls-1]
			}
			return generativeai.GetWorkRequestResponse{
				WorkRequest: generativeai.WorkRequest{Status: generativeai.OperationStatusSucceeded},
			}, nil
		}}
		require.NoError(t, waitForWorkRequest(t.Context(), client, strPtr("wr"), slog.New(&testLogger{})))
		assert.Equal(t, len(errs)+1, calls)
	})
}

func endpointsFor(clusterID string, n int) generativeai.ListEndpointsResponse {
	items := make([]generativeai.EndpointSummary, n)
	for i := range items {
		items[i] = generativeai.EndpointSummary{Id: strPtr(fmt.Sprintf("ep%d", i)), DedicatedAiClusterId: strPtr(clusterID)}
	}
	return generativeai.ListEndpointsResponse{EndpointCollection: generativeai.EndpointCollection{Items: items}}
}

// At most maxConcurrent endpoint deletions run at once.
func TestDeleteEndpointsInDAC_ConcurrencyLimit(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		const clusterID = "dac"
		var mu sync.Mutex
		inFlight, maxInFlight, deleted := 0, 0, 0
		client := &fakeGenAI{
			ListEndpointsResp: endpointsFor(clusterID, 12),
			GetWorkRequestResp: generativeai.GetWorkRequestResponse{
				WorkRequest: generativeai.WorkRequest{Status: generativeai.OperationStatusSucceeded},
			},
			DeleteEndpointFunc: func(generativeai.DeleteEndpointRequest) (generativeai.DeleteEndpointResponse, error) {
				mu.Lock()
				inFlight++
				maxInFlight = max(maxInFlight, inFlight)
				mu.Unlock()
				time.Sleep(time.Second)
				mu.Lock()
				inFlight--
				deleted++
				mu.Unlock()
				return generativeai.DeleteEndpointResponse{OpcWorkRequestId: strPtr("wr")}, nil
			},
		}
		dac := &generativeai.DedicatedAiCluster{Id: strPtr(clusterID), CompartmentId: strPtr("c")}
		require.NoError(t, deleteEndpointsInDAC(t.Context(), client, dac, logging.Discard()))
		assert.Equal(t, 12, deleted)
		assert.LessOrEqual(t, maxInFlight, 5)
	})
}

// Once one deletion fails, no further endpoints are queued.
func TestDeleteEndpointsInDAC_StopsAfterFailure(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		const clusterID = "dac"
		var mu sync.Mutex
		var deleted []string
		client := &fakeGenAI{
			ListEndpointsResp: endpointsFor(clusterID, 10),
			DeleteEndpointFunc: func(req generativeai.DeleteEndpointRequest) (generativeai.DeleteEndpointResponse, error) {
				mu.Lock()
				deleted = append(deleted, *req.EndpointId)
				mu.Unlock()
				if *req.EndpointId == "ep0" {
					return generativeai.DeleteEndpointResponse{}, errors.New("boom")
				}
				time.Sleep(time.Second)
				return generativeai.DeleteEndpointResponse{OpcWorkRequestId: strPtr("wr")}, nil
			},
		}
		dac := &generativeai.DedicatedAiCluster{Id: strPtr(clusterID), CompartmentId: strPtr("c")}
		err := deleteEndpointsInDAC(context.Background(), client, dac, logging.Discard())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "boom")
		// ep0..ep4 start at once; the slot ep0 frees may admit one more
		// before the loop observes the cancelled group context.
		assert.LessOrEqual(t, len(deleted), 6)
	})
}
