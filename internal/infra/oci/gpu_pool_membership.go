package oci

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/core"

	"github.com/jingle2008/toolkit/pkg/models"
)

// Seams for FindGPUPoolInstance, kept apart from newComputeClient /
// newComputeMgmtClient so their narrow interfaces don't grow the fakes
// the action tests use.
var (
	newInstanceGetter func(models.Environment) (instanceGetter, error) = func(env models.Environment) (instanceGetter, error) {
		return NewComputeClient(env)
	}
	newPoolInstanceLister func(models.Environment) (poolInstanceLister, error) = func(env models.Environment) (poolInstanceLister, error) {
		return NewComputeManagementClient(env)
	}
)

type instanceGetter interface {
	GetInstance(ctx context.Context, req core.GetInstanceRequest) (core.GetInstanceResponse, error)
}

type poolInstanceLister interface {
	ListInstancePools(ctx context.Context, req core.ListInstancePoolsRequest) (core.ListInstancePoolsResponse, error)
	ListInstancePoolInstances(ctx context.Context, req core.ListInstancePoolInstancesRequest) (core.ListInstancePoolInstancesResponse, error)
}

// ErrNotGPUPoolInstance is returned for an instance OCID that is not in
// any of the environment's GPU instance pools.
var ErrNotGPUPoolInstance = errors.New("not an instance of any GPU pool in this environment")

// instanceOCIDPrefix starts every compute instance OCID.
const instanceOCIDPrefix = "ocid1.instance."

// PoolInstance is an instance found in one of an environment's GPU
// instance pools.
type PoolInstance struct {
	ID            string
	CompartmentID string
	PoolName      string
	PoolID        string
}

// FindGPUPoolInstance verifies that instanceID is an instance of one of
// the GPU pools named in poolNames (the Terraform-declared pools), with
// no cluster call: it gets the instance for its compartment, lists that
// compartment's instance pools, keeps those whose display name is a GPU
// pool name (as PopulateGPUPools matches them), and looks for the
// instance among their members. Any instance it can't place in such a
// pool is refused with ErrNotGPUPoolInstance.
func FindGPUPoolInstance(
	ctx context.Context,
	env models.Environment,
	instanceID string,
	poolNames []string,
) (*PoolInstance, error) {
	if !strings.HasPrefix(instanceID, instanceOCIDPrefix) {
		return nil, fmt.Errorf("%q is not a compute instance OCID (want %s...)", instanceID, instanceOCIDPrefix)
	}
	compute, err := newInstanceGetter(env)
	if err != nil {
		return nil, fmt.Errorf("failed to create compute client: %w", err)
	}
	inst, err := compute.GetInstance(ctx, core.GetInstanceRequest{InstanceId: common.String(instanceID)})
	if err != nil {
		return nil, fmt.Errorf("failed to get instance %s: %w, request id: %s",
			instanceID, err, derefOr(inst.OpcRequestId, ""))
	}
	compartmentID := derefOr(inst.CompartmentId, "")
	if compartmentID == "" {
		return nil, fmt.Errorf("instance %s reported no compartment", instanceID)
	}

	mgmt, err := newPoolInstanceLister(env)
	if err != nil {
		return nil, fmt.Errorf("failed to create compute management client: %w", err)
	}
	pools, err := gpuInstancePools(ctx, mgmt, compartmentID, poolNames)
	if err != nil {
		return nil, err
	}
	for _, pool := range pools {
		found, err := poolHasInstance(ctx, mgmt, compartmentID, *pool.Id, instanceID)
		if err != nil {
			return nil, err
		}
		if found {
			return &PoolInstance{
				ID:            instanceID,
				CompartmentID: compartmentID,
				PoolName:      *pool.DisplayName,
				PoolID:        *pool.Id,
			}, nil
		}
	}
	return nil, fmt.Errorf("instance %s: %w", instanceID, ErrNotGPUPoolInstance)
}

// gpuInstancePools lists every instance pool in compartmentID, page by
// page, keeping those whose display name is one of poolNames.
func gpuInstancePools(
	ctx context.Context, mgmt poolInstanceLister, compartmentID string, poolNames []string,
) ([]core.InstancePoolSummary, error) {
	names := make(map[string]bool, len(poolNames))
	for _, n := range poolNames {
		names[n] = true
	}
	all, err := listAllPages(func(page *string) ([]core.InstancePoolSummary, *string, error) {
		resp, err := mgmt.ListInstancePools(ctx, core.ListInstancePoolsRequest{
			CompartmentId: common.String(compartmentID),
			Page:          page,
		})
		if err != nil {
			return nil, nil, fmt.Errorf("failed to list instance pools: %w, request id: %s",
				err, derefOr(resp.OpcRequestId, ""))
		}
		return resp.Items, resp.OpcNextPage, nil
	})
	if err != nil {
		return nil, err
	}
	var out []core.InstancePoolSummary
	for _, s := range all {
		if s.Id != nil && s.DisplayName != nil && names[*s.DisplayName] {
			out = append(out, s)
		}
	}
	return out, nil
}

// poolHasInstance reports whether instanceID is a member of instance
// pool poolID, following every page of the pool's instances. It pages by
// hand rather than through listAllPages so it can stop at the first match.
func poolHasInstance(ctx context.Context, mgmt poolInstanceLister, compartmentID, poolID, instanceID string) (bool, error) {
	req := core.ListInstancePoolInstancesRequest{
		CompartmentId:  common.String(compartmentID),
		InstancePoolId: common.String(poolID),
	}
	for {
		resp, err := mgmt.ListInstancePoolInstances(ctx, req)
		if err != nil {
			return false, fmt.Errorf("failed to list instances of pool %s: %w, request id: %s",
				poolID, err, derefOr(resp.OpcRequestId, ""))
		}
		for _, s := range resp.Items {
			if s.Id != nil && *s.Id == instanceID {
				return true, nil
			}
		}
		if resp.OpcNextPage == nil || *resp.OpcNextPage == "" {
			return false, nil
		}
		req.Page = resp.OpcNextPage
	}
}
