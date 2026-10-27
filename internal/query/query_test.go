package query

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jingle2008/toolkit/internal/infra/loader"
	"github.com/jingle2008/toolkit/internal/infra/terraform"
	"github.com/jingle2008/toolkit/pkg/models"
)

// fakeLoader serves the loads these tests make; any other panics.
type fakeLoader struct {
	loader.Composite
	tenants    []models.Tenant
	tenantsErr error
	dacs       map[string][]models.DedicatedAICluster
	dacsErr    error
	workloads  map[string][]models.GPUWorkload
	pools      []models.GPUPool
	poolsErr   error
}

func (f fakeLoader) LoadTenancyOverrideGroup(context.Context, string, models.Environment) (models.TenancyOverrideGroup, error) {
	return models.TenancyOverrideGroup{Tenants: f.tenants}, f.tenantsErr
}

func (f fakeLoader) LoadDedicatedAIClusters(context.Context, string, models.Environment) (map[string][]models.DedicatedAICluster, error) {
	return f.dacs, f.dacsErr
}

func (f fakeLoader) LoadGPUWorkloadsByNode(context.Context, string, models.Environment) (map[string][]models.GPUWorkload, error) {
	return f.workloads, nil
}

func (f fakeLoader) LoadGPUPools(context.Context, string, models.Environment) ([]models.GPUPool, error) {
	return f.pools, f.poolsErr
}

var acme = models.Tenant{Name: "acme", ID: "ocid1.tenancy.oc1..acme"}

// DACs are re-keyed by the owning tenant's name, each keeping its raw id.
func TestDACs_ResolvesOwners(t *testing.T) {
	t.Parallel()
	src := Source{Loader: fakeLoader{
		tenants: []models.Tenant{acme},
		dacs:    map[string][]models.DedicatedAICluster{"acme": {{Name: "dac-1", TenantID: acme.ID}}},
	}}
	got, warnings, err := DACs(context.Background(), src, models.Environment{})
	require.NoError(t, err)
	assert.Empty(t, warnings)
	require.Len(t, got["acme"], 1)
	require.NotNil(t, got["acme"][0].Owner)
	assert.Equal(t, "acme", got["acme"][0].Owner.Name)
	assert.Equal(t, acme.ID, got["acme"][0].TenantID)
}

// Tenants that don't load are a warning: the rows still load, unowned.
func TestDACs_TenantsUnavailable(t *testing.T) {
	t.Parallel()
	src := Source{Loader: fakeLoader{
		tenantsErr: errors.New("repo unreadable"),
		dacs:       map[string][]models.DedicatedAICluster{"acme": {{Name: "dac-1"}}},
	}}
	got, warnings, err := DACs(context.Background(), src, models.Environment{})
	require.NoError(t, err)
	assert.Len(t, got["acme"], 1)
	require.Len(t, warnings, 1)
	assert.Contains(t, warnings[0], "tenant names unresolved")
	assert.Contains(t, warnings[0], "repo unreadable")
}

func TestDACs_LoadFails(t *testing.T) {
	t.Parallel()
	_, _, err := DACs(context.Background(), Source{Loader: fakeLoader{dacsErr: errors.New("forbidden")}}, models.Environment{})
	require.ErrorContains(t, err, "load dedicated AI clusters: forbidden")
}

// A workload whose DACs don't load still lists, with a warning saying
// why it may have no tenant.
func TestGPUWorkloads_DACsUnavailable(t *testing.T) {
	t.Parallel()
	src := Source{Loader: fakeLoader{
		dacsErr:   errors.New("forbidden"),
		workloads: map[string][]models.GPUWorkload{"node-1": {{Name: "pod-1"}}},
	}}
	got, warnings, err := GPUWorkloads(context.Background(), src, models.Environment{})
	require.NoError(t, err)
	assert.Len(t, got["node-1"], 1)
	assert.Equal(t, []string{"DACs for workload tenants unavailable: forbidden"}, warnings)
}

// A partly loaded repo is one warning per failed source, beside the rows
// that did load; enrichment that can't reach OCI is a warning too.
func TestGPUPools_PartialLoad(t *testing.T) {
	t.Parallel()
	src := Source{KubeConfig: t.TempDir() + "/no-kubeconfig", Loader: fakeLoader{
		pools: []models.GPUPool{{Name: "pool-a"}},
		poolsErr: &terraform.PartialLoadError{Source: "GPUPools", Errs: []error{
			errors.New("source-a: nope"),
			errors.New("source-b: also nope"),
		}},
	}}
	got, warnings, err := GPUPools(context.Background(), src, models.Environment{})
	require.NoError(t, err)
	assert.Len(t, got, 1)
	require.Len(t, warnings, 3)
	assert.Equal(t, "load gpu pools: source-a: nope", warnings[0])
	assert.Equal(t, "load gpu pools: source-b: also nope", warnings[1])
	assert.Contains(t, warnings[2], "gpu pool enrichment incomplete")
}

func TestGPUPools_LoadFails(t *testing.T) {
	t.Parallel()
	_, _, err := GPUPools(context.Background(), Source{Loader: fakeLoader{poolsErr: errors.New("no repo")}}, models.Environment{})
	require.ErrorContains(t, err, "load gpu pools: no repo")
}
