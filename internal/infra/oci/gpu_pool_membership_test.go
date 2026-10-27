//nolint:paralleltest // tests swap the package-level client seams
package oci

import (
	"context"
	"errors"
	"strconv"
	"testing"

	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jingle2008/toolkit/pkg/models"
)

type fakeInstanceGetter struct {
	inst core.Instance
	err  error
}

func (f fakeInstanceGetter) GetInstance(context.Context, core.GetInstanceRequest) (core.GetInstanceResponse, error) {
	return core.GetInstanceResponse{Instance: f.inst}, f.err
}

// fakePoolLister serves instance pools and their members in pages of
// one item, so every lookup must follow OpcNextPage to see them all.
type fakePoolLister struct {
	pools     []core.InstancePoolSummary
	members   map[string][]string // pool ID → instance IDs
	poolsErr  error
	listedFor []string // pool IDs whose instances were listed
}

func nextPage(i, n int) *string {
	if i+1 >= n {
		return nil
	}
	return common.String(strconv.Itoa(i + 1))
}

func pageIndex(p *string) int {
	if p == nil {
		return 0
	}
	n, _ := strconv.Atoi(*p)
	return n
}

func (f *fakePoolLister) ListInstancePools(_ context.Context, req core.ListInstancePoolsRequest) (core.ListInstancePoolsResponse, error) {
	if f.poolsErr != nil {
		return core.ListInstancePoolsResponse{}, f.poolsErr
	}
	i := pageIndex(req.Page)
	if i >= len(f.pools) {
		return core.ListInstancePoolsResponse{}, nil
	}
	return core.ListInstancePoolsResponse{Items: f.pools[i : i+1], OpcNextPage: nextPage(i, len(f.pools))}, nil
}

func (f *fakePoolLister) ListInstancePoolInstances(_ context.Context, req core.ListInstancePoolInstancesRequest) (core.ListInstancePoolInstancesResponse, error) {
	ids := f.members[*req.InstancePoolId]
	i := pageIndex(req.Page)
	if i == 0 {
		f.listedFor = append(f.listedFor, *req.InstancePoolId)
	}
	if i >= len(ids) {
		return core.ListInstancePoolInstancesResponse{}, nil
	}
	return core.ListInstancePoolInstancesResponse{
		Items:       []core.InstanceSummary{{Id: common.String(ids[i])}},
		OpcNextPage: nextPage(i, len(ids)),
	}, nil
}

func poolSummary(id, name string) core.InstancePoolSummary {
	return core.InstancePoolSummary{Id: common.String(id), DisplayName: common.String(name)}
}

func stubMembership(t *testing.T, getter fakeInstanceGetter, lister *fakePoolLister) {
	t.Helper()
	origGet, origList := newInstanceGetter, newPoolInstanceLister
	t.Cleanup(func() { newInstanceGetter, newPoolInstanceLister = origGet, origList })
	newInstanceGetter = func(models.Environment) (instanceGetter, error) { return getter, nil }
	newPoolInstanceLister = func(models.Environment) (poolInstanceLister, error) { return lister, nil }
}

func inCompartment(c string) fakeInstanceGetter {
	return fakeInstanceGetter{inst: core.Instance{CompartmentId: common.String(c)}}
}

func TestFindGPUPoolInstance_InPool(t *testing.T) {
	lister := &fakePoolLister{
		pools: []core.InstancePoolSummary{
			poolSummary("ocid1.instancepool.other", "not-gpu"),
			poolSummary("ocid1.instancepool.a", "pool-a"),
			poolSummary("ocid1.instancepool.b", "pool-b"),
		},
		members: map[string][]string{
			"ocid1.instancepool.other": {"ocid1.instance.x"},
			"ocid1.instancepool.a":     {"ocid1.instance.a1"},
			"ocid1.instancepool.b":     {"ocid1.instance.b1", "ocid1.instance.b2"},
		},
	}
	stubMembership(t, inCompartment("ocid1.compartment.c"), lister)

	got, err := FindGPUPoolInstance(context.Background(), models.Environment{}, "ocid1.instance.b2", "ocid1.compartment.c", []string{"pool-a", "pool-b"})
	require.NoError(t, err)
	assert.Equal(t, &PoolInstance{
		ID: "ocid1.instance.b2", CompartmentID: "ocid1.compartment.c",
		PoolName: "pool-b", PoolID: "ocid1.instancepool.b",
	}, got)
	assert.Equal(t, []string{"ocid1.instancepool.a", "ocid1.instancepool.b"}, lister.listedFor,
		"only GPU pools are searched")
}

func TestFindGPUPoolInstance_NotInGPUPool(t *testing.T) {
	lister := &fakePoolLister{
		pools: []core.InstancePoolSummary{
			poolSummary("ocid1.instancepool.other", "not-gpu"),
			poolSummary("ocid1.instancepool.a", "pool-a"),
		},
		members: map[string][]string{
			"ocid1.instancepool.other": {"ocid1.instance.x"},
			"ocid1.instancepool.a":     {"ocid1.instance.a1"},
		},
	}
	stubMembership(t, inCompartment("ocid1.compartment.c"), lister)

	_, err := FindGPUPoolInstance(context.Background(), models.Environment{}, "ocid1.instance.x", "ocid1.compartment.c", []string{"pool-a"})
	require.ErrorIs(t, err, ErrNotGPUPoolInstance)
}

func TestFindGPUPoolInstance_Refusals(t *testing.T) {
	ctx := context.Background()
	t.Run("not an instance OCID", func(t *testing.T) {
		stubMembership(t, fakeInstanceGetter{err: errors.New("must not be called")}, &fakePoolLister{})
		_, err := FindGPUPoolInstance(ctx, models.Environment{}, "ocid1.dedicatedaicluster.x", "c", []string{"pool-a"})
		require.ErrorContains(t, err, "not a compute instance OCID")
	})
	t.Run("get instance fails", func(t *testing.T) {
		stubMembership(t, fakeInstanceGetter{err: errors.New("404 not authorized or not found")}, &fakePoolLister{})
		_, err := FindGPUPoolInstance(ctx, models.Environment{}, "ocid1.instance.x", "c", []string{"pool-a"})
		require.ErrorContains(t, err, "404 not authorized or not found")
	})
	t.Run("no env compartment", func(t *testing.T) {
		stubMembership(t, fakeInstanceGetter{err: errors.New("must not be called")}, &fakePoolLister{})
		_, err := FindGPUPoolInstance(ctx, models.Environment{}, "ocid1.instance.x", "", []string{"pool-a"})
		require.ErrorContains(t, err, "no GPU compartment")
	})
	// A pool in another compartment named like a GPU pool doesn't make
	// its instances the env's: they are refused before any pool is listed.
	t.Run("other compartment", func(t *testing.T) {
		lister := &fakePoolLister{
			pools:   []core.InstancePoolSummary{poolSummary("ocid1.instancepool.a", "pool-a")},
			members: map[string][]string{"ocid1.instancepool.a": {"ocid1.instance.x"}},
		}
		stubMembership(t, inCompartment("other"), lister)
		_, err := FindGPUPoolInstance(ctx, models.Environment{}, "ocid1.instance.x", "c", []string{"pool-a"})
		require.ErrorIs(t, err, ErrNotGPUPoolInstance)
		require.ErrorContains(t, err, `compartment "other", not the env's GPU compartment c`)
		assert.Empty(t, lister.listedFor)
	})
	t.Run("instance reports no compartment", func(t *testing.T) {
		stubMembership(t, fakeInstanceGetter{}, &fakePoolLister{})
		_, err := FindGPUPoolInstance(ctx, models.Environment{}, "ocid1.instance.x", "c", []string{"pool-a"})
		require.ErrorIs(t, err, ErrNotGPUPoolInstance)
	})
	t.Run("list pools fails", func(t *testing.T) {
		stubMembership(t, inCompartment("c"), &fakePoolLister{poolsErr: errors.New("throttled")})
		_, err := FindGPUPoolInstance(ctx, models.Environment{}, "ocid1.instance.x", "c", []string{"pool-a"})
		require.ErrorContains(t, err, "throttled")
	})
}

func TestFindGPUPoolInstance_ClientErrors(t *testing.T) {
	origGet, origList := newInstanceGetter, newPoolInstanceLister
	t.Cleanup(func() { newInstanceGetter, newPoolInstanceLister = origGet, origList })
	ctx := context.Background()

	newInstanceGetter = func(models.Environment) (instanceGetter, error) { return nil, errors.New("no config") }
	_, err := FindGPUPoolInstance(ctx, models.Environment{}, "ocid1.instance.x", "c", nil)
	require.ErrorContains(t, err, "compute client")

	newInstanceGetter = func(models.Environment) (instanceGetter, error) { return inCompartment("c"), nil }
	newPoolInstanceLister = func(models.Environment) (poolInstanceLister, error) { return nil, errors.New("no config") }
	_, err = FindGPUPoolInstance(ctx, models.Environment{}, "ocid1.instance.x", "c", nil)
	require.ErrorContains(t, err, "compute management client")
}

// failingMemberLister fails every ListInstancePoolInstances call.
type failingMemberLister struct{ fakePoolLister }

func (f *failingMemberLister) ListInstancePoolInstances(context.Context, core.ListInstancePoolInstancesRequest) (core.ListInstancePoolInstancesResponse, error) {
	return core.ListInstancePoolInstancesResponse{}, errors.New("pool gone")
}

func TestFindGPUPoolInstance_ListMembersFails(t *testing.T) {
	lister := &failingMemberLister{fakePoolLister{pools: []core.InstancePoolSummary{poolSummary("p", "pool-a")}}}
	origGet, origList := newInstanceGetter, newPoolInstanceLister
	t.Cleanup(func() { newInstanceGetter, newPoolInstanceLister = origGet, origList })
	newInstanceGetter = func(models.Environment) (instanceGetter, error) { return inCompartment("c"), nil }
	newPoolInstanceLister = func(models.Environment) (poolInstanceLister, error) { return lister, nil }

	_, err := FindGPUPoolInstance(context.Background(), models.Environment{}, "ocid1.instance.x", "c", []string{"pool-a"})
	require.ErrorContains(t, err, "pool gone")
}
